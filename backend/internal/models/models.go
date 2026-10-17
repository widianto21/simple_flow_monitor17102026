package models

import "time"

// ---------------------------------------------------------------------------
// Enumerations
// ---------------------------------------------------------------------------

// NodeKind describes the role of a node in the business flow diagram.
type NodeKind string

const (
	KindStart    NodeKind = "start"
	KindTask     NodeKind = "task"
	KindDecision NodeKind = "decision"
	KindGateway  NodeKind = "gateway"
	KindExternal NodeKind = "external"
	KindEnd      NodeKind = "end"
)

// RunStatus is the status of a whole flow execution.
type RunStatus string

const (
	RunRunning   RunStatus = "running"
	RunCompleted RunStatus = "completed"
	RunFailed    RunStatus = "failed"
	RunCancelled RunStatus = "cancelled"
)

// StepStatus is the status of a single task (node execution).
type StepStatus string

const (
	StepRunning   StepStatus = "running"
	StepCompleted StepStatus = "completed"
	StepFailed    StepStatus = "failed"
	StepSkipped   StepStatus = "skipped"
)

// AlertType classifies an alert.
type AlertType string

const (
	AlertStuck   AlertType = "stuck"
	AlertFailure AlertType = "failure"
)

// ---------------------------------------------------------------------------
// Definition (what the business flow looks like)
// ---------------------------------------------------------------------------

// Flow is a business flow definition.
type Flow struct {
	ID          uint      `gorm:"primaryKey" json:"id"`
	Key         string    `gorm:"uniqueIndex;size:96" json:"key"` // machine name used by instrumentation
	Name        string    `gorm:"size:160" json:"name"`
	Description string    `json:"description"`
	Category    string    `gorm:"size:64" json:"category"`
	Owner       string    `gorm:"size:128" json:"owner"`
	Version     string    `gorm:"size:32" json:"version"`
	Active      bool      `json:"active"`
	CreatedAt   time.Time `json:"createdAt"`
	UpdatedAt   time.Time `json:"updatedAt"`

	Nodes []FlowNode `gorm:"foreignKey:FlowID;constraint:OnDelete:CASCADE" json:"nodes,omitempty"`
	Edges []FlowEdge `gorm:"foreignKey:FlowID;constraint:OnDelete:CASCADE" json:"edges,omitempty"`
}

