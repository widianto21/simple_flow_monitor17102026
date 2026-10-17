// Package dto holds the transport shapes of the HTTP API (camelCase JSON) and
// the ingestion event contract.
package dto

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// ---------------------------------------------------------------------------
// Ingestion (external systems → FlowMonitor)
// ---------------------------------------------------------------------------

// IngestEvent is the payload other systems POST to /api/ingest/events.
type IngestEvent struct {
	FlowKey     string          `json:"flowKey"`
	NodeKey     string          `json:"nodeKey"`
	BusinessKey string          `json:"businessKey"`
	Status      string          `json:"status"`
	OccurredAt  *time.Time      `json:"occurredAt"`
	DurationMs  int64           `json:"durationMs"`
	Attempt     int             `json:"attempt"`
	Source      string          `json:"source"`
	Actor       string          `json:"actor"`
	InitiatedBy string          `json:"initiatedBy"`
	Request     json.RawMessage `json:"request"`
	Response    json.RawMessage `json:"response"`
	Error       string          `json:"error"`
	Metadata    map[string]any  `json:"metadata"`
}

var statusAliases = map[string]string{
	"start":       "started",
	"started":     "started",
	"in_progress": "started",
	"running":     "started",
	"complete":    "completed",
	"completed":   "completed",
	"done":        "completed",
	"success":     "completed",
	"finish":      "completed",
	"failed":      "failed",
	"fail":        "failed",
	"error":       "failed",
	"skipped":     "skipped",
	"skip":        "skipped",
}

// Normalize trims fields and applies documented defaults/aliases.
func (e *IngestEvent) Normalize() {
	e.FlowKey = strings.TrimSpace(e.FlowKey)
	e.NodeKey = strings.TrimSpace(e.NodeKey)
	e.BusinessKey = strings.TrimSpace(e.BusinessKey)
	e.Status = statusAliases[strings.ToLower(strings.TrimSpace(e.Status))]
	if e.OccurredAt == nil {
		now := time.Now().UTC()
		e.OccurredAt = &now
	} else {
		t := e.OccurredAt.UTC()
		e.OccurredAt = &t
	}
	if e.Attempt < 1 {
		e.Attempt = 1
	}
	if strings.TrimSpace(e.InitiatedBy) == "" {
		e.InitiatedBy = e.Actor
	}
}

// Validate reports whether the event carries the mandatory correlation fields.
func (e *IngestEvent) Validate() error {
	switch {
	case e.FlowKey == "":
		return fmt.Errorf("flowKey is required")
	case e.NodeKey == "":
		return fmt.Errorf("nodeKey is required")
	case e.BusinessKey == "":
		return fmt.Errorf("businessKey is required")
	}
	switch e.Status {
	case "started", "completed", "failed", "skipped":
		return nil
	default:
		return fmt.Errorf("status %q is invalid (expected started|completed|failed|skipped)", e.Status)
	}
}

// DedupeHash identifies a delivery so at-least-once senders stay idempotent.
func (e *IngestEvent) DedupeHash() string {
	raw := strings.Join([]string{
		e.FlowKey,
		e.BusinessKey,
		e.NodeKey,
		e.Status,
		e.OccurredAt.UTC().Format(time.RFC3339Nano),
		fmt.Sprintf("%d", e.Attempt),
	}, "|")
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}

// IngestResult is returned for each accepted event.
type IngestResult struct {
	FlowKey     string `json:"flowKey"`
	NodeKey     string `json:"nodeKey"`
	BusinessKey string `json:"businessKey"`
	Status      string `json:"status"`
	Accepted    bool   `json:"accepted"`
	Duplicate   bool   `json:"duplicate"`
	RunID       uint   `json:"runId,omitempty"`
	StepRunID   uint   `json:"stepRunId,omitempty"`
	RunStatus   string `json:"runStatus,omitempty"`
	StepStatus  string `json:"stepStatus,omitempty"`
	AutoCreated bool   `json:"autoCreated,omitempty"` // flow/node was auto-registered
	Message     string `json:"message,omitempty"`
}

