package service

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"flowmonitor/backend/internal/config"
	"flowmonitor/backend/internal/dto"
	"flowmonitor/backend/internal/models"
)

// ---------------------------------------------------------------------------
// fixtures
// ---------------------------------------------------------------------------

func testServices(t *testing.T) *Services {
	t.Helper()

	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"))
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}
	if err := db.AutoMigrate(
		&models.Flow{}, &models.FlowNode{}, &models.FlowEdge{},
		&models.Run{}, &models.StepRun{}, &models.Event{}, &models.Alert{},
	); err != nil {
		t.Fatalf("migrate test db: %v", err)
	}

	cfg := &config.Config{AutoRegister: true, DefaultSLAMinutes: 60}
	return New(db, cfg)
}

// seedFlow stores a start → pay(task) → slow(task) → done(end) flow.
func seedFlow(t *testing.T, svc *Services) models.Flow {
	t.Helper()

	flow := models.Flow{Key: "order", Name: "Order", Active: true}
	if err := svc.DB.Create(&flow).Error; err != nil {
		t.Fatalf("create flow: %v", err)
	}
	nodes := []models.FlowNode{
		// The start node gets a generous SLA so tests can start a flow a while
		// ago without making the start step look stuck.
		{FlowID: flow.ID, Key: "start", Name: "Received", Kind: models.KindStart, TimeoutMinutes: 30},
		{
			FlowID: flow.ID, Key: "pay", Name: "Pay", Kind: models.KindTask, TimeoutMinutes: 5,
			System: "payment-service", Method: "POST", Endpoint: "/payments/authorize",
			RequestSchema: `{"amount":10}`, ResponseSample: `{"ok":true}`,
		},
		{FlowID: flow.ID, Key: "slow", Name: "Slow Step", Kind: models.KindTask, TimeoutMinutes: 1},
		{FlowID: flow.ID, Key: "done", Name: "Done", Kind: models.KindEnd, TimeoutMinutes: 5},
	}
	if err := svc.DB.Create(&nodes).Error; err != nil {
		t.Fatalf("create nodes: %v", err)
	}
	edges := []models.FlowEdge{
		{FlowID: flow.ID, SourceKey: "start", TargetKey: "pay"},
		{FlowID: flow.ID, SourceKey: "pay", TargetKey: "slow"},
		{FlowID: flow.ID, SourceKey: "slow", TargetKey: "done", Label: "yes"},
	}
	if err := svc.DB.Create(&edges).Error; err != nil {
		t.Fatalf("create edges: %v", err)
	}
	return flow
}

func event(nodeKey, businessKey, status string, at time.Time) dto.IngestEvent {
	return dto.IngestEvent{
		FlowKey:     "order",
		NodeKey:     nodeKey,
		BusinessKey: businessKey,
		Status:      status,
		OccurredAt:  &at,
		Source:      "test-suite",
	}
}

func send(t *testing.T, svc *Services, ev dto.IngestEvent) *dto.IngestResult {
	t.Helper()
	res, err := svc.Ingest.Process(context.Background(), ev)
	if err != nil {
		t.Fatalf("ingest %s/%s: %v", ev.NodeKey, ev.Status, err)
	}
	return res
}

func loadRun(t *testing.T, svc *Services, businessKey string) models.Run {
	t.Helper()
	var run models.Run
	if err := svc.DB.Where("business_key = ?", businessKey).Order("id desc").First(&run).Error; err != nil {
		t.Fatalf("load run: %v", err)
	}
	return run
}

func loadSteps(t *testing.T, svc *Services, runID uint) []models.StepRun {
	t.Helper()
	var steps []models.StepRun
	if err := svc.DB.Where("run_id = ?", runID).Order("sequence asc").Find(&steps).Error; err != nil {
		t.Fatalf("load steps: %v", err)
	}
	return steps
}

