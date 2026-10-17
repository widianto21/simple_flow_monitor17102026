// Package handlers_test exercises the HTTP surface end to end through the real
// router: every route is called with the real handlers and a real (in-memory)
// database, so path parameters, status codes and JSON shapes are verified.
package handlers_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"flowmonitor/backend/internal/config"
	"flowmonitor/backend/internal/dto"
	"flowmonitor/backend/internal/models"
	"flowmonitor/backend/internal/routes"
	"flowmonitor/backend/internal/service"
)

type api struct {
	router *gin.Engine
	svc    *service.Services
}

func newAPI(t *testing.T) *api {
	t.Helper()
	gin.SetMode(gin.TestMode)

	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"))
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}
	if err := db.AutoMigrate(
		&models.Flow{}, &models.FlowNode{}, &models.FlowEdge{},
		&models.Run{}, &models.StepRun{}, &models.Event{}, &models.Alert{},
	); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	cfg := &config.Config{AutoRegister: true, DefaultSLAMinutes: 60}
	svc := service.New(db, cfg)

	// A tiny flow + one finished and one failed execution.
	flow := models.Flow{Key: "flow-one", Name: "Flow One", Active: true}
	if err := db.Create(&flow).Error; err != nil {
		t.Fatal(err)
	}
	nodes := []models.FlowNode{
		{FlowID: flow.ID, Key: "start", Name: "Start", Kind: models.KindStart, TimeoutMinutes: 30},
		{FlowID: flow.ID, Key: "work", Name: "Work", Kind: models.KindTask, System: "worker", Method: "POST", Endpoint: "/work", TimeoutMinutes: 5, RequestSchema: `{"in":true}`, ResponseSample: `{"out":true}`},
		{FlowID: flow.ID, Key: "end", Name: "End", Kind: models.KindEnd, TimeoutMinutes: 30},
	}
	if err := db.Create(&nodes).Error; err != nil {
		t.Fatal(err)
	}
	edges := []models.FlowEdge{
		{FlowID: flow.ID, SourceKey: "start", TargetKey: "work"},
		{FlowID: flow.ID, SourceKey: "work", TargetKey: "end"},
	}
	if err := db.Create(&edges).Error; err != nil {
		t.Fatal(err)
	}

	now := time.Now().UTC()
	t0 := now.Add(-10 * time.Minute)
	events := []dto.IngestEvent{
		{FlowKey: "flow-one", NodeKey: "start", BusinessKey: "BK-1", Status: "started", OccurredAt: &t0},
		{FlowKey: "flow-one", NodeKey: "work", BusinessKey: "BK-1", Status: "started", OccurredAt: timePtr(t0.Add(time.Second)), Request: []byte(`{"in":true}`)},
		{FlowKey: "flow-one", NodeKey: "work", BusinessKey: "BK-1", Status: "failed", OccurredAt: timePtr(t0.Add(2 * time.Minute)), Error: "boom"},
		// a second execution is still running (and stuck, `work` has a 5m SLA)
		{FlowKey: "flow-one", NodeKey: "start", BusinessKey: "BK-2", Status: "started", OccurredAt: timePtr(now.Add(-8 * time.Minute))},
		{FlowKey: "flow-one", NodeKey: "work", BusinessKey: "BK-2", Status: "started", OccurredAt: timePtr(now.Add(-7 * time.Minute))},
	}
	for _, ev := range events {
		if _, err := svc.Ingest.Process(context.Background(), ev); err != nil {
			t.Fatalf("seed ingest: %v", err)
		}
	}

	return &api{router: routes.New(svc, cfg), svc: svc}
}

func timePtr(t time.Time) *time.Time { return &t }

