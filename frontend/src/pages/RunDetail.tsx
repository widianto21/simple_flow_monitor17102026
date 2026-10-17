import { useState } from "react";
import { Link, useParams } from "react-router-dom";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { ArrowLeft, BellRing, Check, ExternalLink, History } from "lucide-react";
import { ackAlert, errorMessage, getRun } from "@/lib/api";
import type { StepRun } from "@/lib/types";
import { formatDateTime, formatDuration, formatRelative, dayjs } from "@/lib/format";
import { Button } from "@/components/ui/Form";
import { Card, CardBody, CardHeader, ErrorState, Spinner } from "@/components/ui/primitives";
import { MetaGrid } from "@/components/ui/Meta";
import { JsonBlock } from "@/components/ui/JsonBlock";
import { AlertTypeBadge, SeverityBadge, StatusBadge, StuckBadge } from "@/components/ui/badges";
import { PageHeader } from "@/components/ui/PageHeader";
import { StepDetailDrawer } from "@/components/diagram/StepDetailDrawer";
import { RunTable } from "@/components/RunTable";

const stepColor: Record<string, string> = {
  running: "bg-indigo-500",
  completed: "bg-emerald-500",
  failed: "bg-rose-500",
  skipped: "bg-slate-300",
};

export default function RunDetail() {
  const { id = "" } = useParams();
  const queryClient = useQueryClient();
  const [selected, setSelected] = useState<StepRun | null>(null);

  const query = useQuery({
    queryKey: ["run", id],
    queryFn: () => getRun(id),
    refetchInterval: 5_000,
  });

  const ack = useMutation({
    mutationFn: (alertId: number) => ackAlert(alertId),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ["run", id] }),
  });

  const run = query.data;

  return (
    <div className="flex h-full flex-col">
      <PageHeader
        title={
          <span className="flex items-center gap-2">
            <Link to="/runs" className="text-slate-400 hover:text-slate-700">
              <ArrowLeft className="size-5" />
            </Link>
            Execution <span className="font-mono">{run?.businessKey ?? `#${id}`}</span>
          </span>
        }
        subtitle={run ? `${run.flowName || run.flowKey} · started ${formatRelative(run.startedAt)}` : undefined}
        actions={
          run ? (
            <div className="flex items-center gap-2">
              <StatusBadge status={run.status} />
              {run.stuck ? <StuckBadge overdueMs={run.elapsedMs} /> : null}
            </div>
          ) : null
        }
      />

      <div className="space-y-4 p-6">
        {query.isLoading ? <Spinner label="Loading execution…" /> : null}
        {query.isError ? (
          <ErrorState message={errorMessage(query.error)} onRetry={() => query.refetch()} />
        ) : null}

        {run ? (
          <>
            {run.failureError ? (
              <div className="rounded-lg border border-rose-200 bg-rose-50 px-4 py-2 text-sm text-rose-700">
                <span className="font-medium">Failed:</span> {run.failureError}
              </div>
            ) : null}

            <Card>
              <CardBody>
                <MetaGrid
                  columns={3}
                  items={[
                    { label: "Flow", value: <Link className="text-indigo-600 hover:text-indigo-800" to={`/flows/${run.flowKey}`}>{run.flowName || run.flowKey}</Link> },
                    { label: "Status", value: <StatusBadge status={run.status} /> },
                    { label: "Current step", value: run.currentNodeName || run.currentNodeKey || "—" },
                    { label: "Started", value: formatDateTime(run.startedAt) },
                    {
                      label: "Completed",
                      value: run.completedAt ? formatDateTime(run.completedAt) : "still running",
                    },
                    {
                      label: "Duration",
                      value: run.completedAt ? formatDuration(run.durationMs) : `${formatDuration(run.elapsedMs)} (running)`,
                    },
                    { label: "Initiated by", value: run.initiatedBy || "—" },
                    { label: "Tasks", value: `${run.stepCount}` },
                    { label: "Business key", value: run.businessKey, mono: true },
                  ]}
                />
              </CardBody>
            </Card>

            <Card>
              <CardHeader title="Step timeline" subtitle="click a task to inspect its payload" />
              {run.steps.length === 0 ? (
                <CardBody>
                  <p className="py-4 text-center text-sm text-slate-400">No tasks recorded yet.</p>
                </CardBody>
              ) : (
                <CardBody className="space-y-1">
                  <Timeline steps={run.steps} runStartedAt={run.startedAt} runCompletedAt={run.completedAt} onSelect={setSelected} />
                </CardBody>
              )}
            </Card>

            <div className="grid grid-cols-1 gap-4 xl:grid-cols-2">
              <Card>
                <CardHeader title="Alerts" subtitle="raised for this execution" />
                {run.alerts.length === 0 ? (
                  <CardBody>
                    <p className="py-4 text-center text-sm text-slate-400">No alerts.</p>
                  </CardBody>
                ) : (
                  <ul className="divide-y divide-slate-100">
                    {run.alerts.map((alert) => (
                      <li key={alert.id} className="flex items-start justify-between gap-3 px-4 py-2.5 text-sm">
                        <div className="min-w-0">
                          <div className="flex items-center gap-1.5">
                            <BellRing className="size-3.5 text-slate-400" />
                            <AlertTypeBadge type={alert.type} />
                            <SeverityBadge severity={alert.severity} />
                            {alert.resolved ? (
                              <span className="text-[11px] text-emerald-600">resolved</span>
                            ) : null}
                          </div>
                          <div className="mt-0.5 text-xs text-slate-500">
                            {alert.message || `${alert.nodeKey}`} · {formatRelative(alert.createdAt)}
                          </div>
                        </div>
                        {!alert.acknowledged ? (
                          <Button
                            variant="secondary"
                            size="sm"
                            onClick={() => ack.mutate(alert.id)}
                            disabled={ack.isPending}
                          >
                            <Check className="size-3.5" />
                            Ack
                          </Button>
                        ) : (
                          <span className="shrink-0 text-[11px] text-slate-400">acknowledged</span>
                        )}
                      </li>
                    ))}
                  </ul>
                )}
              </Card>

              <Card>
                <CardHeader
                  title={
                    <span className="flex items-center gap-2">
                      <History className="size-4 text-slate-400" />
                      Previous runs
                    </span>
                  }
                  subtitle={`other executions of ${run.businessKey}`}
                />
                {run.previousRuns.length === 0 ? (
                  <CardBody>
                    <p className="py-4 text-center text-sm text-slate-400">
                      This is the first recorded execution for this business key.
                    </p>
                  </CardBody>
                ) : (
                  <RunTable runs={run.previousRuns} showFlow={false} />
                )}
              </Card>
            </div>

            <div className="grid grid-cols-1 gap-4 xl:grid-cols-2">
              <Card>
                <CardHeader title="Raw event log" subtitle="what the systems actually sent" />
                <CardBody>
                  {run.events.length === 0 ? (
                    <p className="py-4 text-center text-sm text-slate-400">No raw events.</p>
                  ) : (
                    <ul className="space-y-2">
                      {run.events.map((event) => (
                        <li key={event.id} className="rounded-lg border border-slate-100 p-2">
                          <div className="flex items-center justify-between text-xs text-slate-500">
                            <span className="flex items-center gap-1.5">
                              <span className="font-medium text-slate-700">{event.nodeKey}</span>
                              <StatusBadge
                                status={
                                  event.status === "started"
                                    ? "running"
                                    : event.status === "completed"
                                      ? "completed"
                                      : event.status === "failed"
                                        ? "failed"
                                        : "skipped"
                                }
                              />
                              {event.source ? <span className="text-slate-400">{event.source}</span> : null}
                            </span>
                            <span>{formatDateTime(event.occurredAt)}</span>
                          </div>
                          {event.error ? (
                            <div className="mt-1 text-xs text-rose-600">{event.error}</div>
                          ) : null}
                          <div className="mt-1">
                            <JsonBlock value={event.payload} maxHeight="10rem" />
                          </div>
                        </li>
                      ))}
                    </ul>
                  )}
                </CardBody>
              </Card>

              <Card>
                <CardHeader title="Metadata" />
                <CardBody>
                  <JsonBlock value={run.metadata} emptyLabel="no metadata" maxHeight="20rem" />
                </CardBody>
              </Card>
            </div>

            <p className="flex items-center gap-1.5 px-1 text-xs text-slate-400">
              <ExternalLink className="size-3.5" />
              live — refreshes every 5 seconds
            </p>
          </>
        ) : null}
      </div>

      <StepDetailDrawer
        runId={selected?.runId ?? null}
        stepId={selected?.id ?? null}
        open={selected !== null}
        onClose={() => setSelected(null)}
      />
    </div>
  );
}

