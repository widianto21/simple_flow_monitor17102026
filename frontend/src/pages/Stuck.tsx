import { useState } from "react";
import { Link } from "react-router-dom";
import { useQuery } from "@tanstack/react-query";
import { ExternalLink, RefreshCw, Timer, TriangleAlert } from "lucide-react";
import { errorMessage, listStuck } from "@/lib/api";
import { formatDuration, formatRelative } from "@/lib/format";
import { Button } from "@/components/ui/Form";
import { Card, ErrorState, Spinner, StatCard } from "@/components/ui/primitives";
import { JsonBlock } from "@/components/ui/JsonBlock";
import { StatusBadge } from "@/components/ui/badges";
import { PageBody, PageHeader } from "@/components/ui/PageHeader";
import { StepDetailDrawer } from "@/components/diagram/StepDetailDrawer";

export default function Stuck() {
  const [selected, setSelected] = useState<{ runId: number; stepId: number } | null>(null);

  const query = useQuery({
    queryKey: ["stuck"],
    queryFn: () => listStuck(100),
    refetchInterval: 5_000,
  });

  const tasks = query.data?.items ?? [];
  const worst = tasks.reduce((max, task) => Math.max(max, task.overdueMs), 0);
  const flows = new Set(tasks.map((task) => task.flowKey)).size;

  return (
    <div className="flex h-full flex-col">
      <PageHeader
        title="Stuck tasks"
        subtitle="Process steps running past their SLA — investigate, then clear the obstruction"
        actions={
          <Button variant="secondary" size="sm" onClick={() => query.refetch()} disabled={query.isFetching}>
            <RefreshCw className={`size-3.5 ${query.isFetching ? "animate-spin" : ""}`} />
            Refresh
          </Button>
        }
      />

      <PageBody>
        {query.isLoading ? <Spinner label="Loading stuck tasks…" /> : null}
        {query.isError ? (
          <ErrorState message={errorMessage(query.error)} onRetry={() => query.refetch()} />
        ) : null}

        {query.data ? (
          <>
            <div className="grid grid-cols-1 gap-3 sm:grid-cols-3">
              <StatCard
                label="Stuck now"
                value={tasks.length}
                tone={tasks.length > 0 ? "warn" : "good"}
                icon={<Timer className="size-4" />}
              />
              <StatCard label="Worst overdue" value={tasks.length ? formatDuration(worst) : "—"} tone="bad" />
              <StatCard label="Flows affected" value={flows} />
            </div>

            {tasks.length === 0 ? (
              <Card className="p-10 text-center">
                <p className="text-sm font-medium text-emerald-700">Nothing is over SLA right now 🎉</p>
                <p className="mt-1 text-xs text-slate-500">
                  Tasks appear here when they run longer than their node's timeout.
                </p>
              </Card>
            ) : (
              <div className="grid grid-cols-1 gap-4 xl:grid-cols-2">
                {tasks.map((task) => (
                  <Card key={task.id} className="overflow-hidden">
                    <div className="flex items-start justify-between gap-3 border-b border-slate-100 px-4 py-3">
                      <div className="min-w-0">
                        <div className="flex items-center gap-2">
                          <TriangleAlert className="size-4 text-amber-500" />
                          <span className="truncate font-semibold text-slate-800">
                            {task.nodeName || task.nodeKey}
                          </span>
                          <StatusBadge status={task.status} />
                        </div>
                        <div className="mt-0.5 truncate text-xs text-slate-500">
                          {task.flowName || task.flowKey} ·{" "}
                          <span className="font-mono">{task.businessKey}</span>
                          {task.system ? ` · ${task.system}` : ""}
                        </div>
                      </div>
                      <div className="shrink-0 text-right">
                        <div className="text-sm font-semibold text-amber-600">
                          +{formatDuration(task.overdueMs)}
                        </div>
                        <div className="text-[11px] text-slate-400">
                          SLA {task.timeoutMinutes}m · {formatDuration(task.elapsedMs)} elapsed
                        </div>
                      </div>
                    </div>

                    <div className="space-y-2 px-4 py-3">
                      <div className="flex flex-wrap items-center gap-3 text-xs text-slate-500">
                        <span>started {formatRelative(task.startedAt)}</span>
                        <Link
                          to={`/runs/${task.runId}`}
                          className="inline-flex items-center gap-1 font-medium text-indigo-600 hover:text-indigo-800"
                        >
                          Open execution #{task.runId}
                          <ExternalLink className="size-3" />
                        </Link>
                        <button
                          type="button"
                          onClick={() => setSelected({ runId: task.runId, stepId: task.id })}
                          className="font-medium text-slate-600 hover:text-slate-900"
                        >
                          View payload
                        </button>
                      </div>
                      <JsonBlock value={task.request} label="Request payload" emptyLabel="no request payload" maxHeight="12rem" />
                    </div>
                  </Card>
                ))}
              </div>
            )}
          </>
        ) : null}
      </PageBody>

      <StepDetailDrawer
        runId={selected?.runId ?? null}
        stepId={selected?.stepId ?? null}
        open={selected !== null}
        onClose={() => setSelected(null)}
      />
    </div>
  );
}
