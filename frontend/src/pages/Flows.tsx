import { useQuery } from "@tanstack/react-query";
import { Link } from "react-router-dom";
import { Plus, RefreshCw, Workflow } from "lucide-react";
import { errorMessage, listFlows } from "@/lib/api";
import { Button } from "@/components/ui/Form";
import { Card, CardHeader, ErrorState, Spinner } from "@/components/ui/primitives";
import { PageBody, PageHeader } from "@/components/ui/PageHeader";
import { FlowTable } from "@/components/FlowTable";

export default function Flows() {
  const query = useQuery({
    queryKey: ["flows"],
    queryFn: listFlows,
    refetchInterval: 5_000,
  });

  const flows = query.data ?? [];
  const totalNodes = flows.reduce((sum, flow) => sum + flow.nodeCount, 0);
  const runningTasks = flows.reduce((sum, flow) => sum + flow.runningTasks, 0);
  const stuckTasks = flows.reduce((sum, flow) => sum + flow.stuckTasks, 0);

  return (
    <div className="flex h-full flex-col">
      <PageHeader
        title="Business flows"
        subtitle="Every flow your teams instrument, with live counters"
        actions={
          <>
            <Button variant="secondary" size="sm" onClick={() => query.refetch()} disabled={query.isFetching}>
              <RefreshCw className={`size-3.5 ${query.isFetching ? "animate-spin" : ""}`} />
              Refresh
            </Button>
            <Link to="/flows/new/edit">
              <Button size="sm">
                <Plus className="size-3.5" />
                New flow
              </Button>
            </Link>
          </>
        }
      />

      <PageBody>
        {query.isLoading ? <Spinner label="Loading flows…" /> : null}
        {query.isError ? (
          <ErrorState message={errorMessage(query.error)} onRetry={() => query.refetch()} />
        ) : null}

        {query.data ? (
          <>
            <div className="grid grid-cols-2 gap-3 sm:grid-cols-4">
              <Summary label="Flows" value={flows.length} />
              <Summary label="Steps modeled" value={totalNodes} />
              <Summary label="Tasks running" value={runningTasks} tone="info" />
              <Summary label="Tasks stuck" value={stuckTasks} tone={stuckTasks > 0 ? "warn" : undefined} />
            </div>

            <Card>
              <CardHeader
                title={
                  <span className="flex items-center gap-2">
                    <Workflow className="size-4 text-indigo-500" />
                    All flows
                  </span>
                }
                subtitle="open a flow to see the live diagram, or edit its definition"
              />
              <FlowTable flows={flows} />
            </Card>
          </>
        ) : null}
      </PageBody>
    </div>
  );
}

function Summary({
  label,
  value,
  tone,
}: {
  label: string;
  value: number;
  tone?: "info" | "warn";
}) {
  return (
    <Card className="p-3">
      <div className="text-xs font-medium uppercase tracking-wide text-slate-500">{label}</div>
      <div
        className={`mt-1 text-xl font-semibold tabular-nums ${
          tone === "warn" ? "text-amber-600" : tone === "info" ? "text-indigo-600" : "text-slate-900"
        }`}
      >
        {value}
      </div>
    </Card>
  );
}