function Timeline({
  steps,
  runStartedAt,
  runCompletedAt,
  onSelect,
}: {
  steps: StepRun[];
  runStartedAt: string;
  runCompletedAt: string | null;
  onSelect: (step: StepRun) => void;
}) {
  const start = Math.min(dayjs(runStartedAt).valueOf(), ...steps.map((s) => dayjs(s.startedAt).valueOf()));
  const ends = steps.map((s) => (s.completedAt ? dayjs(s.completedAt).valueOf() : Date.now()));
  const end = Math.max(dayjs(runCompletedAt ?? undefined).valueOf() || Date.now(), Date.now(), ...ends);
  const span = Math.max(1, end - start);

  return (
    <div className="space-y-1.5">
      {steps.map((step) => {
        const stepStart = dayjs(step.startedAt).valueOf();
        const stepEnd = step.completedAt ? dayjs(step.completedAt).valueOf() : Date.now();
        const left = ((stepStart - start) / span) * 100;
        const width = Math.max(0.6, ((stepEnd - stepStart) / span) * 100);
        return (
          <button
            key={step.id}
            type="button"
            onClick={() => onSelect(step)}
            className="flex w-full items-center gap-3 rounded-md px-1 py-0.5 text-left hover:bg-slate-50"
          >
            <span className="w-44 shrink-0 truncate text-xs text-slate-600">
              <span className="mr-1 text-slate-400 tabular-nums">#{step.sequence}</span>
              {step.nodeName || step.nodeKey}
            </span>
            <span className="relative h-4 flex-1 rounded bg-slate-100">
              <span
                className={`absolute top-0 h-4 rounded ${stepColor[step.status] ?? "bg-slate-400"} ${
                  step.status === "running" ? "animate-pulse" : ""
                }`}
                style={{ left: `${left}%`, width: `${width}%`, minWidth: 4 }}
                title={`${step.status} · ${formatDuration(step.completedAt ? step.durationMs : step.elapsedMs)}`}
              />
            </span>
            <span className="w-20 shrink-0 text-right text-xs tabular-nums text-slate-500">
              {formatDuration(step.completedAt ? step.durationMs : step.elapsedMs)}
            </span>
            {step.stuck ? <StuckBadge compact /> : <span className="w-12" />}
          </button>
        );
      })}
    </div>
  );
}