// ---------------------------------------------------------------------------
// Definition shapes
// ---------------------------------------------------------------------------

// NodeStats is the live + historical view of one node.
type NodeStats struct {
	NodeKey       string     `json:"nodeKey"`
	Running       int        `json:"running"`
	Stuck         int        `json:"stuck"`
	Total24h      int        `json:"total24h"`
	Failed24h     int        `json:"failed24h"`
	AvgDurationMs int64      `json:"avgDurationMs"`
	P95DurationMs int64      `json:"p95DurationMs"`
	FailureRate   float64    `json:"failureRate"`
	MaxElapsedMs  int64      `json:"maxElapsedMs"` // longest currently-running task
	LastActivity  *time.Time `json:"lastActivityAt"`
}

// FlowNodeDTO is a node plus its live stats.
type FlowNodeDTO struct {
	ID             uint            `json:"id"`
	Key            string          `json:"key"`
	Name           string          `json:"name"`
	Kind           string          `json:"kind"`
	Description    string          `json:"description"`
	System         string          `json:"system"`
	Repository     string          `json:"repository"`
	Team           string          `json:"team"`
	Method         string          `json:"method"`
	Endpoint       string          `json:"endpoint"`
	RequestSchema  json.RawMessage `json:"requestSchema"`
	ResponseSample json.RawMessage `json:"responseSample"`
	TimeoutMinutes int             `json:"timeoutMinutes"`
	PosX           float64         `json:"posX"`
	PosY           float64         `json:"posY"`
	Stats          NodeStats       `json:"stats"`
}

// FlowEdgeDTO is an edge of the diagram.
type FlowEdgeDTO struct {
	ID        uint   `json:"id"`
	SourceKey string `json:"sourceKey"`
	TargetKey string `json:"targetKey"`
	Label     string `json:"label"`
	Condition string `json:"condition"`
}

// FlowSummary is a flow plus its live counters.
type FlowSummary struct {
	ID            uint       `json:"id"`
	Key           string     `json:"key"`
	Name          string     `json:"name"`
	Description   string     `json:"description"`
	Category      string     `json:"category"`
	Owner         string     `json:"owner"`
	Version       string     `json:"version"`
	Active        bool       `json:"active"`
	NodeCount     int        `json:"nodeCount"`
	Running       int        `json:"running"`      // executions currently running
	RunningTasks  int        `json:"runningTasks"` // tasks currently running
	StuckTasks    int        `json:"stuckTasks"`
	Completed24h  int        `json:"completed24h"`
	Failed24h     int        `json:"failed24h"`
	SuccessRate   float64    `json:"successRate"`
	AvgDurationMs int64      `json:"avgDurationMs"`
	LastActivity  *time.Time `json:"lastActivityAt"`
	UpdatedAt     time.Time  `json:"updatedAt"`
}

// FlowDetail is the full definition used by the diagram (monitor + editor).
type FlowDetail struct {
	FlowSummary
	Nodes []FlowNodeDTO `json:"nodes"`
	Edges []FlowEdgeDTO `json:"edges"`

	// NodeContracts is the ingestion manifest (same data, keyed by node).
	NodeContracts []NodeContract `json:"nodeContracts"`
}

// NodeContract tells an instrumenting developer what to log for a step.
type NodeContract struct {
	Key            string          `json:"key"`
	Name           string          `json:"name"`
	Kind           string          `json:"kind"`
	System         string          `json:"system"`
	Repository     string          `json:"repository"`
	Team           string          `json:"team"`
	Method         string          `json:"method"`
	Endpoint       string          `json:"endpoint"`
	RequestSchema  json.RawMessage `json:"requestSchema"`
	ResponseSample json.RawMessage `json:"responseSample"`
	TimeoutMinutes int             `json:"timeoutMinutes"`
}

// FlowInput is the body of POST/PUT /api/flows (create/update definition).
type FlowInput struct {
	Key         string          `json:"key"`
	Name        string          `json:"name"`
	Description string          `json:"description"`
	Category    string          `json:"category"`
	Owner       string          `json:"owner"`
	Version     string          `json:"version"`
	Active      *bool           `json:"active"`
	Nodes       []FlowNodeInput `json:"nodes"`
	Edges       []FlowEdgeInput `json:"edges"`
}