// call performs a request and asserts the expected status code.
// It returns either map[string]any (for JSON objects) or []any (for JSON arrays).
func (a *api) call(t *testing.T, method, path string, body string, wantStatus int) any {
	t.Helper()

	var reader *bytes.Reader
	if body == "" {
		reader = bytes.NewReader(nil)
	} else {
		reader = bytes.NewReader([]byte(body))
	}
	req := httptest.NewRequest(method, path, reader)
	req.Header.Set("Content-Type", "application/json")

	rec := httptest.NewRecorder()
	a.router.ServeHTTP(rec, req)

	if rec.Code != wantStatus {
		t.Fatalf("%s %s: status = %d, want %d (body: %s)", method, path, rec.Code, wantStatus, rec.Body.String())
	}
	if rec.Body.Len() == 0 {
		return map[string]any{}
	}
	var obj map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &obj); err == nil {
		return obj
	}
	var arr []any
	if err := json.Unmarshal(rec.Body.Bytes(), &arr); err == nil {
		return arr
	}
	t.Fatalf("unmarshable JSON from %s %s: %s", method, path, rec.Body.String())
	return nil
}

func TestReadEndpoints(t *testing.T) {
	a := newAPI(t)

	a.call(t, http.MethodGet, "/api/health", "", http.StatusOK)

	board := a.call(t, http.MethodGet, "/api/dashboard", "", http.StatusOK).(map[string]any)
	kpis := board["kpis"].(map[string]any)
	if kpis["stuckTasks"].(float64) != 1 {
		t.Fatalf("stuckTasks = %v, want 1", kpis["stuckTasks"])
	}
	if kpis["failed24h"].(float64) != 1 {
		t.Fatalf("failed24h = %v, want 1", kpis["failed24h"])
	}

	a.call(t, http.MethodGet, "/api/flows", "", http.StatusOK)
	a.call(t, http.MethodGet, "/api/flows/flow-one", "", http.StatusOK)
	a.call(t, http.MethodGet, "/api/flows/1", "", http.StatusOK)
	a.call(t, http.MethodGet, "/api/flows/flow-one/stats", "", http.StatusOK)

	// the drill-down envelope: { node, tasks, recentRuns }
	drilldown := a.call(t, http.MethodGet, "/api/flows/flow-one/nodes/work", "", http.StatusOK).(map[string]any)
	node := drilldown["node"].(map[string]any)
	if node["key"] != "work" {
		t.Fatalf("node detail key = %v", node["key"])
	}
	if node["system"] != "worker" || node["endpoint"] != "/work" {
		t.Fatalf("node contract missing: %v", node)
	}
	stats := node["stats"].(map[string]any)
	if stats["running"].(float64) != 1 {
		t.Fatalf("node stats = %v, want running=1", node["stats"])
	}
	tasks := drilldown["tasks"].(map[string]any)
	if tasks["total"].(float64) != 2 {
		t.Fatalf("node task history = %v, want total=2", drilldown["tasks"])
	}

	a.call(t, http.MethodGet, "/api/flows/nope", "", http.StatusNotFound)
	a.call(t, http.MethodGet, "/api/flows/flow-one/nodes/ghost", "", http.StatusNotFound)

	runs := a.call(t, http.MethodGet, "/api/runs?size=10", "", http.StatusOK).(map[string]any)
	if runs["total"].(float64) != 2 {
		t.Fatalf("runs total = %v, want 2", runs["total"])
	}

	// the failed execution, by id from the list
	items := runs["items"].([]any)
	var failedID, runningID float64
	for _, raw := range items {
		run := raw.(map[string]any)
		switch run["businessKey"] {
		case "BK-1":
			failedID = run["id"].(float64)
		case "BK-2":
			runningID = run["id"].(float64)
			if run["stuck"] != true {
				t.Fatalf("running execution should be flagged stuck: %v", run)
			}
		}
	}

	run := a.call(t, http.MethodGet, fmt.Sprintf("/api/runs/%.0f", failedID), "", http.StatusOK).(map[string]any)
	if run["status"] != "failed" || run["failureError"] != "boom" {
		t.Fatalf("unexpected run: %v", run)
	}
	steps := run["steps"].([]any)
	if len(steps) != 2 {
		t.Fatalf("run steps = %d, want 2", len(steps))
	}
	stepID := steps[1].(map[string]any)["id"].(float64)

	step := a.call(t, http.MethodGet, fmt.Sprintf("/api/runs/%.0f/steps/%.0f", failedID, stepID), "", http.StatusOK).(map[string]any)
	if step["step"].(map[string]any)["nodeKey"] != "work" {
		t.Fatalf("unexpected step detail: %v", step["step"])
	}
	if step["node"].(map[string]any)["endpoint"] != "/work" {
		t.Fatalf("step detail should include the node contract: %v", step["node"])
	}

	a.call(t, http.MethodGet, "/api/steps?flowKey=flow-one", "", http.StatusOK)
	a.call(t, http.MethodGet, "/api/steps?stuck=true", "", http.StatusOK)

	stuck := a.call(t, http.MethodGet, "/api/tasks/stuck", "", http.StatusOK).(map[string]any)
	if stuck["total"].(float64) != 1 {
		t.Fatalf("stuck total = %v, want 1", stuck["total"])
	}

	stuckRuns := a.call(t, http.MethodGet, "/api/runs?status=stuck", "", http.StatusOK).(map[string]any)
	if stuckRuns["total"].(float64) != 1 {
		t.Fatalf("stuck runs = %v, want 1", stuckRuns["total"])
	}

	history := a.call(t, http.MethodGet, "/api/business-keys/BK-1/runs", "", http.StatusOK)
	if history == nil {
		t.Fatal("business key history should return a body")
	}

	alerts := a.call(t, http.MethodGet, "/api/alerts", "", http.StatusOK).(map[string]any)
	if alerts["total"].(float64) < 1 {
		t.Fatalf("expected an alert for the failure, got %v", alerts["total"])
	}
	alertID := alerts["items"].([]any)[0].(map[string]any)["id"].(float64)
	acked := a.call(t, http.MethodPost, fmt.Sprintf("/api/alerts/%.0f/ack", alertID), "", http.StatusOK).(map[string]any)
	if acked["acknowledged"] != true {
		t.Fatalf("alert not acknowledged: %v", acked)
	}

	_ = runningID
}

