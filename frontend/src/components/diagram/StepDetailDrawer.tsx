import { useQuery } from "@tanstack/react-query";
import { Link } from "react-router-dom";
import { ExternalLink } from "lucide-react";
import { getStep, errorMessage } from "@/lib/api";
import type { StepRun } from "@/lib/types";
import { formatDateTime, formatDuration } from "@/lib/format";
import { Drawer } from "@/components/ui/Drawer";
import { MetaGrid } from "@/components/ui/Meta";
import { JsonBlock } from "@/components/ui/JsonBlock";
import { KindBadge, StatusBadge, StuckBadge } from "@/components/ui/badges";
import { ErrorState, Spinner } from "@/components/ui/primitives";

/** Drawer showing one task execution: payloads, timings, attempts, siblings, node history. */
export function StepDetailDrawer({
  runId,
  stepId,
  open,
  onClose,
}: {
  runId: number | null;
  stepId: number | null;
  open: boolean;
  onClose: () => void;
}) {
  const query = useQuery({
    queryKey: ["step", runId, stepId],
    queryFn: () => getStep(runId as number, stepId as number),
    enabled: open && runId !== null && stepId !== null,
  });

  const title = query.data ? query.data.step.nodeName || query.data.step.nodeKey : "Task";

  return (
    <Drawer
      open={open}
      onClose={onClose}
      title={title}
      subtitle={query.data ? `${query.data.step.flowName} · ${query.data.step.businessKey}` : undefined}
      widthClass="max-w-3xl"
      footer={
        query.data ? (
          <Link
            to={`/runs/${query.data.run.id}`}
            className="inline-flex items-center gap-1 font-medium text-indigo-600 hover:text-indigo-800"
          >
            Open execution #{query.data.run.id}
            <ExternalLink className="size-3.5" />
          </Link>
        ) : null
      }
    >
      {query.isLoading ? <Spinner /> : null}
      {query.isError ? (
        <ErrorState message={errorMessage(query.error)} onRetry={() => query.refetch()} />
      ) : null}

      {query.data ? (
        <div className="space-y-5 p-4">
          <div className="flex flex-wrap items-center gap-2">
            <StatusBadge status={query.data.step.status} />
            {query.data.step.stuck ? <StuckBadge overdueMs={query.data.step.overdueMs} /> : null}
            <KindBadge kind={query.data.step.nodeKind} />
            {query.data.step.attempt > 1 ? (
              <span className="text-xs text-slate-500">attempt {query.data.step.attempt}</span>
            ) : null}
          </div>

          {query.data.step.error ? (
            <div className="rounded-lg border border-rose-200 bg-rose-50 px-3 py-2 text-sm text-rose-700">
              {query.data.step.error}
            </div>
          ) : null}

          <MetaGrid
            columns={3}
            items={[
              { label: "Node", value: query.data.step.nodeKey, mono: true },
              { label: "System", value: query.data.step.system || "—" },
              { label: "Source", value: query.data.step.source || "—" },
              { label: "Started", value: formatDateTime(query.data.step.startedAt) },
              { label: "Completed", value: query.data.step.completedAt ? formatDateTime(query.data.step.completedAt) : "still running" },
              {
                label: "Duration",
                value: query.data.step.completedAt
                  ? formatDuration(query.data.step.durationMs)
                  : `running ${formatDuration(query.data.step.elapsedMs)}`,
              },
              { label: "SLA", value: `${query.data.step.timeoutMinutes} min` },
              { label: "Actor", value: query.data.step.actor || "—" },
              { label: "Business key", value: query.data.step.businessKey, mono: true },
            ]}
          />

          <div className="grid grid-cols-1 gap-3 lg:grid-cols-2">
            <JsonBlock value={query.data.step.request} label="Request payload" />
            <JsonBlock value={query.data.step.response} label="Response payload" />
          </div>

          {query.data.node ? (
            <section>
              <h3 className="mb-2 text-xs font-semibold uppercase tracking-wide text-slate-500">
                Node contract
              </h3>
              <div className="rounded-lg border border-slate-200 p-3">
                <MetaGrid
                  columns={2}
                  items={[
                    { label: "System", value: query.data.node.system || "—" },
                    { label: "Team", value: query.data.node.team || "—" },
                    { label: "Endpoint", value: `${query.data.node.method} ${query.data.node.endpoint}`, mono: true },
                    { label: "Repository", value: query.data.node.repository || "—", mono: true },
                    { label: "SLA", value: `${query.data.node.timeoutMinutes} min` },
                  ]}
                />
                <div className="mt-3 grid grid-cols-1 gap-3 lg:grid-cols-2">
                  <JsonBlock value={query.data.node.requestSchema} label="Accepted params" maxHeight="14rem" />
                  <JsonBlock value={query.data.node.responseSample} label="Sample response" maxHeight="14rem" />
                </div>
              </div>
            </section>
          ) : null}

          <SiblingList
            title="Earlier in this execution"
            steps={query.data.previousSteps}
          />
          <SiblingList title="Later in this execution" steps={query.data.nextSteps} />

          {query.data.nodeHistory.length > 0 ? (
            <section>
              <h3 className="mb-2 text-xs font-semibold uppercase tracking-wide text-slate-500">
                Recent executions of this node
              </h3>
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
                    {query.data.nodeHistory.map((step) => (
                      <tr key={step.id} className="hover:bg-slate-50">
                        <td className="px-3 py-1.5 whitespace-nowrap text-slate-600">
                          {formatDateTime(step.startedAt)}
                        </td>
                        <td className="px-3 py-1.5 font-mono text-slate-600">{step.businessKey}</td>
                        <td className="px-3 py-1.5">
                          <StatusBadge status={step.status} />
                        </td>
                        <td className="px-3 py-1.5 text-right tabular-nums text-slate-600">
                          {formatDuration(step.completedAt ? step.durationMs : step.elapsedMs)}
                        </td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
            </section>
          ) : null}
        </div>
      ) : null}
    </Drawer>
  );
}

function SiblingList({ title, steps }: { title: string; steps: StepRun[] }) {
  if (steps.length === 0) return null;
  return (
    <section>
      <h3 className="mb-2 text-xs font-semibold uppercase tracking-wide text-slate-500">{title}</h3>
      <ul className="space-y-1">
        {steps.map((step) => (
          <li
            key={step.id}
            className="flex items-center justify-between rounded-md border border-slate-100 px-3 py-1.5 text-xs"
          >
            <span className="flex items-center gap-2">
              <span className="text-slate-400 tabular-nums">#{step.sequence}</span>
              <span className="font-medium text-slate-700">{step.nodeName || step.nodeKey}</span>
            </span>
            <span className="flex items-center gap-2">
              <StatusBadge status={step.status} />
              <span className="tabular-nums text-slate-500">
                {formatDuration(step.completedAt ? step.durationMs : step.elapsedMs)}
              </span>
            </span>
          </li>
        ))}
      </ul>
    </section>
  );
}
