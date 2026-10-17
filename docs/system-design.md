# FlowMonitor — system design

A **business flow monitoring** system. Teams define their business flow as a diagram, then
every participating system logs HTTP events at each process step. FlowMonitor stitches those
events into executions, draws the live state on the diagram, and keeps full history.

## 1. Requirements → design mapping

| Requirement | How it is satisfied |
| --- | --- |
| Create business flow diagrams | `flows` + `flow_nodes` + `flow_edges`, edited in the UI editor (`PUT /api/flows/:id`) |
| Show the diagram with all currently running tasks | Node live stats (`running`, `stuck`, `failed24h`, avg/p95 duration) rendered on the diagram nodes; polled every 5s |
| Track tasks from each business flow | `step_runs` (one row per node execution) linked to `runs` (one row per flow execution per business key) |
| View historical state of each task | `step_runs` keeps attempt, start/end, duration, request/response payload, error, source system; `events` keeps the raw ingested log |
| HTTP API callable from other systems | `POST /api/ingest/events` (+ `/batch`), optional `X-API-Key`, idempotent, auto-registers unknown flows/nodes |
| Data-flow-diagram look with richer info on hover/click | Node carries `system`, `repository`, `team`, `method`, `endpoint`, `requestSchema`, `responseSample`, `timeoutMinutes` |
| Process location (system / repository) | Node fields `system`, `repository`, `team` |
| Parameters a process accepts | Node fields `requestSchema` (JSON, example/schema) |
| Sample response of a process | Node field `responseSample` (JSON) |
| View tasks stuck on a process | Derived SLA check (`elapsed > node.timeoutMinutes`), `GET /api/tasks/stuck`, background watcher raises/resolves `stuck` alerts |
| View the payload | `step_runs.requestPayload` / `responsePayload`, `events.payload`, exposed in run/step/stuck endpoints |
| See historical runs before it | `GET /api/business-keys/:key/runs` (chain per business key) and per-node history in `GET /api/flows/:key/nodes/:nodeKey` and `GET /api/runs/:id/steps/:stepId` |

## 2. Architecture

```
 third-party systems                        FlowMonitor
┌───────────────────┐                ┌────────────────────────────────────────┐
│ order-service     │  HTTP POST     │  Gin API                               │
│ payment-service   │──/api/ingest──▶│   ├─ /ingest/*  (instrumentation)      │
│ inventory-service │   /events      │   ├─ /flows/*   (definitions + stats)  │
└───────────────────┘                │   ├─ /runs/*    (executions + steps)   │
                                     │   └─ /tasks/stuck, /alerts, /dashboard │
        React SPA (Vite)             │                                        │
┌───────────────────┐   JSON         │  Services: Ingest · Query · Simulator  │
│ React Flow diagram│◀──polls 5s────▶│            · StuckWatcher (ticker)     │
│ drill-down panels │                │  GORM → SQLite (WAL, single writer)    │
└───────────────────┘                └────────────────────────────────────────┘
```

Layering: `handlers` (HTTP) → `service` (domain logic) → `models`/`dto` (data + transport shapes) → `database`.

Stuck detection runs in-process every `FLOWMON_STUCK_INTERVAL_SECONDS` (default 30s) so it does
not depend on anyone polling the UI.

## 3. Data model

```
flows ─1:N─ flow_nodes            (definition: diagram + contract per process step)
      └─1:N─ flow_edges

runs ─1:N─ step_runs              (telemetry: one execution of a flow / of a node)
     └─1:N─ events                (raw ingested log, idempotent via dedupe_hash)
alerts                            (stuck / failure, auto-resolved when condition clears)
```

### flows
`key` (unique, used by instrumentation), `name`, `description`, `category`, `owner`, `version`, `active`.

### flow_nodes — the "rich DFD" component
`key` (unique per flow), `name`, `kind` (`start|task|decision|gateway|external|end`),
`description`, `system`, `repository`, `team`, `method`, `endpoint`,
`request_schema` (JSON), `response_sample` (JSON), `timeout_minutes` (SLA → stuck threshold),
`pos_x`, `pos_y` (diagram coordinates).

### flow_edges
`source_key`, `target_key`, `label`, `condition` (labels drive decision branches, e.g. `yes`/`no`).

### runs — one flow execution
`flow_id`, `business_key` (correlation id: order id, claim id, …), `status`
(`running|completed|failed|cancelled`), `current_node_key`, `started_at`, `completed_at`,
`duration_ms`, `initiated_by`, `metadata`, `failure_error`.

### step_runs — one task execution (the core tracking record)
`run_id`, `flow_id`, `node_key`, `node_name`, `node_kind`, `status`
(`running|completed|failed|skipped`), `sequence`, `attempt`, `started_at`, `completed_at`,
`duration_ms`, `request_payload`, `response_payload`, `error`, `source`, `actor`, `metadata`.

### events — raw ingestion log
`flow_key`, `node_key`, `business_key`, `status`, `occurred_at`, `received_at`, `source`,
`run_id`, `step_run_id`, `payload`, `error`, `dedupe_hash` (unique).

### alerts
`flow_id`, `flow_key`, `run_id`, `node_key`, `type` (`stuck|failure`), `severity`, `message`,
`acknowledged`, `acknowledged_at`, `resolved`, `resolved_at`, `dedupe_key` (unique).

## 4. Ingestion contract

`POST /api/ingest/events` (single) / `POST /api/ingest/events/batch` (array).
Header `X-API-Key` is required only when `FLOWMON_INGEST_KEY` is configured.

