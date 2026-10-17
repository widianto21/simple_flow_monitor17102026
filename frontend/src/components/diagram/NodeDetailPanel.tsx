import { useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { getNodeDetail, errorMessage } from "@/lib/api";
import type { StepRun } from "@/lib/types";
import { formatDateTime, formatDuration, formatPercent, formatRelative } from "@/lib/format";
import { Drawer } from "@/components/ui/Drawer";
import { Tabs } from "@/components/ui/Tabs";
import { MetaGrid } from "@/components/ui/Meta";
import { JsonBlock } from "@/components/ui/JsonBlock";
import { KindBadge, StatusBadge, StuckBadge } from "@/components/ui/badges";
import { ErrorState, Spinner } from "@/components/ui/primitives";
import { StepDetailDrawer } from "./StepDetailDrawer";

/** Side drawer for one diagram component: contract, live tasks and history. */
export function NodeDetailPanel({
  flowIdOrKey,
  nodeKey,
  open,
  onClose,
}: {
  flowIdOrKey: string | number;
  nodeKey: string | null;
  open: boolean;
  onClose: () => void;
}) {
  const [tab, setTab] = useState("overview");
  const [selected, setSelected] = useState<{ runId: number; stepId: number } | null>(null);

  const query = useQuery({
    queryKey: ["node", String(flowIdOrKey), nodeKey],
    queryFn: () => getNodeDetail(flowIdOrKey, nodeKey as string),
    enabled: open && nodeKey !== null,
  });

  const node = query.data?.node;
  const tasks = query.data?.tasks.items ?? [];
  const liveTasks = tasks.filter((task) => task.status === "running");

  return (
    <>
      <Drawer
        open={open}
        onClose={onClose}
        title={node ? node.name || node.key : "Process"}
        subtitle={
          node ? `${node.system || "unassigned"} · ${node.method} ${node.endpoint}`.trim() : undefined
        }
        widthClass="max-w-2xl"
      >
        {query.isLoading ? <Spinner /> : null}
        {query.isError ? (
          <ErrorState message={errorMessage(query.error)} onRetry={() => query.refetch()} />
        ) : null}

        {node ? (
          <div>
            <div className="flex flex-wrap items-center gap-2 px-4 pt-3">
              <KindBadge kind={node.kind} />
              <StatusBadge status={node.stats.running > 0 ? "running" : "completed"} />
              {node.stats.stuck > 0 ? (
                <StuckBadge
                  overdueMs={Math.max(0, node.stats.maxElapsedMs - node.timeoutMinutes * 60_000)}
                />
              ) : null}
              <span className="ml-auto text-xs text-slate-400">
                last activity {formatRelative(node.stats.lastActivityAt)}
              </span>
            </div>

            <Tabs
              className="mt-3 px-4"
              active={tab}
              onChange={setTab}
              tabs={[
                { id: "overview", label: "Overview" },
                { id: "live", label: "Live tasks", badge: liveTasks.length || undefined },
                { id: "history", label: "History", badge: query.data?.tasks.total || undefined },
              ]}
            />

            <div className="p-4">
              {tab === "overview" ? (
                <div className="space-y-4">
                  <div className="grid grid-cols-2 gap-3 sm:grid-cols-4">
                    <Stat label="Running" value={String(node.stats.running)} />
                    <Stat label="Stuck" value={String(node.stats.stuck)} tone={node.stats.stuck > 0 ? "warn" : undefined} />
                    <Stat label="Done 24h" value={String(node.stats.total24h)} />
                    <Stat label="Failed 24h" value={String(node.stats.failed24h)} />
                    <Stat label="Avg duration" value={formatDuration(node.stats.avgDurationMs)} />
                    <Stat label="p95 duration" value={formatDuration(node.stats.p95DurationMs)} />
                    <Stat label="Failure rate" value={formatPercent(node.stats.failureRate, 1)} />
                    <Stat label="SLA" value={`${node.timeoutMinutes} min`} />
                  </div>

                  <section className="rounded-lg border border-slate-200 p-3">
                    <h3 className="mb-2 text-xs font-semibold uppercase tracking-wide text-slate-500">
                      Where it lives and what it does
                    </h3>
                    {node.description ? (
                      <p className="mb-3 text-sm text-slate-600">{node.description}</p>
                    ) : null}
                    <MetaGrid
                      columns={2}
                      items={[
                        { label: "System", value: node.system || "—" },
                        { label: "Team", value: node.team || "—" },
                        { label: "Endpoint", value: `${node.method} ${node.endpoint}`, mono: true },
                        { label: "Repository", value: node.repository || "—", mono: true },
                        { label: "Node key", value: node.key, mono: true },
                        { label: "Kind", value: node.kind },
                      ]}
                    />
                  </section>

                  <div className="grid grid-cols-1 gap-3 lg:grid-cols-2">
                    <JsonBlock value={node.requestSchema} label="Accepted parameters" maxHeight="16rem" />
                    <JsonBlock value={node.responseSample} label="Sample response" maxHeight="16rem" />
                  </div>
                </div>
              ) : null}

              {tab === "live" ? (
                <TaskList
                  empty="No tasks are running on this process right now."
                  tasks={liveTasks}
                  onSelect={(runId, stepId) => setSelected({ runId, stepId })}
                />
              ) : null}

              {tab === "history" ? (
                <TaskList
                  empty="No task history recorded yet."
                  tasks={tasks}
                  onSelect={(runId, stepId) => setSelected({ runId, stepId })}
                />
              ) : null}
            </div>
          </div>
        ) : null}
      </Drawer>

      <StepDetailDrawer
        runId={selected?.runId ?? null}
        stepId={selected?.stepId ?? null}
        open={selected !== null}
        onClose={() => setSelected(null)}
      />
    </>
  );
}

function Stat({ label, value, tone }: { label: string; value: string; tone?: "warn" }) {
  return (
    <div className="rounded-lg border border-slate-100 bg-slate-50 px-3 py-2">
      <div className="text-[10px] font-medium uppercase tracking-wide text-slate-400">{label}</div>
      <div
        className={`mt-0.5 text-sm font-semibold tabular-nums ${
          tone === "warn" ? "text-amber-600" : "text-slate-800"
        }`}
      >
        {value}
      </div>
    </div>
  );
}

function TaskList({
  tasks,
  empty,
  onSelect,
}: {
  tasks: StepRun[];
  empty: string;
  onSelect: (runId: number, stepId: number) => void;
}) {
  if (tasks.length === 0) {
    return <p className="py-6 text-center text-sm text-slate-400">{empty}</p>;
  }
  return (
    <div className="overflow-hidden rounded-lg border border-slate-200">
      <table className="w-full text-left text-xs">
        <thead className="bg-slate-50 text-slate-500">
          <tr>
            <th className="px-3 py-1.5 font-medium">Started</th>
            <th className="px-3 py-1.5 font-medium">Business key</th>
            <th className="px-3 py-1.5 font-medium">Status</th>
            <th className="px-3 py-1.5 text-right font-medium">Duration</th>
          </tr>
        </thead>
        <tbody className="divide-y divide-slate-100">
          {tasks.map((task) => (
            <tr
              key={task.id}
              onClick={() => onSelect(task.runId, task.id)}
              className="cursor-pointer hover:bg-slate-50"
            >
              <td className="px-3 py-1.5 whitespace-nowrap text-slate-600">
                {formatDateTime(task.startedAt)}
              </td>
              <td className="px-3 py-1.5 font-mono text-slate-600">{task.businessKey}</td>
              <td className="px-3 py-1.5">
                <span className="flex items-center gap-1.5">
                  <StatusBadge status={task.status} />
                  {task.stuck ? <StuckBadge compact /> : null}
                </span>
              </td>
              <td className="px-3 py-1.5 text-right tabular-nums text-slate-600">
                {formatDuration(task.completedAt ? task.durationMs : task.elapsedMs)}
              </td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}
