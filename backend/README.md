# FlowMonitor Backend

Go + Gin + GORM + SQLite service for business-flow monitoring. It stores flow **definitions**
(the diagram and each node's contract) separately from **telemetry** (executions, tasks, raw
events) plus **alerts**, and exposes a small HTTP API that other systems call to log their flow.

## Stack

| Layer | Choice |
| --- | --- |
| Language | Go |
| HTTP | Gin |
| ORM | GORM |
| Database | SQLite via `glebarez/sqlite` (pure Go — no CGO/gcc needed) |

## Structure

```
backend/
├── cmd/server/main.go            # config → db → migrate → seed → watcher → routes → listen
└── internal/
    ├── config/config.go          # env-based configuration
    ├── database/database.go      # connection + AutoMigrate
    ├── models/models.go          # GORM models and enums
    ├── dto/dto.go                # request/response transport shapes (camelCase JSON)
    ├── service/
    │   ├── service.go            # Services container
    │   ├── ingest.go             # ingestion state machine, dedupe, auto-registration
    │   ├── stuck.go              # open tasks, SLA detection, background watcher
    │   ├── query.go              # all read endpoints (aggregates, runs, dashboard…)
    │   └── simulate.go           # demo traffic generator
    ├── seed/seed.go              # demo flows with node contracts + simulated history
    ├── handlers/                 # Gin handlers (thin wrappers over services)
    └── routes/routes.go          # router, CORS, route registration
```

## Run

```bash
cd backend
go mod tidy            # first time only
go run ./cmd/server
```

Starts on `http://localhost:8080`. `flowmon.db` is created automatically, and on an empty
database the demo seed loads three flows (`order-fulfillment`, `customer-onboarding`,
`claim-processing`) with full node contracts plus ~72h of simulated traffic replayed through the
real ingestion path.

```bash
go test ./...          # 34 tests (service + HTTP integration)
go vet ./...
go build ./...
```

## Concepts

- **Flow** — a business process: a set of **nodes** (process steps) joined by **edges**. Each node
  carries a contract: `system`, `repository`, `team`, `method` + `endpoint`, `requestSchema`
  (accepted parameters), `responseSample`, and `timeoutMinutes` (its SLA).
- **Execution** (`run`) — one pass of a flow for a given `businessKey` (an order id, a claim id…).
- **Task** (`step_run`) — one node execution inside an execution: start/end, duration, attempt,
  request/response payload, source system, error.
- **Event** — the raw ingested log line, kept verbatim for auditing and replay.

Definitions are edited through the API/UI; telemetry is written only by ingestion.

## Ingestion API

Called by other systems, at each step. Optional `X-API-Key` when `FLOWMON_INGEST_KEY` is set.

```http
POST /api/ingest/events
Content-Type: application/json

{
  "flowKey": "order-fulfillment",
  "nodeKey": "validate_payment",
  "businessKey": "ORD-10231",
  "status": "started",
  "occurredAt": "2026-10-17T09:31:02Z",
  "durationMs": 0,
  "attempt": 1,
  "source": "payment-service",
  "actor": "system",
  "initiatedBy": "checkout-ui",
  "request":  { "orderId": "ORD-10231", "amount": 129.9 },
  "response": { "authorized": true },
  "error": "",
  "metadata": { "region": "ap-southeast-1" }
}
```

| Field | Notes |
| --- | --- |
| `status` | `started` \| `completed` \| `failed` \| `skipped`. Aliases accepted: `start`, `done`/`success`/`finish`, `error`. |
| `businessKey` | your own correlation id; ties every step of one execution together |
| `occurredAt` | when it happened (defaults to now); part of the idempotency key |
| `attempt` | retry counter (default 1); part of the idempotency key |
| `request` / `response` | any JSON, stored verbatim and shown in the UI |
| `durationMs` | optional; used for a single-shot `completed` event |

**State machine.** A `started` event opens a task (or reuses one already open for the node). A
`completed`/`failed`/`skipped` event closes the open task; if none is open, a closed task is
synthesised (single-shot instrumentation, honouring `durationMs`). When a task fails, the
execution fails and a `failure` alert is raised.

**Idempotency.** `dedupe_hash = sha256(flowKey|businessKey|nodeKey|status|occurredAt|attempt)`.
Replaying an identical event returns `duplicate: true` and changes nothing.

**Auto-registration.** With `FLOWMON_AUTO_REGISTER=true` (default), a `flowKey`/`nodeKey` that is
not yet known is created as a placeholder (with a sensible position) so no telemetry is lost;
complete its contract later in the editor.