// FlowNodeInput is a node in a create/update request.
type FlowNodeInput struct {
	Key            string          `json:"key"`
	Name           string          `json:"name"`
	Kind           string          `json:"kind"`
	Description    string          `json:"description"`
	System         string          `json:"system"`
	Repository     string          `json:"repository"`
	Team           string          `json:"team"`
	Method         string          `json:"method"`
	Endpoint       string          `json:"endpoint"`
	RequestSchema  json.RawMessage `json:"requestSchema"`
	ResponseSample json.RawMessage `json:"responseSample"`
	TimeoutMinutes int             `json:"timeoutMinutes"`
	PosX           float64         `json:"posX"`
	PosY           float64         `json:"posY"`
}

// FlowEdgeInput is an edge in a create/update request.
type FlowEdgeInput struct {
	SourceKey string `json:"sourceKey"`
	TargetKey string `json:"targetKey"`
	Label     string `json:"label"`
	Condition string `json:"condition"`
}

// ---------------------------------------------------------------------------
// Telemetry shapes
// ---------------------------------------------------------------------------

// RunSummary is one execution in list views.
type RunSummary struct {
	ID             uint            `json:"id"`
	FlowID         uint            `json:"flowId"`
	FlowKey        string          `json:"flowKey"`
	FlowName       string          `json:"flowName"`
	BusinessKey    string          `json:"businessKey"`
	Status         string          `json:"status"`
	CurrentNodeKey string          `json:"currentNodeKey"`
	CurrentNode    string          `json:"currentNodeName"`
	StartedAt      time.Time       `json:"startedAt"`
	CompletedAt    *time.Time      `json:"completedAt"`
	DurationMs     int64           `json:"durationMs"`
	ElapsedMs      int64           `json:"elapsedMs"`
	InitiatedBy    string          `json:"initiatedBy"`
	FailureError   string          `json:"failureError"`
	StepCount      int             `json:"stepCount"`
	Stuck          bool            `json:"stuck"`
	Metadata       json.RawMessage `json:"metadata"`
}

// StepRunDTO is one task execution, with payloads for inspection.
type StepRunDTO struct {
	ID              uint            `json:"id"`
	RunID           uint            `json:"runId"`
	FlowID          uint            `json:"flowId"`
	FlowKey         string          `json:"flowKey"`
	FlowName        string          `json:"flowName"`
	BusinessKey     string          `json:"businessKey"`
	RunStatus       string          `json:"runStatus"`
	NodeKey         string          `json:"nodeKey"`
	NodeName        string          `json:"nodeName"`
	NodeKind        string          `json:"nodeKind"`
	Status          string          `json:"status"`
	Sequence        int             `json:"sequence"`
	Attempt         int             `json:"attempt"`
	StartedAt       time.Time       `json:"startedAt"`
	CompletedAt     *time.Time      `json:"completedAt"`
	DurationMs      int64           `json:"durationMs"`
	ElapsedMs       int64           `json:"elapsedMs"`
	RequestPayload  json.RawMessage `json:"request"`
	ResponsePayload json.RawMessage `json:"response"`
	Error           string          `json:"error"`
	Source          string          `json:"source"`
	Actor           string          `json:"actor"`
	Metadata        json.RawMessage `json:"metadata"`

	// Enrichment for "stuck" views and drill-downs.
	Stuck          bool   `json:"stuck"`
	TimeoutMinutes int    `json:"timeoutMinutes"`
	OverdueMs      int64  `json:"overdueMs"`
	System         string `json:"system,omitempty"`
}

// EventDTO is a raw ingested event (audit trail).
type EventDTO struct {
	ID         uint            `json:"id"`
	NodeKey    string          `json:"nodeKey"`
	Status     string          `json:"status"`
	OccurredAt time.Time       `json:"occurredAt"`
	ReceivedAt time.Time       `json:"receivedAt"`
	Source     string          `json:"source"`
	Payload    json.RawMessage `json:"payload"`
	Error      string          `json:"error,omitempty"`
}