// ---------------------------------------------------------------------------
// ingestion lifecycle
// ---------------------------------------------------------------------------

func TestIngestRunLifecycle(t *testing.T) {
	svc := testServices(t)
	seedFlow(t, svc)
	t0 := time.Now().UTC().Add(-30 * time.Minute)

	// start of the flow
	if res := send(t, svc, event("start", "ORD-1", "started", t0)); res.RunID == 0 {
		t.Fatal("expected a run to be created")
	}
	send(t, svc, event("start", "ORD-1", "completed", t0.Add(time.Second)))

	// payment step with a payload
	startRes := send(t, svc, dto.IngestEvent{
		FlowKey: "order", NodeKey: "pay", BusinessKey: "ORD-1", Status: "started",
		OccurredAt: ptr(t0.Add(2 * time.Second)), Request: []byte(`{"amount":10,"currency":"USD"}`),
		Source: "payment-service", Actor: "system",
	})
	if startRes.RunStatus != string(models.RunRunning) {
		t.Fatalf("run should still be running, got %s", startRes.RunStatus)
	}
	send(t, svc, dto.IngestEvent{
		FlowKey: "order", NodeKey: "pay", BusinessKey: "ORD-1", Status: "completed",
		OccurredAt: ptr(t0.Add(3 * time.Minute)), Response: []byte(`{"authorized":true}`),
		Source: "payment-service",
	})

	// remaining steps
	send(t, svc, event("slow", "ORD-1", "started", t0.Add(3*time.Minute+time.Second)))
	send(t, svc, event("slow", "ORD-1", "completed", t0.Add(4*time.Minute)))
	send(t, svc, event("done", "ORD-1", "started", t0.Add(4*time.Minute+time.Second)))
	last := send(t, svc, event("done", "ORD-1", "completed", t0.Add(4*time.Minute+2*time.Second)))

	if last.RunStatus != string(models.RunCompleted) {
		t.Fatalf("run should be completed, got %s", last.RunStatus)
	}

	run := loadRun(t, svc, "ORD-1")
	if run.Status != models.RunCompleted {
		t.Fatalf("run status = %s, want completed", run.Status)
	}
	if run.CompletedAt == nil {
		t.Fatal("completed_at not set")
	}
	if run.CurrentNodeKey != "done" {
		t.Fatalf("current node = %q, want done", run.CurrentNodeKey)
	}
	if run.DurationMs <= 0 {
		t.Fatalf("duration = %d, want > 0", run.DurationMs)
	}

	steps := loadSteps(t, svc, run.ID)
	if len(steps) != 4 {
		t.Fatalf("steps = %d, want 4", len(steps))
	}
	pay := steps[1]
	if pay.NodeKey != "pay" {
		t.Fatalf("second step = %q, want pay", pay.NodeKey)
	}
	if pay.RequestPayload != `{"amount":10,"currency":"USD"}` {
		t.Fatalf("request payload = %q", pay.RequestPayload)
	}
	if pay.ResponsePayload != `{"authorized":true}` {
		t.Fatalf("response payload = %q", pay.ResponsePayload)
	}
	if pay.DurationMs != int64((3*time.Minute - 2*time.Second).Milliseconds()) {
		t.Fatalf("pay duration = %d ms", pay.DurationMs)
	}
	if pay.Sequence != 2 {
		t.Fatalf("pay sequence = %d, want 2", pay.Sequence)
	}
}

func TestIngestIsIdempotent(t *testing.T) {
	svc := testServices(t)
	seedFlow(t, svc)
	t0 := time.Now().UTC().Add(-5 * time.Minute)

	ev := event("start", "ORD-DUP", "started", t0)
	first := send(t, svc, ev)
	second := send(t, svc, ev)

	if !second.Duplicate {
		t.Fatal("second delivery should be flagged as duplicate")
	}
	if first.RunID != second.RunID {
		t.Fatalf("duplicate resolved to a different run (%d != %d)", first.RunID, second.RunID)
	}

	var events int64
	if err := svc.DB.Model(&models.Event{}).Count(&events).Error; err != nil {
		t.Fatal(err)
	}
	if events != 1 {
		t.Fatalf("events = %d, want 1", events)
	}
	if steps := loadSteps(t, svc, first.RunID); len(steps) != 1 {
		t.Fatalf("steps = %d, want 1", len(steps))
	}
}

