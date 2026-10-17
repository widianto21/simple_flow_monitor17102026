// ---------------------------------------------------------------------------
// Transport shapes — mirror backend/internal/dto (camelCase JSON).
// Keep in sync whenever a DTO changes on the Go side.
// ---------------------------------------------------------------------------

/** Free-form JSON value as produced by Go's json.RawMessage (may be null). */
export type Json = unknown;

export type NodeKind = "start" | "task" | "decision" | "gateway" | "external" | "end";
export type RunStatus = "running" | "completed" | "failed" | "cancelled";
export type StepStatus = "running" | "completed" | "failed" | "skipped";
export type AlertType = "stuck" | "failure";
export type AlertSeverity = "info" | "warning" | "critical";

// --- definition -----------------------------------------------------------

export interface NodeStats {
  nodeKey: string;
  running: number;
  stuck: number;
  total24h: number;
  failed24h: number;
  avgDurationMs: number;
  p95DurationMs: number;
  failureRate: number;
  maxElapsedMs: number;
  lastActivityAt: string | null;
}

export interface FlowNode {
  id: number;
  key: string;
  name: string;
  kind: NodeKind;
  description: string;
  system: string;
  repository: string;
  team: string;
  method: string;
  endpoint: string;
  requestSchema: Json;
  responseSample: Json;
  timeoutMinutes: number;
  posX: number;
  posY: number;
  stats: NodeStats;
}

export interface FlowEdge {
  id: number;
  sourceKey: string;
  targetKey: string;
  label: string;
  condition: string;
}

export interface FlowSummary {
  id: number;
  key: string;
  name: string;
  description: string;
  category: string;
  owner: string;
  version: string;
  active: boolean;
  nodeCount: number;
  running: number;
  runningTasks: number;
  stuckTasks: number;
  completed24h: number;
  failed24h: number;
  successRate: number;
  avgDurationMs: number;
  lastActivityAt: string | null;
  updatedAt: string;
}

export interface NodeContract {
  key: string;
  name: string;
  kind: NodeKind;
  system: string;
  repository: string;
  team: string;
  method: string;
  endpoint: string;
  requestSchema: Json;
  responseSample: Json;
  timeoutMinutes: number;
}

export interface FlowDetail extends FlowSummary {
  nodes: FlowNode[];
  edges: FlowEdge[];
  nodeContracts: NodeContract[];
}

export interface FlowNodeInput {
  key: string;
  name: string;
  kind: NodeKind;
  description: string;
  system: string;
  repository: string;
  team: string;
  method: string;
  endpoint: string;
  requestSchema: Json;
  responseSample: Json;
  timeoutMinutes: number;
  posX: number;
  posY: number;
}

export interface FlowEdgeInput {
  sourceKey: string;
  targetKey: string;
  label: string;
  condition: string;
}

export interface FlowInput {
  key: string;
  name: string;
  description: string;
  category: string;
  owner: string;
  version: string;
  active: boolean;
  nodes: FlowNodeInput[];
  edges: FlowEdgeInput[];
}

// --- telemetry ------------------------------------------------------------

export interface RunSummary {
  id: number;
  flowId: number;
  flowKey: string;
  flowName: string;
  businessKey: string;
  status: RunStatus;
  currentNodeKey: string;
  currentNodeName: string;
  startedAt: string;
  completedAt: string | null;
  durationMs: number;
  elapsedMs: number;
  initiatedBy: string;
  failureError: string;
  stepCount: number;
  stuck: boolean;
  metadata: Json;
}

export interface StepRun {
  id: number;
  runId: number;
  flowId: number;
  flowKey: string;
  flowName: string;
  businessKey: string;
  runStatus: RunStatus;
  nodeKey: string;
  nodeName: string;
  nodeKind: NodeKind;
  status: StepStatus;
  sequence: number;
  attempt: number;
  startedAt: string;
  completedAt: string | null;
  durationMs: number;
  elapsedMs: number;
  request: Json;
  response: Json;
  error: string;
  source: string;
  actor: string;
  metadata: Json;
  stuck: boolean;
  timeoutMinutes: number;
  overdueMs: number;
  system?: string;
}

export interface EventItem {
  id: number;
  nodeKey: string;
  status: string;
  occurredAt: string;
  receivedAt: string;
  source: string;
  payload: Json;
  error?: string;
}

export interface Alert {
  id: number;
  flowId: number;
  flowKey: string;
  flowName: string;
  runId: number;
  businessKey: string;
  nodeKey: string;
  type: AlertType;
  severity: AlertSeverity;
  message: string;
  createdAt: string;
  acknowledged: boolean;
  acknowledgedAt: string | null;
  resolved: boolean;
  resolvedAt: string | null;
}

export interface RunDetail extends RunSummary {
  steps: StepRun[];
  events: EventItem[];
  alerts: Alert[];
  previousRuns: RunSummary[];
}

export interface StepDetail {
  step: StepRun;
  run: RunSummary;
  previousSteps: StepRun[];
  nextSteps: StepRun[];
  nodeHistory: StepRun[];
  node?: NodeContract;
}

export interface NodeDetail {
  node: FlowNode;
  tasks: ListResponse<StepRun>;
  recentRuns: ListResponse<RunSummary>;
}

// --- envelopes ------------------------------------------------------------

export interface ListResponse<T> {
  items: T[];
  total: number;
  page: number;
  size: number;
}

/** Some endpoints (stuck tasks, alerts) return just items + total. */
export interface SliceResponse<T> {
  items: T[];
  total: number;
}

// --- dashboard ------------------------------------------------------------

export interface KPIs {
  runningTasks: number;
  stuckTasks: number;
  runningExecutions: number;
  completed24h: number;
  failed24h: number;
  avgCycleTimeMs: number;
  successRate: number;
  openAlerts: number;
  flows: number;
}

export interface ThroughputPoint {
  hour: string;
  started: number;
  completed: number;
  failed: number;
}

export interface TrendPoint {
  day: string;
  completed: number;
  failed: number;
  successRate: number;
  avgDurationMs: number;
}

export interface StatusCount {
  status: string;
  count: number;
}

export interface Dashboard {
  generatedAt: string;
  kpis: KPIs;
  throughput: ThroughputPoint[];
  trend: TrendPoint[];
  statusBreakdown: StatusCount[];
  flows: FlowSummary[];
  recentRuns: RunSummary[];
  stuck: StepRun[];
  recentAlerts: Alert[];
}

// --- misc -----------------------------------------------------------------

export interface Health {
  status: string;
  database: string;
  timestamp: string;
  autoRegister: boolean;
  ingestAuth: boolean;
}

export interface IngestResult {
  flowKey: string;
  nodeKey: string;
  businessKey: string;
  status: string;
  accepted: boolean;
  duplicate: boolean;
  runId?: number;
  stepRunId?: number;
  runStatus?: string;
  stepStatus?: string;
  autoCreated?: boolean;
  message?: string;
}

export interface BatchResult {
  received: number;
  accepted: number;
  rejected: number;
  duplicates: number;
  results: IngestResult[];
}

export interface ManifestIngest {
  singleEndpoint: string;
  batchEndpoint: string;
  method: string;
  authHeader: string;
  statuses: string[];
  businessKey: string;
}

export interface Manifest {
  flow: {
    key: string;
    name: string;
    description: string;
    category: string;
    owner: string;
    version: string;
  };
  ingest: ManifestIngest;
  nodes: NodeContract[];
}

export interface SimStats {
  flows: number;
  runs: number;
  steps: number;
  events: number;
  stuck: number;
}