// AlertDTO is an alert in list views.
type AlertDTO struct {
	ID             uint       `json:"id"`
	FlowID         uint       `json:"flowId"`
	FlowKey        string     `json:"flowKey"`
	FlowName       string     `json:"flowName"`
	RunID          uint       `json:"runId"`
	BusinessKey    string     `json:"businessKey"`
	NodeKey        string     `json:"nodeKey"`
	Type           string     `json:"type"`
	Severity       string     `json:"severity"`
	Message        string     `json:"message"`
	CreatedAt      time.Time  `json:"createdAt"`
	Acknowledged   bool       `json:"acknowledged"`
	AcknowledgedAt *time.Time `json:"acknowledgedAt"`
	Resolved       bool       `json:"resolved"`
	ResolvedAt     *time.Time `json:"resolvedAt"`
}

// RunDetail is the full drill-down of one execution.
type RunDetail struct {
	RunSummary
	Steps        []StepRunDTO `json:"steps"`
	Events       []EventDTO   `json:"events"`
	Alerts       []AlertDTO   `json:"alerts"`
	PreviousRuns []RunSummary `json:"previousRuns"` // earlier executions of the same business key
}

// StepDetail is the drill-down of one task execution.
type StepDetail struct {
	Step     StepRunDTO    `json:"step"`
	Run      RunSummary    `json:"run"`
	Previous []StepRunDTO  `json:"previousSteps"` // earlier steps in the same run
	Next     []StepRunDTO  `json:"nextSteps"`     // later steps in the same run
	History  []StepRunDTO  `json:"nodeHistory"`   // recent executions of this node (other runs)
	Node     *NodeContract `json:"node,omitempty"`
}

// ListResponse is the standard paginated envelope.
type ListResponse[T any] struct {
	Items []T   `json:"items"`
	Total int64 `json:"total"`
	Page  int   `json:"page"`
	Size  int   `json:"size"`
}

// ---------------------------------------------------------------------------
// Dashboard
// ---------------------------------------------------------------------------

// KPIs is the headline numbers of the dashboard.
type KPIs struct {
	RunningTasks      int     `json:"runningTasks"`
	StuckTasks        int     `json:"stuckTasks"`
	RunningExecutions int     `json:"runningExecutions"`
	Completed24h      int     `json:"completed24h"`
	Failed24h         int     `json:"failed24h"`
	AvgCycleTimeMs    int64   `json:"avgCycleTimeMs"`
	SuccessRate       float64 `json:"successRate"`
	OpenAlerts        int     `json:"openAlerts"`
	Flows             int     `json:"flows"`
}

// ThroughputPoint is one hour bucket of the 24h throughput chart.
type ThroughputPoint struct {
	Hour      time.Time `json:"hour"`
	Started   int       `json:"started"`
	Completed int       `json:"completed"`
	Failed    int       `json:"failed"`
}

// TrendPoint is one day bucket of the 7d trend chart.
type TrendPoint struct {
	Day           time.Time `json:"day"`
	Completed     int       `json:"completed"`
	Failed        int       `json:"failed"`
	SuccessRate   float64   `json:"successRate"`
	AvgDurationMs int64     `json:"avgDurationMs"`
}

// StatusCount is a slice of the status breakdown chart.
type StatusCount struct {
	Status string `json:"status"`
	Count  int    `json:"count"`
}

// Dashboard is the whole dashboard payload in one round-trip.
type Dashboard struct {
	GeneratedAt     time.Time         `json:"generatedAt"`
	KPIs            KPIs              `json:"kpis"`
	Throughput      []ThroughputPoint `json:"throughput"`
	Trend           []TrendPoint      `json:"trend"`
	StatusBreakdown []StatusCount     `json:"statusBreakdown"`
	Flows           []FlowSummary     `json:"flows"`
	RecentRuns      []RunSummary      `json:"recentRuns"`
	Stuck           []StepRunDTO      `json:"stuck"`
	RecentAlerts    []AlertDTO        `json:"recentAlerts"`
}