func TestIngestFailureFailsRunAndRaisesAlert(t *testing.T) {
	svc := testServices(t)
	seedFlow(t, svc)
	t0 := time.Now().UTC().Add(-10 * time.Minute)

	send(t, svc, event("start", "ORD-FAIL", "started", t0))
	send(t, svc, event("pay", "ORD-FAIL", "started", t0.Add(time.Second)))
	res := send(t, svc, dto.IngestEvent{
		FlowKey: "order", NodeKey: "pay", BusinessKey: "ORD-FAIL", Status: "failed",
		OccurredAt: ptr(t0.Add(2 * time.Minute)), Error: "card declined",
	})

	if res.RunStatus != string(models.RunFailed) {
		t.Fatalf("run status = %s, want failed", res.RunStatus)
	}
	run := loadRun(t, svc, "ORD-FAIL")
	if run.FailureError != "card declined" {
		t.Fatalf("failure error = %q", run.FailureError)
	}

	var alert models.Alert
	if err := svc.DB.Where("run_id = ? AND type = ?", run.ID, models.AlertFailure).First(&alert).Error; err != nil {
		t.Fatalf("failure alert not found: %v", err)
	}
	if alert.Severity != "critical" {
		t.Fatalf("alert severity = %s, want critical", alert.Severity)
	}
}

func TestIngestSingleShotCompletionCreatesClosedStep(t *testing.T) {
	svc := testServices(t)
	seedFlow(t, svc)
	t0 := time.Now().UTC().Add(-3 * time.Minute)

	res := send(t, svc, event("start", "ORD-SHOT", "started", t0))
	// only the terminal event is reported for the next step
	send(t, svc, dto.IngestEvent{
		FlowKey: "order", NodeKey: "pay", BusinessKey: "ORD-SHOT", Status: "completed",
		OccurredAt: ptr(t0.Add(time.Minute)), DurationMs: 45_000, Response: []byte(`{"authorized":true}`),
	})

	steps := loadSteps(t, svc, res.RunID)
	if len(steps) != 2 {
		t.Fatalf("steps = %d, want 2 (start + synthesized pay)", len(steps))
	}
	pay := steps[1]
	if pay.Status != models.StepCompleted {
		t.Fatalf("pay status = %s, want completed", pay.Status)
	}
	if pay.DurationMs != 45_000 {
		t.Fatalf("pay duration = %d, want 45000", pay.DurationMs)
	}
	if run := loadRun(t, svc, "ORD-SHOT"); run.Status != models.RunRunning {
		t.Fatalf("run status = %s, want still running", run.Status)
	}
}