func TestIngestEndpoints(t *testing.T) {
	a := newAPI(t)

	// single event, with an alias status
	body := `{"flowKey":"flow-one","nodeKey":"start","businessKey":"BK-3","status":"start","source":"test"}`
	res := a.call(t, http.MethodPost, "/api/ingest/events", body, http.StatusAccepted).(map[string]any)
	if res["accepted"] != true || res["runId"].(float64) == 0 {
		t.Fatalf("unexpected ingest result: %v", res)
	}

	// batch, including one event that cannot be applied
	batch := `[
		{"flowKey":"flow-one","nodeKey":"work","businessKey":"BK-3","status":"started","request":{"in":true}},
		{"flowKey":"flow-one","nodeKey":"work","businessKey":"BK-3","status":"completed","response":{"out":true}},
		{"flowKey":"flow-one","nodeKey":"ghost-step","businessKey":"BK-MISSING","status":"completed"}
	]`
	res = a.call(t, http.MethodPost, "/api/ingest/events/batch", batch, http.StatusAccepted).(map[string]any)
	if res["received"].(float64) != 3 || res["accepted"].(float64) != 2 || res["rejected"].(float64) != 1 {
		t.Fatalf("unexpected batch result: %v", res)
	}

	// bad requests
	a.call(t, http.MethodPost, "/api/ingest/events", `{"nodeKey":"x","businessKey":"y","status":"started"}`, http.StatusBadRequest)
	a.call(t, http.MethodPost, "/api/ingest/events", `{"flowKey":"flow-one","nodeKey":"x","businessKey":"y","status":"exploded"}`, http.StatusBadRequest)
	a.call(t, http.MethodPost, "/api/ingest/events", `{"flowKey":"flow-one","nodeKey":"work","businessKey":"BK-NONE","status":"completed"}`, http.StatusConflict)
	a.call(t, http.MethodPost, "/api/ingest/events/batch", `[]`, http.StatusBadRequest)

	// the contract other teams instrument against
	manifest := a.call(t, http.MethodGet, "/api/ingest/manifests/flow-one", "", http.StatusOK).(map[string]any)
	nodes := manifest["nodes"].([]any)
	if len(nodes) != 3 {
		t.Fatalf("manifest nodes = %d, want 3", len(nodes))
	}
	a.call(t, http.MethodGet, "/api/ingest/manifests/unknown-flow", "", http.StatusNotFound)
}

