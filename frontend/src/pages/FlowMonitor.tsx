import { useState } from "react";
import { Link, useParams } from "react-router-dom";
import { useQuery } from "@tanstack/react-query";
import { Code2, Pencil, RefreshCw, Workflow } from "lucide-react";
import { errorMessage, getFlow } from "@/lib/api";
import type { FlowNode } from "@/lib/types";
import { formatDuration, formatPercent, formatRelative } from "@/lib/format";
import { Button } from "@/components/ui/Form";
import { ErrorState, Spinner } from "@/components/ui/primitives";
import { Badge } from "@/components/ui/badges";
import { FlowCanvas } from "@/components/diagram/FlowCanvas";
import { NodeDetailPanel } from "@/components/diagram/NodeDetailPanel";

export default function FlowMonitor() {
  const { idOrKey = "" } = useParams();
  const [selected, setSelected] = useState<FlowNode | null>(null);

  const query = useQuery({
    queryKey: ["flow", idOrKey],
    queryFn: () => getFlow(idOrKey),
    refetchInterval: 5_000,
  });

  const flow = query.data;

  return (
    <div className="flex h-full flex-col">
      <header className="flex flex-wrap items-center justify-between gap-3 border-b border-slate-200 bg-white px-6 py-3">
        <div className="min-w-0">
          <div className="flex items-center gap-2">
            <Workflow className="size-5 text-indigo-600" />
            <h1 className="truncate text-lg font-semibold tracking-tight text-slate-900">
              {flow ? flow.name : "Flow"}
            </h1>
            {flow ? <Badge tone="slate">{flow.key}</Badge> : null}
            {flow ? (
              <Badge tone={flow.active ? "emerald" : "slate"}>{flow.active ? "active" : "inactive"}</Badge>
            ) : null}
          </div>
          {flow ? (
            <p className="mt-0.5 truncate text-sm text-slate-500">
              {flow.owner || "unowned"}
              {flow.category ? ` · ${flow.category}` : ""} · v{flow.version} · updated{" "}
              {formatRelative(flow.updatedAt)}
            </p>
          ) : null}
        </div>

        <div className="flex items-center gap-2">
          {flow ? (
            <div className="mr-2 hidden items-center gap-3 text-xs text-slate-500 lg:flex">
              <Metric label="Running" value={String(flow.running)} tone="info" />
              <Metric label="Running tasks" value={String(flow.runningTasks)} tone="info" />
              <Metric label="Stuck" value={String(flow.stuckTasks)} tone={flow.stuckTasks > 0 ? "warn" : undefined} />
              <Metric label="Success 24h" value={formatPercent(flow.successRate)} />
              <Metric label="Avg" value={formatDuration(flow.avgDurationMs)} />
            </div>
          ) : null}
          <Button variant="secondary" size="sm" onClick={() => query.refetch()} disabled={query.isFetching}>
            <RefreshCw className={`size-3.5 ${query.isFetching ? "animate-spin" : ""}`} />
            Refresh
          </Button>
          <Link to={`/flows/${idOrKey}/edit`}>
            <Button variant="secondary" size="sm">
              <Pencil className="size-3.5" />
              Edit
            </Button>
          </Link>
          <Link to="/integrate">
            <Button size="sm">
              <Code2 className="size-3.5" />
              Integrate
            </Button>
          </Link>
        </div>
      </header>

      <div className="relative min-h-0 flex-1 bg-slate-50">
        {query.isLoading ? <Spinner label="Loading flow…" /> : null}
        {query.isError ? (
          <ErrorState message={errorMessage(query.error)} onRetry={() => query.refetch()} />
        ) : null}
        {flow ? (
          <>
            <FlowCanvas
              nodes={flow.nodes}
              edges={flow.edges}
              onSelectNode={setSelected}
              selectedKey={selected?.key}
            />
            <div className="pointer-events-none absolute bottom-3 left-3 z-10 rounded-md border border-slate-200 bg-white/90 px-3 py-2 text-[11px] text-slate-500 shadow-sm">
              Hover a step for its contract · click for live tasks and history
            </div>
          </>
        ) : null}
      </div>

      <NodeDetailPanel
        flowIdOrKey={idOrKey}
        nodeKey={selected?.key ?? null}
        open={selected !== null}
        onClose={() => setSelected(null)}
      />
    </div>
  );
}

function Metric({ label, value, tone }: { label: string; value: string; tone?: "info" | "warn" }) {
  return (
    <div className="text-center">
      <div className="text-[10px] font-medium uppercase tracking-wide text-slate-400">{label}</div>
      <div
        className={`text-sm font-semibold tabular-nums ${
          tone === "warn" ? "text-amber-600" : tone === "info" ? "text-indigo-600" : "text-slate-700"
        }`}
      >
        {value}
      </div>
    </div>
  );
}
