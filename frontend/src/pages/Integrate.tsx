import { useMemo, useState } from "react";
import { Link } from "react-router-dom";
import { useMutation, useQuery } from "@tanstack/react-query";
import { Play, Plug, Send } from "lucide-react";
import { errorMessage, getManifest, listFlows, postIngest } from "@/lib/api";
import { prettyJson } from "@/lib/format";
import { Button, Field, Select, TextArea, TextInput } from "@/components/ui/Form";
import { Card, CardBody, CardHeader, ErrorState, Spinner } from "@/components/ui/primitives";
import { KindBadge } from "@/components/ui/badges";
import { CodeBlock } from "@/components/ui/CodeBlock";
import { JsonBlock } from "@/components/ui/JsonBlock";
import { MetaGrid } from "@/components/ui/Meta";
import { PageBody, PageHeader } from "@/components/ui/PageHeader";
import { Tabs } from "@/components/ui/Tabs";

const snippetTabs = [
  { id: "curl", label: "curl" },
  { id: "go", label: "Go" },
  { id: "node", label: "Node" },
  { id: "python", label: "Python" },
];

export default function Integrate() {
  const base = typeof window !== "undefined" ? window.location.origin : "http://localhost:8080";
  const endpoint = `${base}/api/ingest/events`;

  const flowsQuery = useQuery({ queryKey: ["flows"], queryFn: listFlows });
  const [flowKey, setFlowKey] = useState("");
  const [language, setLanguage] = useState("curl");
  const [apiKey, setApiKey] = useState("");
  const [payload, setPayload] = useState(() =>
    JSON.stringify(
      {
        flowKey: "order-fulfillment",
        nodeKey: "order_received",
        businessKey: "ORD-20001",
        status: "started",
        source: "order-service",
        request: { orderId: "ORD-20001", amount: 42.5 },
      },
      null,
      2,
    ),
  );

  const flowOptions = useMemo(
    () => [
      { value: "", label: "Select a flow…" },
      ...(flowsQuery.data ?? []).map((flow) => ({ value: flow.key, label: `${flow.name} (${flow.key})` })),
    ],
    [flowsQuery.data],
  );

  const manifestQuery = useQuery({
    queryKey: ["manifest", flowKey],
    queryFn: () => getManifest(flowKey),
    enabled: flowKey !== "",
  });

  const send = useMutation({
    mutationFn: () => postIngest(JSON.parse(payload), apiKey.trim() || undefined),
  });

  const snippets: Record<string, string> = {
    curl: `curl -X POST ${endpoint} \\
  -H "Content-Type: application/json" \\
${apiKey.trim() ? '  -H "X-API-Key: $FLOWMON_INGEST_KEY" \\\n' : ""}  -d '${JSON.stringify(
      {
        flowKey: "order-fulfillment",
        nodeKey: "order_received",
        businessKey: "ORD-20001",
        status: "started",
        source: "order-service",
        request: { orderId: "ORD-20001", amount: 42.5 },
      },
      null,
      0,
    )}'`,
    go: `body, _ := json.Marshal(map[string]any{
    "flowKey":     "order-fulfillment",
    "nodeKey":     "order_received",
    "businessKey": "ORD-20001",
    "status":      "started",
    "source":      "order-service",
    "request":     map[string]any{"orderId": "ORD-20001", "amount": 42.5},
})

req, _ := http.NewRequest("POST", "${endpoint}", bytes.NewReader(body))
req.Header.Set("Content-Type", "application/json")
${apiKey.trim() ? 'req.Header.Set("X-API-Key", os.Getenv("FLOWMON_INGEST_KEY"))\n' : ""}resp, _ := http.DefaultClient.Do(req)
defer resp.Body.Close()`,
    node: `await fetch("${endpoint}", {
  method: "POST",
  headers: {
    "Content-Type": "application/json",${apiKey.trim() ? '\n    "X-API-Key": process.env.FLOWMON_INGEST_KEY,' : ""}
  },
  body: JSON.stringify({
    flowKey: "order-fulfillment",
    nodeKey: "order_received",
    businessKey: "ORD-20001",
    status: "started",
    source: "order-service",
    request: { orderId: "ORD-20001", amount: 42.5 },
  }),
});`,
    python: `import os, requests

requests.post(
    "${endpoint}",
    headers={
        "Content-Type": "application/json",${apiKey.trim() ? '\n        "X-API-Key": os.environ["FLOWMON_INGEST_KEY"],' : ""}
    },
    json={
        "flowKey": "order-fulfillment",
        "nodeKey": "order_received",
        "businessKey": "ORD-20001",
        "status": "started",
        "source": "order-service",
        "request": {"orderId": "ORD-20001", "amount": 42.5},
    },
)`,
  };

  return (
    <div className="flex h-full flex-col">
      <PageHeader
        title="Integrate"
        subtitle="Add one HTTP call per process step so FlowMonitor can stitch and monitor your business flow"
      />

      <PageBody>
        <div className="grid grid-cols-1 gap-4 xl:grid-cols-3">
          <Card className="xl:col-span-2">
            <CardHeader
              title={
                <span className="flex items-center gap-2">
                  <Plug className="size-4 text-indigo-500" />
                  The ingestion contract
                </span>
              }
              subtitle="one event per step transition, called from the system that performs it"
            />
            <CardBody className="space-y-4">
              <MetaGrid
                columns={2}
                items={[
                  { label: "Single event", value: `/api/ingest/events`, mono: true },
                  { label: "Batch", value: `/api/ingest/events/batch`, mono: true },
                  { label: "Method", value: "POST" },
                  { label: "Auth header", value: "X-API-Key (only when configured)", mono: true },
                  { label: "Statuses", value: "started · completed · failed · skipped" },
                  { label: "Correlation", value: "your own businessKey (order id, claim id…)" },
                ]}
              />
              <p className="text-sm text-slate-600">
                Log a <span className="font-mono text-xs">started</span> event when a step begins and a{" "}
                <span className="font-mono text-xs">completed</span>/<span className="font-mono text-xs">failed</span>{" "}
                event when it ends. Include the request/response as JSON and FlowMonitor keeps the payload,
                timings and SLA for you. Deliveries are idempotent — replaying the same event is safe.
              </p>
              <JsonBlock
                label="Event body"
                value={{
                  flowKey: "order-fulfillment",
                  nodeKey: "validate_payment",
                  businessKey: "ORD-10231",
                  status: "started",
                  occurredAt: "2026-10-17T09:31:02Z",
                  durationMs: 0,
                  attempt: 1,
                  source: "payment-service",
                  actor: "system",
                  initiatedBy: "checkout-ui",
                  request: { orderId: "ORD-10231", amount: 129.9 },
                  response: { authorized: true },
                  error: "card declined",
                  metadata: { region: "ap-southeast-1" },
                }}
                maxHeight="26rem"
              />
            </CardBody>
          </Card>

          <Card>
            <CardHeader title="Snippets" subtitle="copy into the system at each step" />
            <CardBody className="space-y-3">
              <Field label="X-API-Key (optional)">
                <TextInput value={apiKey} onChange={setApiKey} placeholder="only if your deployment requires it" />
              </Field>
              <Tabs tabs={snippetTabs} active={language} onChange={setLanguage} />
              <CodeBlock code={snippets[language] ?? ""} language={language} />
            </CardBody>
          </Card>
        </div>

        <Card>
          <CardHeader
            title="Try it"
            subtitle="POST a real event to the running backend"
          />
          <CardBody className="grid grid-cols-1 gap-4 lg:grid-cols-2">
            <div className="space-y-3">
              <Field label="Event JSON">
                <TextArea value={payload} onChange={setPayload} mono rows={14} />
              </Field>
              <Button onClick={() => send.mutate()} disabled={send.isPending}>
                <Send className="size-3.5" />
                {send.isPending ? "Sending…" : "POST event"}
              </Button>
            </div>
            <div>
              <div className="mb-1 text-xs font-medium text-slate-600">Result</div>
              {send.isError ? (
                <div className="rounded-lg border border-rose-200 bg-rose-50 px-3 py-2 text-sm text-rose-700">
                  {errorMessage(send.error)}
                </div>
              ) : send.data ? (
                <JsonBlock value={send.data} maxHeight="20rem" />
              ) : (
                <p className="rounded-lg border border-dashed border-slate-200 px-3 py-6 text-center text-sm text-slate-400">
                  The response from the ingestion API will appear here.
                </p>
              )}
            </div>
          </CardBody>
        </Card>

        <Card>
          <CardHeader
            title="Flow manifest"
            subtitle="the exact steps, systems and payloads to instrument for a flow"
            actions={
              <div className="w-64">
                <Select value={flowKey} onChange={setFlowKey} options={flowOptions} />
              </div>
            }
          />
          {flowKey === "" ? (
            <CardBody>
              <p className="py-6 text-center text-sm text-slate-400">
                Pick a flow to see what each of its steps expects.
              </p>
            </CardBody>
          ) : manifestQuery.isLoading ? (
            <Spinner label="Loading manifest…" />
          ) : manifestQuery.isError ? (
            <ErrorState message={errorMessage(manifestQuery.error)} onRetry={() => manifestQuery.refetch()} />
          ) : manifestQuery.data ? (
            <div className="divide-y divide-slate-100">
              <div className="px-4 py-3 text-xs text-slate-500">
                <span className="font-medium text-slate-700">{manifestQuery.data.flow.name}</span> ·{" "}
                {manifestQuery.data.flow.description} · owner {manifestQuery.data.flow.owner}
              </div>
              {manifestQuery.data.nodes.map((node) => (
                <div key={node.key} className="grid grid-cols-1 gap-3 px-4 py-3 lg:grid-cols-2">
                  <div>
                    <div className="flex items-center gap-2">
                      <span className="font-medium text-slate-800">{node.name || node.key}</span>
                      <KindBadge kind={node.kind} />
                      <span className="font-mono text-xs text-slate-400">{node.key}</span>
                    </div>
                    <MetaGrid
                      className="mt-2"
                      columns={1}
                      items={[
                        { label: "System", value: node.system || "—" },
                        { label: "Endpoint", value: `${node.method} ${node.endpoint}`, mono: true },
                        { label: "Repository", value: node.repository || "—", mono: true },
                        { label: "SLA", value: `${node.timeoutMinutes} min` },
                      ]}
                    />
                  </div>
                  <div className="grid grid-cols-1 gap-2 sm:grid-cols-2">
                    <JsonBlock value={node.requestSchema} label="Parameters" emptyLabel="—" maxHeight="12rem" />
                    <JsonBlock value={node.responseSample} label="Sample response" emptyLabel="—" maxHeight="12rem" />
                  </div>
                </div>
              ))}
            </div>
          ) : null}
        </Card>

        <p className="flex items-center gap-1.5 px-1 text-xs text-slate-400">
          <Play className="size-3.5" />
          Tip: unknown flow/node keys are auto-registered as placeholders so no telemetry is lost —{" "}
          <Link to="/flows" className="text-indigo-600 hover:text-indigo-800">
            complete their contract
          </Link>{" "}
          in the editor afterwards. The raw payload is always kept: {" "}
          <span className="font-mono">{prettyJson({ region: "ap-southeast-1" })}</span>
        </p>
      </PageBody>
    </div>
  );
}