```json
{
  "flowKey": "order-fulfillment",
  "nodeKey": "validate_payment",
  "businessKey": "ORD-10231",
  "status": "started | completed | failed | skipped",
  "occurredAt": "2026-10-17T09:31:02Z",
  "durationMs": 0,
  "attempt": 1,
  "source": "payment-service",
  "actor": "system",
  "initiatedBy": "checkout-ui",
  "request":  { "orderId": "ORD-10231", "amount": 129.9 },
  "response": { "authorized": true },
  "error": "card declined",
  "metadata": { "region": "ap-southeast-1" }
}
```

Status aliases accepted for convenience: `start` → `started`, `done|success|finish` → `completed`, `error` → `failed`.

### State machine

| Event | Effect |
| --- | --- |
| `started` on any node | Find open run for `businessKey`; create it if absent (first `started` of the flow). Open a `step_run` (`attempt +1` if one is already open for that node). Run `current_node_key` = node. |
| `completed` | Close the open `step_run`, store response, compute duration. If the node is an `end` node → run becomes `completed`. |
| `failed` | Close the task as `failed`, run becomes `failed` + `failure_error`, raise a `failure` alert. |
| `skipped` | Close the task as `skipped` without failing the run. |
| `completed` with no open task | Synthesized closed task (single-shot instrumentation, e.g. fire-and-forget logging). `durationMs` is respected. |

Idempotency: duplicate deliveries with the same `(flowKey, businessKey, nodeKey, status, occurredAt, attempt)`
are detected via `dedupe_hash` and return `duplicate: true` without mutating state.

Instrumentation-friendly extras:
- Unknown `flowKey`/`nodeKey` are auto-registered (config `FLOWMON_AUTO_REGISTER=true`) as placeholder
  nodes so no telemetry is ever lost; the team then completes the node contract in the editor.
- `GET /api/ingest/manifests/:flowKey` returns the full node contract (system, endpoint, params,
  sample response) so developers know exactly what to log where.

## 5. Stuck detection

A task is **stuck** when it is still `running` and `now - started_at > node.timeout_minutes`
(fallback `FLOWMON_DEFAULT_SLA_MINUTES`, default 60). It is computed on read (always accurate,
no drift) and additionally materialized as an `alert`
(`severity: warning` at 1×, `critical` at 2× the SLA) by the background watcher. When the task
finally completes, the alert is auto-resolved.

## 6. API surface

| Method | Path | Purpose |
| --- | --- | --- |
| GET | `/api/health` | liveness + DB ping |
| POST | `/api/ingest/events` | log one task event (external systems) |
| POST | `/api/ingest/events/batch` | log many events |
| GET | `/api/ingest/manifests/:flowKey` | contract of every step, for instrumenting teams |
| GET | `/api/flows` | flow list + live counters |
| POST | `/api/flows` | create flow |
| GET | `/api/flows/:idOrKey` | flow definition + nodes (with stats) + edges |
| PUT | `/api/flows/:idOrKey` | replace definition (editor save) |
| DELETE | `/api/flows/:idOrKey` | delete flow + its telemetry |
| GET | `/api/flows/:idOrKey/stats` | per-node live/historic stats |
| GET | `/api/flows/:idOrKey/nodes/:nodeKey` | node contract + stats + task history |
| GET | `/api/runs` | executions with filters (`flowKey,status,businessKey,nodeKey,from,to,page,size`) |
| GET | `/api/runs/:id` | execution + all tasks + events + previous executions of the business key |
| GET | `/api/runs/:id/steps/:stepId` | one task: payloads + sibling tasks + history of that node |
| GET | `/api/steps` | task search across flows (`flowKey,nodeKey,status,businessKey,stuck,from,to`) |
| GET | `/api/tasks/stuck` | tasks over SLA, with payload + overflow |
| GET | `/api/business-keys/:key/runs` | full history chain for one business key |
| GET | `/api/dashboard` | KPIs, 24h throughput, 7d trend, per-flow summary, recent runs, stuck, alerts |
| GET | `/api/alerts` | alerts (`?status=open\|acked\|resolved\|all`) |
| POST | `/api/alerts/:id/ack` | acknowledge alert |
| POST | `/api/demo/seed` | generate extra demo traffic |

## 7. Frontend structure

| Route | Page | Purpose |
| --- | --- | --- |
| `/dashboard` | Dashboard | KPIs, throughput/trend charts, flow table, stuck preview, alerts |
| `/flows` | Flows | list, create, open monitor/editor |
| `/flows/new/edit` | Flow editor | create a new flow definition |
| `/flows/:idOrKey` | Flow monitor | live React Flow diagram, node hover tooltip, click → drawer with Overview / Live tasks / History |
| `/flows/:idOrKey/edit` | Flow editor | add/connect/edit/delete nodes, edit contract fields, save definition |
| `/runs` | Runs | filter/search executions, status incl. `stuck` |
| `/runs/:id` | Run detail | step timeline (Gantt), payload inspection, alerts, previous runs of the business key |
| `/stuck` | Stuck tasks | over-SLA tasks with payload, links to run, alert ack |
| `/alerts` | Alerts | all alerts, acknowledge |
| `/integrate` | Integrate | how to instrument: endpoint, auth, event schema, curl/Go/Node/Python snippets, per-flow manifest table |

Diagram interaction:
- **hover** a node → compact tooltip (system, endpoint, running/stuck, avg + p95 duration, failure rate, SLA).
- **click** a node → side drawer: contract (system/repo/team/endpoint/params/sample response), live tasks on that node, and recent history.
- **click** a running task → its payload, timings, attempts, and the run it belongs to.