// FlowNode is one process step in the diagram, including the "rich" contract
// metadata shown when a user hovers/clicks the component.
type FlowNode struct {
	ID     uint     `gorm:"primaryKey" json:"id"`
	FlowID uint     `gorm:"uniqueIndex:idx_flow_node_key;index" json:"flowId"`
	Key    string   `gorm:"uniqueIndex:idx_flow_node_key;size:96" json:"key"`
	Name   string   `gorm:"size:160" json:"name"`
	Kind   NodeKind `gorm:"size:24" json:"kind"`

	Description string `json:"description"`
	System      string `gorm:"size:96" json:"system"`      // which system/service runs this step
	Repository  string `gorm:"size:200" json:"repository"` // where the code lives
	Team        string `gorm:"size:96" json:"team"`        // owning team
	Method      string `gorm:"size:12" json:"method"`      // HTTP method of the step, if callable
	Endpoint    string `gorm:"size:200" json:"endpoint"`   // e.g. POST /payments/authorize

	RequestSchema  string `gorm:"type:text" json:"-"` // JSON: parameters the step accepts (example/schema)
	ResponseSample string `gorm:"type:text" json:"-"` // JSON: sample response of the step

	TimeoutMinutes int     `json:"timeoutMinutes"` // SLA: beyond this a running task is "stuck"
	PosX           float64 `json:"posX"`
	PosY           float64 `json:"posY"`

	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

// FlowEdge connects two nodes; Label/Condition drive decision branches.
type FlowEdge struct {
	ID        uint    `gorm:"primaryKey" json:"id"`
	FlowID    uint    `gorm:"index" json:"flowId"`
	SourceKey string  `gorm:"size:96" json:"sourceKey"`
	TargetKey string  `gorm:"size:96" json:"targetKey"`
	Label     string  `gorm:"size:64" json:"label"`
	Condition string  `gorm:"size:200" json:"condition"`
	PosX      float64 `json:"posX,omitempty"` // optional manual label placement
	PosY      float64 `json:"posY,omitempty"`
}

// ---------------------------------------------------------------------------
// Telemetry (what actually happened)
// ---------------------------------------------------------------------------

// Run is one execution of a business flow for a business key.
type Run struct {
	ID             uint       `gorm:"primaryKey" json:"id"`
	FlowID         uint       `gorm:"index" json:"flowId"`
	BusinessKey    string     `gorm:"index;size:128" json:"businessKey"`
	Status         RunStatus  `gorm:"size:24;index" json:"status"`
	CurrentNodeKey string     `gorm:"size:96" json:"currentNodeKey"`
	StartedAt      time.Time  `gorm:"index" json:"startedAt"`
	CompletedAt    *time.Time `json:"completedAt"`
	DurationMs     int64      `json:"durationMs"`
	InitiatedBy    string     `gorm:"size:128" json:"initiatedBy"`
	Metadata       string     `gorm:"type:text" json:"-"`
	FailureError   string     `json:"failureError"`

	Steps []StepRun `gorm:"foreignKey:RunID" json:"steps,omitempty"`
}

// StepRun is one execution of a node ("task") inside a run.
type StepRun struct {
	ID       uint       `gorm:"primaryKey" json:"id"`
	RunID    uint       `gorm:"index" json:"runId"`
	FlowID   uint       `gorm:"index" json:"flowId"`
	NodeKey  string     `gorm:"index;size:96" json:"nodeKey"`
	NodeName string     `gorm:"size:160" json:"nodeName"`
	NodeKind NodeKind   `gorm:"size:24" json:"nodeKind"`
	Status   StepStatus `gorm:"size:24;index" json:"status"`

	Sequence    int        `gorm:"index" json:"sequence"`
	Attempt     int        `json:"attempt"`
	StartedAt   time.Time  `gorm:"index" json:"startedAt"`
	CompletedAt *time.Time `json:"completedAt"`
	DurationMs  int64      `json:"durationMs"`

	RequestPayload  string `gorm:"type:text" json:"-"`
	ResponsePayload string `gorm:"type:text" json:"-"`
	Error           string `json:"error"`
	Source          string `gorm:"size:96" json:"source"`
	Actor           string `gorm:"size:128" json:"actor"`
	Metadata        string `gorm:"type:text" json:"-"`

	CreatedAt time.Time `json:"createdAt"`
}

// Event is the raw, append-only ingestion log (audit + replay + debugging).
type Event struct {
	ID          uint      `gorm:"primaryKey" json:"id"`
	FlowKey     string    `gorm:"index;size:96" json:"flowKey"`
	NodeKey     string    `gorm:"index;size:96" json:"nodeKey"`
	BusinessKey string    `gorm:"index;size:128" json:"businessKey"`
	Status      string    `gorm:"size:24;index" json:"status"`
	OccurredAt  time.Time `gorm:"index" json:"occurredAt"`
	ReceivedAt  time.Time `json:"receivedAt"`
	Source      string    `gorm:"size:96" json:"source"`
	RunID       uint      `gorm:"index" json:"runId"`
	StepRunID   uint      `gorm:"index" json:"stepRunId"`
	Payload     string    `gorm:"type:text" json:"-"`
	Error       string    `json:"error,omitempty"`
	DedupeHash  string    `gorm:"uniqueIndex;size:64" json:"-"`
}

// Alert is raised for stuck tasks and failures; it auto-resolves when the
// underlying condition clears.
type Alert struct {
	ID       uint      `gorm:"primaryKey" json:"id"`
	FlowID   uint      `gorm:"index" json:"flowId"`
	FlowKey  string    `gorm:"size:96" json:"flowKey"`
	RunID    uint      `gorm:"index" json:"runId"`
	NodeKey  string    `gorm:"size:96" json:"nodeKey"`
	Type     AlertType `gorm:"size:24;index" json:"type"`
	Severity string    `gorm:"size:16" json:"severity"` // info | warning | critical
	Message  string    `json:"message"`

	CreatedAt      time.Time  `gorm:"index" json:"createdAt"`
	Acknowledged   bool       `json:"acknowledged"`
	AcknowledgedAt *time.Time `json:"acknowledgedAt"`
	Resolved       bool       `gorm:"index" json:"resolved"`
	ResolvedAt     *time.Time `json:"resolvedAt"`
	DedupeKey      string     `gorm:"uniqueIndex;size:128" json:"-"`
}