func TestIngestTerminalEventWithoutRunFails(t *testing.T) {
	svc := testServices(t)
	seedFlow(t, svc)

	_, err := svc.Ingest.Process(context.Background(), event("pay", "ORD-NONE", "completed", time.Now().UTC()))
	if err == nil {
		t.Fatal("expected an error when completing a step with no active execution")
	}
	if !strings.Contains(err.Error(), "no active execution") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestIngestValidatesRequiredFields(t *testing.T) {
	svc := testServices(t)
	seedFlow(t, svc)

	if _, err := svc.Ingest.Process(context.Background(), dto.IngestEvent{FlowKey: "order", Status: "started"}); err == nil {
		t.Fatal("expected missing nodeKey/businessKey to fail")
	}
	if _, err := svc.Ingest.Process(context.Background(), event("start", "ORD-X", "exploded", time.Now().UTC())); err == nil {
		t.Fatal("expected invalid status to fail")
	}
	// status aliases are accepted
	if _, err := svc.Ingest.Process(context.Background(), event("start", "ORD-ALIAS", "start", time.Now().UTC())); err != nil {
		t.Fatalf("status alias should be accepted: %v", err)
	}
}

// ---------------------------------------------------------------------------
// stuck detection
// ---------------------------------------------------------------------------

func TestDetectStuck(t *testing.T) {
	now := time.Now().UTC()
	tasks := []OpenTask{
		{StepRunID: 1, NodeKey: "fast", StartedAt: now.Add(-2 * time.Minute), Timeout: 5},
		{StepRunID: 2, NodeKey: "slow", StartedAt: now.Add(-30 * time.Minute), Timeout: 5},
		{StepRunID: 3, NodeKey: "no-sla", StartedAt: now.Add(-5 * time.Hour), Timeout: 0},
	}

	stuck := DetectStuck(tasks, now)
	if len(stuck) != 1 {
		t.Fatalf("stuck = %d, want 1", len(stuck))
	}
	if stuck[0].StepRunID != 2 {
		t.Fatalf("stuck task = %d, want 2", stuck[0].StepRunID)
	}
	if overdue := stuck[0].Overdue(now); overdue < 24*time.Minute || overdue > 26*time.Minute {
		t.Fatalf("overdue = %s, want about 25m", overdue)
	}
}

func TestStuckTasksSurfaceInApiViews(t *testing.T) {
	svc := testServices(t)
	seedFlow(t, svc)
	now := time.Now().UTC()

	// `slow` has a 1 minute SLA → starting it 10 minutes ago makes it stuck.
	res := send(t, svc, event("start", "ORD-STUCK", "started", now.Add(-12*time.Minute)))
	send(t, svc, event("slow", "ORD-STUCK", "started", now.Add(-10*time.Minute)))

	stuck, err := svc.Query.ListStuck(10)
	if err != nil {
		t.Fatal(err)
	}
	if len(stuck) != 1 {
		t.Fatalf("stuck tasks = %d, want 1", len(stuck))
	}
	if stuck[0].NodeKey != "slow" || !stuck[0].Stuck {
		t.Fatalf("unexpected stuck task: %+v", stuck[0])
	}
	if stuck[0].TimeoutMinutes != 1 {
		t.Fatalf("timeout = %d, want 1", stuck[0].TimeoutMinutes)
	}
	if stuck[0].OverdueMs <= 0 {
		t.Fatalf("overdue = %d, want > 0", stuck[0].OverdueMs)
	}

	steps, err := svc.Query.ListSteps(StepFilter{FlowKey: "order", StuckOnly: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(steps.Items) != 1 {
		t.Fatalf("steps stuck filter returned %d, want 1", len(steps.Items))
	}

	nodeStats, err := svc.Query.FlowNodeStats("order")
	if err != nil {
		t.Fatal(err)
	}
	byKey := map[string]dto.FlowNodeDTO{}
	for _, node := range nodeStats {
		byKey[node.Key] = node
	}
	if byKey["slow"].Stats.Stuck != 1 || byKey["slow"].Stats.Running != 1 {
		t.Fatalf("slow stats = %+v, want running=1 stuck=1", byKey["slow"].Stats)
	}
	if byKey["pay"].Stats.Running != 0 {
		t.Fatalf("pay stats = %+v, want no running tasks", byKey["pay"].Stats)
	}

	// The run view should flag the execution as stuck.
	run, err := svc.Query.GetRun(res.RunID)
	if err != nil {
		t.Fatal(err)
	}
	if !run.Stuck {
		t.Fatal("run should be flagged as stuck")
	}
	if run.ElapsedMs <= 0 || run.Status != string(models.RunRunning) {
		t.Fatalf("unexpected run summary: %+v", run.RunSummary)
	}

	stuckRuns, err := svc.Query.ListRuns(RunFilter{Status: "stuck"})
	if err != nil {
		t.Fatal(err)
	}
	if stuckRuns.Total != 1 {
		t.Fatalf("stuck runs = %d, want 1", stuckRuns.Total)
	}
}

func TestWatcherRaisesAndResolvesStuckAlerts(t *testing.T) {
	svc := testServices(t)
	seedFlow(t, svc)
	now := time.Now().UTC()

	res := send(t, svc, event("start", "ORD-WATCH", "started", now.Add(-12*time.Minute)))
	step := send(t, svc, event("slow", "ORD-WATCH", "started", now.Add(-10*time.Minute)))

	watcher := NewWatcher(svc.DB, svc.Cfg)
	watcher.Scan(now)

	var alert models.Alert
	if err := svc.DB.Where("dedupe_key = ?", stuckKey(step.StepRunID)).First(&alert).Error; err != nil {
		t.Fatalf("stuck alert not raised: %v", err)
	}
	if alert.Type != models.AlertStuck || alert.Resolved {
		t.Fatalf("unexpected alert: %+v", alert)
	}

	// Scanning again must not duplicate the alert.
	watcher.Scan(now)
	var count int64
	svc.DB.Model(&models.Alert{}).Where("dedupe_key = ?", stuckKey(step.StepRunID)).Count(&count)
	if count != 1 {
		t.Fatalf("alerts = %d, want 1 (deduplicated)", count)
	}

	// Finishing the task resolves the alert.
	send(t, svc, event("slow", "ORD-WATCH", "completed", now))
	watcher.Scan(now)

	var resolved models.Alert
	if err := svc.DB.First(&resolved, alert.ID).Error; err != nil {
		t.Fatal(err)
	}
	if !resolved.Resolved || resolved.ResolvedAt == nil {
		t.Fatalf("alert should be resolved: %+v", resolved)
	}

	run := loadRun(t, svc, "ORD-WATCH")
	if run.Status != models.RunRunning {
		t.Fatalf("run status = %s, want running", run.Status)
	}
	_ = res
}

// ---------------------------------------------------------------------------
// history & dashboard
// ---------------------------------------------------------------------------

func TestBusinessKeyHistoryKeepsRetries(t *testing.T) {
	svc := testServices(t)
	seedFlow(t, svc)
	base := time.Now().UTC().Add(-2 * time.Hour)

	// first attempt fails
	send(t, svc, event("start", "ORD-RETRY", "started", base))
	send(t, svc, event("pay", "ORD-RETRY", "started", base.Add(time.Second)))
	send(t, svc, event("pay", "ORD-RETRY", "failed", base.Add(2*time.Minute)))

	// retry creates a second execution for the same business key
	send(t, svc, event("start", "ORD-RETRY", "started", base.Add(time.Hour)))
	send(t, svc, event("pay", "ORD-RETRY", "started", base.Add(time.Hour+time.Second)))
	send(t, svc, event("pay", "ORD-RETRY", "completed", base.Add(time.Hour+30*time.Second)))
	send(t, svc, event("slow", "ORD-RETRY", "started", base.Add(time.Hour+31*time.Second)))
	send(t, svc, event("slow", "ORD-RETRY", "completed", base.Add(time.Hour+32*time.Second)))
	send(t, svc, event("done", "ORD-RETRY", "started", base.Add(time.Hour+33*time.Second)))
	send(t, svc, event("done", "ORD-RETRY", "completed", base.Add(time.Hour+34*time.Second)))

	history, err := svc.Query.BusinessKeyHistory("ORD-RETRY", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(history) != 2 {
		t.Fatalf("history = %d runs, want 2 (failed attempt + retry)", len(history))
	}
	newest, oldest := history[0], history[1]
	if newest.Status != string(models.RunCompleted) {
		t.Fatalf("newest run status = %s, want completed", newest.Status)
	}
	if oldest.Status != string(models.RunFailed) {
		t.Fatalf("oldest run status = %s, want failed", oldest.Status)
	}
	if len(newest.Steps) != 4 || len(oldest.Steps) != 2 {
		t.Fatalf("step counts = %d/%d, want 4/2", len(newest.Steps), len(oldest.Steps))
	}

	// per-node history is available too ("what happened on this step before")
	stepID := newest.Steps[1].ID
	detail, err := svc.Query.GetStep(stepID)
	if err != nil {
		t.Fatal(err)
	}
	if detail.Step.NodeKey != "pay" {
		t.Fatalf("step node = %s, want pay", detail.Step.NodeKey)
	}
	if len(detail.History) != 1 {
		t.Fatalf("node history = %d, want 1 earlier execution", len(detail.History))
	}
	if detail.Node == nil || detail.Node.Endpoint != "/payments/authorize" {
		t.Fatalf("node contract not attached: %+v", detail.Node)
	}
	if len(detail.Previous) != 1 {
		t.Fatalf("previous steps in run = %d, want 1", len(detail.Previous))
	}
}

func TestDashboardAndFlowDefinition(t *testing.T) {
	svc := testServices(t)
	seedFlow(t, svc)
	now := time.Now().UTC()

	send(t, svc, event("start", "ORD-D1", "started", now.Add(-20*time.Minute)))
	send(t, svc, event("pay", "ORD-D1", "started", now.Add(-19*time.Minute)))
	send(t, svc, event("pay", "ORD-D1", "completed", now.Add(-15*time.Minute)))
	send(t, svc, event("slow", "ORD-D1", "started", now.Add(-15*time.Minute)))
	send(t, svc, event("slow", "ORD-D1", "completed", now.Add(-14*time.Minute)))
	send(t, svc, event("done", "ORD-D1", "started", now.Add(-14*time.Minute)))
	send(t, svc, event("done", "ORD-D1", "completed", now.Add(-13*time.Minute)))

	board, err := svc.Query.Dashboard()
	if err != nil {
		t.Fatal(err)
	}
	if board.KPIs.Completed24h != 1 {
		t.Fatalf("completed24h = %d, want 1", board.KPIs.Completed24h)
	}
	if board.KPIs.SuccessRate != 1 {
		t.Fatalf("success rate = %v, want 1", board.KPIs.SuccessRate)
	}
	if board.KPIs.AvgCycleTimeMs <= 0 {
		t.Fatalf("avg cycle time = %d, want > 0", board.KPIs.AvgCycleTimeMs)
	}
	if len(board.Flows) != 1 || board.Flows[0].Key != "order" {
		t.Fatalf("unexpected flow summaries: %+v", board.Flows)
	}
	if board.Flows[0].NodeCount != 4 {
		t.Fatalf("node count = %d, want 4", board.Flows[0].NodeCount)
	}
	if len(board.Throughput) == 0 || len(board.Trend) != 7 {
		t.Fatalf("charts missing: throughput=%d trend=%d", len(board.Throughput), len(board.Trend))
	}
	if len(board.RecentRuns) != 1 {
		t.Fatalf("recent runs = %d, want 1", len(board.RecentRuns))
	}

	detail, err := svc.Query.GetFlow("order")
	if err != nil {
		t.Fatal(err)
	}
	if len(detail.Nodes) != 4 || len(detail.Edges) != 3 {
		t.Fatalf("nodes/edges = %d/%d, want 4/3", len(detail.Nodes), len(detail.Edges))
	}
	if len(detail.NodeContracts) != 4 {
		t.Fatalf("contracts = %d, want 4", len(detail.NodeContracts))
	}
	// 24h stats per node
	for _, node := range detail.Nodes {
		if node.Key == "pay" {
			if node.Stats.Total24h != 1 || node.Stats.AvgDurationMs != int64(4*time.Minute/time.Millisecond) {
				t.Fatalf("pay stats = %+v", node.Stats)
			}
		}
	}
}

func TestSaveFlowReplacesDefinition(t *testing.T) {
	svc := testServices(t)
	flow := seedFlow(t, svc)

	input := dto.FlowInput{
		Key: "order", Name: "Order (edited)", Category: "Sales", Version: "2.0.0",
		Nodes: []dto.FlowNodeInput{
			{Key: "start", Name: "Received", Kind: "start", PosX: 10, PosY: 20, TimeoutMinutes: 5},
			{Key: "pay", Name: "Pay (edited)", Kind: "task", System: "payment-service", Endpoint: "/v2/payments", TimeoutMinutes: 7,
				RequestSchema: []byte(`{"amount":1}`), ResponseSample: []byte(`{"ok":true}`)},
			{Key: "done", Name: "Done", Kind: "end", TimeoutMinutes: 5},
		},
		Edges: []dto.FlowEdgeInput{
			{SourceKey: "start", TargetKey: "pay"},
			{SourceKey: "pay", TargetKey: "done"},
		},
	}
	detail, err := svc.Query.SaveFlow(fmt.Sprint(flow.ID), input)
	if err != nil {
		t.Fatal(err)
	}
	if detail.Name != "Order (edited)" || len(detail.Nodes) != 3 || len(detail.Edges) != 2 {
		t.Fatalf("unexpected detail: name=%s nodes=%d edges=%d", detail.Name, len(detail.Nodes), len(detail.Edges))
	}

	// `slow` was removed from the definition.
	var count int64
	svc.DB.Model(&models.FlowNode{}).Where("flow_id = ? AND key = ?", flow.ID, "slow").Count(&count)
	if count != 0 {
		t.Fatal("removed node should be deleted")
	}

	// invalid definitions are rejected
	if _, err := svc.Query.SaveFlow(fmt.Sprint(flow.ID), dto.FlowInput{Key: "order", Name: "x"}); err == nil {
		t.Fatal("expected validation error for a flow without nodes")
	}
	if _, err := svc.Query.SaveFlow(fmt.Sprint(flow.ID), dto.FlowInput{
		Key: "order", Name: "x",
		Nodes: []dto.FlowNodeInput{{Key: "a"}},
		Edges: []dto.FlowEdgeInput{{SourceKey: "a", TargetKey: "ghost"}},
	}); err == nil {
		t.Fatal("expected validation error for an edge referencing a missing node")
	}
}

func TestAutoRegistrationKeepsTelemetry(t *testing.T) {
	svc := testServices(t)
	now := time.Now().UTC()

	res, err := svc.Ingest.Process(context.Background(), dto.IngestEvent{
		FlowKey: "brand-new-flow", NodeKey: "some_step", BusinessKey: "BK-1",
		Status: "started", OccurredAt: &now, Source: "new-service",
	})
	if err != nil {
		t.Fatalf("auto-registration should accept unknown flows: %v", err)
	}
	if !res.AutoCreated {
		t.Fatal("result should report auto-created flow/node")
	}

	detail, err := svc.Query.GetFlow("brand-new-flow")
	if err != nil {
		t.Fatal(err)
	}
	if len(detail.Nodes) != 1 || detail.Nodes[0].Kind != "task" {
		t.Fatalf("expected one auto-registered task node, got %+v", detail.Nodes)
	}

	// Auto-registration can be switched off.
	off := testServices(t)
	off.Cfg.AutoRegister = false
	if _, err := off.Ingest.Process(context.Background(), dto.IngestEvent{
		FlowKey: "unknown", NodeKey: "n", BusinessKey: "b", Status: "started", OccurredAt: &now,
	}); err == nil {
		t.Fatal("expected error when auto-registration is disabled")
	}
}

func ptr(t time.Time) *time.Time { return &t }