**Responses.** `202` accepted · `400` validation · `401` bad/missing key · `409` terminal event
with no active execution for that business key · `422` unknown flow while auto-register is off.

Batch: `POST /api/ingest/events/batch` with a JSON array; returns
`{received, accepted, rejected, duplicates, results}`.

## API reference

| Method | Path | Purpose |
| --- | --- | --- |
| `GET` | `/api/health` | liveness, DB ping, `ingestAuth` / `autoRegister` flags |
| `GET` | `/api/dashboard` | KPIs, throughput, 7-day trend, status breakdown, flows, recent runs, stuck tasks, alerts |
| `POST` | `/api/ingest/events` | log one task event |
| `POST` | `/api/ingest/events/batch` | log many task events |
| `GET` | `/api/ingest/manifests/:flowKey` | what to instrument for a flow (nodes, systems, endpoints, params, sample responses) |
| `GET` | `/api/flows` | list flows with live counters |
| `POST` | `/api/flows` | create a flow definition |
| `GET` | `/api/flows/:idOrKey` | full definition + per-node live stats (accepts numeric id or key) |
| `PUT` | `/api/flows/:idOrKey` | replace a flow definition |
| `DELETE` | `/api/flows/:idOrKey` | delete a flow and its telemetry |
| `GET` | `/api/flows/:idOrKey/stats` | per-node stats only |
| `GET` | `/api/flows/:idOrKey/nodes/:nodeKey` | node drill-down: `{node, tasks, recentRuns}` |
| `GET` | `/api/runs` | list executions (`flowKey`, `status`, `businessKey`, `nodeKey`, `from`, `to`, `page`, `size`; `status` also accepts `stuck`) |
| `GET` | `/api/runs/:id` | execution detail with `steps`, `events`, `alerts`, `previousRuns` |
| `GET` | `/api/runs/:id/steps/:stepId` | task detail: `{step, run, previousSteps, nextSteps, nodeHistory, node}` |
| `GET` | `/api/steps` | list tasks (`flowKey`, `nodeKey`, `status`, `businessKey`, `stuck`, …) |
| `GET` | `/api/tasks/stuck` | tasks running beyond their SLA |
| `GET` | `/api/business-keys/:key/runs` | every execution recorded for a business key |
| `GET` | `/api/alerts` | alerts by `status=open\|acked\|resolved\|all` |
| `POST` | `/api/alerts/:id/ack` | acknowledge an alert |
| `POST` | `/api/demo/seed` | replay a batch of simulated traffic |

`GET /api/flows/:idOrKey` accepts either the numeric id or the flow key, so instrumenting systems
can use the human-readable key.

## Stuck detection

A task is **stuck** when it is still `running` and `now - started_at` exceeds the node's SLA
(`timeoutMinutes`, falling back to `FLOWMON_DEFAULT_SLA_MINUTES`). Stuckness is computed on read so
the UI is always accurate, and a background watcher (every `FLOWMON_STUCK_INTERVAL_SECONDS`)
materialises `stuck` alerts, resolving them when the task finishes. `0` minutes means "no SLA".

## Configuration

| Env var | Default | Purpose |
| --- | --- | --- |
| `FLOWMON_PORT` | `8080` | HTTP port |
| `FLOWMON_DB` | `flowmon.db` | SQLite file path |
| `FLOWMON_CORS_ALLOW` | `http://localhost:5173` | allowed browser origin |
| `FLOWMON_INGEST_KEY` | *(empty)* | when set, ingestion requires `X-API-Key` |
| `FLOWMON_AUTO_REGISTER` | `true` | auto-create unknown flows/nodes on ingest |
| `FLOWMON_DEFAULT_SLA_MINUTES` | `60` | fallback SLA (`0` disables stuck detection) |
| `FLOWMON_STUCK_INTERVAL_SECONDS` | `30` | background watcher interval |
| `FLOWMON_SEED_DEMO` | `true` | load demo flows + simulated traffic on an empty DB |

## Design notes

- Definitions and telemetry are deliberately separate tables so editing a diagram never rewrites
  history, and re-pointing an edge never loses data.
- Per-execution events are committed in one transaction (`Ingest.ProcessTx`), which preserves
  step ordering even when the demo simulator replays a whole run at once.
- The API speaks camelCase JSON end to end; the frontend mirrors these shapes in
  `frontend/src/lib/types.ts`.