func TestFlowDefinitionEndpoints(t *testing.T) {
	a := newAPI(t)

	create := `{
		"key":"flow-two","name":"Flow Two","category":"Test","owner":"Team A","version":"1.0.0",
		"nodes":[
			{"key":"start","name":"Start","kind":"start","posX":0,"posY":0,"timeoutMinutes":10},
			{"key":"decide","name":"Decide","kind":"decision","posX":200,"posY":0,"timeoutMinutes":5,
				"system":"rules","method":"POST","endpoint":"/decide",
				"requestSchema":{"orderId":"ORD-1"},"responseSample":{"approved":true}},
			{"key":"end","name":"End","kind":"end","posX":400,"posY":0,"timeoutMinutes":10}
		],
		"edges":[
			{"sourceKey":"start","targetKey":"decide"},
			{"sourceKey":"decide","targetKey":"end","label":"yes"}
		]
	}`
	created := a.call(t, http.MethodPost, "/api/flows", create, http.StatusCreated).(map[string]any)
	flowID := created["id"].(float64)
	if created["nodeCount"].(float64) != 3 {
		t.Fatalf("created flow nodeCount = %v", created["nodeCount"])
	}

	// create returns the projection we need (contract fields survive the round trip)
	for _, raw := range created["nodes"].([]any) {
		node := raw.(map[string]any)
		if node["key"] == "decide" {
			if node["system"] != "rules" || node["endpoint"] != "/decide" {
				t.Fatalf("contract lost in create: %v", node)
			}
			if node["requestSchema"].(map[string]any)["orderId"] != "ORD-1" {
				t.Fatalf("request schema lost: %v", node["requestSchema"])
			}
		}
	}

	update := `{
		"key":"flow-two","name":"Flow Two (renamed)","category":"Test",
		"nodes":[
			{"key":"start","name":"Start","kind":"start","timeoutMinutes":10},
			{"key":"end","name":"End","kind":"end","timeoutMinutes":10}
		],
		"edges":[{"sourceKey":"start","targetKey":"end"}]
	}`
	updated := a.call(t, http.MethodPut, fmt.Sprintf("/api/flows/%.0f", flowID), update, http.StatusOK).(map[string]any)
	if updated["name"] != "Flow Two (renamed)" || updated["nodeCount"].(float64) != 2 {
		t.Fatalf("unexpected update result: name=%v nodes=%v", updated["name"], updated["nodeCount"])
	}

	a.call(t, http.MethodPut, fmt.Sprintf("/api/flows/%.0f", flowID), `{"key":"flow-two","name":"x"}`, http.StatusBadRequest)
	a.call(t, http.MethodDelete, fmt.Sprintf("/api/flows/%.0f", flowID), "", http.StatusOK)
	a.call(t, http.MethodGet, fmt.Sprintf("/api/flows/%.0f", flowID), "", http.StatusNotFound)
}

func TestIngestAPIKeyEnforcement(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db, err := gorm.Open(sqlite.Open("file:auth-test?mode=memory&cache=shared"), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&models.Flow{}, &models.FlowNode{}, &models.FlowEdge{}, &models.Run{}, &models.StepRun{}, &models.Event{}, &models.Alert{}); err != nil {
		t.Fatal(err)
	}
	cfg := &config.Config{AutoRegister: true, DefaultSLAMinutes: 60, IngestAPIKey: "s3cret"}
	router := routes.New(service.New(db, cfg), cfg)

	post := func(key string) int {
		req := httptest.NewRequest(http.MethodPost, "/api/ingest/events",
			bytes.NewReader([]byte(`{"flowKey":"auto","nodeKey":"n","businessKey":"b","status":"started"}`)))
		req.Header.Set("Content-Type", "application/json")
		if key != "" {
			req.Header.Set("X-API-Key", key)
		}
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		return rec.Code
	}

	if code := post(""); code != http.StatusUnauthorized {
		t.Fatalf("without key: status = %d, want 401", code)
	}
	if code := post("wrong"); code != http.StatusUnauthorized {
		t.Fatalf("wrong key: status = %d, want 401", code)
	}
	if code := post("s3cret"); code != http.StatusAccepted {
		t.Fatalf("correct key: status = %d, want 202", code)
	}

	// read endpoints stay open
	req := httptest.NewRequest(http.MethodGet, "/api/flows", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /api/flows: status = %d, want 200", rec.Code)
	}
}
