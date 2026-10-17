# FlowMonitor Frontend

React + TypeScript monitoring UI for FlowMonitor. It draws each business flow as a live diagram
(React Flow), lets you drill into any step for its contract, its running tasks and its history, and
provides dashboard, executions, stuck-task, alert and integration views.

## Stack

| Concern | Choice | Why |
| --- | --- | --- |
| Build tool | Vite 6 | fast dev server + `/api` proxy to the Go backend |
| UI | React 19 + TypeScript | |
| Diagram | **@xyflow/react** (React Flow 12) | custom nodes, edges, pan/zoom/minimap, editable canvas |
| Routing | React Router 7 | nested layout + routes |
| Server state | TanStack Query 5 | caching + polling (monitoring needs freshness) |
| Styling | Tailwind CSS v4 | utility-first, zero config with Vite |
| HTTP | axios | one configured client in `src/lib/api.ts` |
| Charts | recharts | dashboard throughput / success-trend charts |
| Dates | dayjs | durations, relative times |
| Icons | lucide-react | |

## Structure

```
frontend/
├── index.html
├── vite.config.ts                 # @ alias, Tailwind, /api → :8080 proxy
└── src/
    ├── main.tsx                   # React root + QueryClientProvider
    ├── App.tsx                    # all routes
    ├── index.css                  # Tailwind entry
    ├── lib/
    │   ├── types.ts               # mirrors backend DTOs (camelCase JSON)
    │   ├── api.ts                 # axios client + every endpoint + errorMessage()
    │   ├── format.ts              # durations, relative time, percentages, JSON helpers
    │   └── cx.ts                  # classnames joiner
    ├── components/
    │   ├── AppLayout.tsx          # sidebar nav + backend status
    │   ├── FlowTable.tsx          # flow list with live counters
    │   ├── RunTable.tsx           # execution list
    │   ├── TaskTable.tsx          # task list
    │   ├── ui/                    # primitives: Card, StatCard, Badge, Drawer, Tabs,
    │   │                          #   JsonBlock, CodeBlock, Form, Meta, Pagination, …
    │   └── diagram/
    │       ├── ProcessNode.tsx    # live node (kind styling + running/stuck badges)
    │       ├── FlowCanvas.tsx     # read-only diagram + hover tooltip
    │       ├── NodeHoverCard.tsx  # hover summary (system, endpoint, SLA, stats)
    │       ├── NodeDetailPanel.tsx# node drill-down (contract / live tasks / history)
    │       ├── StepDetailDrawer.tsx # task drill-down (payloads, timings, attempts, history)
    │       └── EditorProcessNode.tsx# editor node
    └── pages/
        ├── Dashboard.tsx          # KPIs, charts, flows, stuck, alerts, recent runs
        ├── Flows.tsx              # flow list + "new flow"
        ├── FlowMonitor.tsx        # live diagram for one flow
        ├── FlowEditor.tsx         # create/edit a flow definition
        ├── Runs.tsx               # executions with filters + pagination
        ├── RunDetail.tsx          # one execution: step timeline, events, alerts, previous runs
        ├── Stuck.tsx              # tasks over SLA, with payloads
        ├── Alerts.tsx             # alert inbox with acknowledge
        └── Integrate.tsx          # ingestion contract, snippets, manifest, "try it"
```

## Run

```bash
cd frontend
npm install        # first time only
npm run dev        # http://localhost:5173
```

The dev server proxies `/api/*` to `http://localhost:8080` (see `vite.config.ts`), so start the
backend first.

```bash
npm run build      # tsc --noEmit && vite build
npm run preview    # serve the production build
```

`noUnusedLocals` and `noUnusedParameters` are enabled, so unused imports break the build.

## Routes

| Route | Screen |
| --- | --- |
| `/dashboard` | KPIs, throughput (24h), success trend (7d), flows, stuck tasks, recent alerts, recent executions |
| `/flows` | all flows with live counters |
| `/flows/new/edit` | create a flow definition |
| `/flows/:idOrKey` | live diagram; hover a node for its contract, click for tasks + history |
| `/flows/:idOrKey/edit` | edit a flow definition |
| `/runs` | executions, filterable by flow / status / business key / node |
| `/runs/:id` | one execution: step timeline, payloads, raw events, alerts, previous runs |
| `/stuck` | tasks over SLA, with overdue time and request payload |
| `/alerts` | open / acknowledged / resolved alerts, acknowledge inline |
| `/integrate` | ingestion contract, curl/Go/Node/Python snippets, per-flow manifest, live "try it" |

## Live data

Pages poll every 5 seconds (`refetchInterval`) from TanStack Query; node and edge styling reacts to
the polled counters — a node with running tasks shows a pulsing badge, its outgoing edges animate,
and overdue tasks show an amber "stuck" marker.

## Notes

- `src/lib/types.ts` mirrors `backend/internal/dto/dto.go`; keep them in sync when a DTO changes.
- The dashboard can replay demo traffic via `POST /api/demo/seed` ("Seed demo traffic").
- The editor validates locally (key/name, unique node keys, edge endpoints) and surfaces any
  backend validation error inline; `requestSchema` and `responseSample` are edited as JSON and
  parsed before saving.
