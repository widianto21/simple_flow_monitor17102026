import { useState } from "react";
import { Link } from "react-router-dom";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { BellOff, Check, ExternalLink, RefreshCw } from "lucide-react";
import { ackAlert, errorMessage, listAlerts } from "@/lib/api";
import { formatRelative } from "@/lib/format";
import { Button } from "@/components/ui/Form";
import { Card, EmptyState, ErrorState, Spinner } from "@/components/ui/primitives";
import { AlertTypeBadge, SeverityBadge } from "@/components/ui/badges";
import { PageBody, PageHeader } from "@/components/ui/PageHeader";
import { Tabs } from "@/components/ui/Tabs";

const statusTabs = [
  { id: "open", label: "Open" },
  { id: "acked", label: "Acknowledged" },
  { id: "resolved", label: "Resolved" },
  { id: "all", label: "All" },
];

export default function Alerts() {
  const [status, setStatus] = useState("open");
  const queryClient = useQueryClient();

  const query = useQuery({
    queryKey: ["alerts", status],
    queryFn: () => listAlerts(status, 200),
    refetchInterval: 5_000,
  });

  const ack = useMutation({
    mutationFn: (id: number) => ackAlert(id),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ["alerts"] }),
  });

  const alerts = query.data?.items ?? [];

  return (
    <div className="flex h-full flex-col">
      <PageHeader
        title="Alerts"
        subtitle="Stuck and failure alerts raised by the watcher"
        actions={
          <Button variant="secondary" size="sm" onClick={() => query.refetch()} disabled={query.isFetching}>
            <RefreshCw className={`size-3.5 ${query.isFetching ? "animate-spin" : ""}`} />
            Refresh
          </Button>
        }
      />

      <PageBody>
        <Tabs tabs={statusTabs} active={status} onChange={setStatus} />

        {query.isLoading ? <Spinner label="Loading alerts…" /> : null}
        {query.isError ? (
          <ErrorState message={errorMessage(query.error)} onRetry={() => query.refetch()} />
        ) : null}

        {query.data ? (
          <Card>
            {alerts.length === 0 ? (
              <EmptyState
                title={status === "open" ? "No open alerts" : "No alerts here"}
                hint="Alerts appear when a task breaches its SLA or an execution fails."
              />
            ) : (
              <ul className="divide-y divide-slate-100">
                {alerts.map((alert) => (
                  <li key={alert.id} className="flex items-start justify-between gap-4 px-4 py-3">
                    <div className="min-w-0">
                      <div className="flex flex-wrap items-center gap-1.5">
                        <AlertTypeBadge type={alert.type} />
                        <SeverityBadge severity={alert.severity} />
                        {alert.resolved ? (
                          <span className="inline-flex items-center gap-1 text-[11px] text-emerald-600">
                            <BellOff className="size-3" />
                            resolved {formatRelative(alert.resolvedAt)}
                          </span>
                        ) : null}
                        {alert.acknowledged && !alert.resolved ? (
                          <span className="text-[11px] text-slate-400">acknowledged</span>
                        ) : null}
                      </div>
                      <div className="mt-1 text-sm text-slate-800">{alert.message || `${alert.type} on ${alert.nodeKey}`}</div>
                      <div className="mt-0.5 flex flex-wrap items-center gap-3 text-xs text-slate-500">
                        <Link to={`/flows/${alert.flowKey}`} className="hover:text-indigo-700">
                          {alert.flowName || alert.flowKey}
                        </Link>
                        <span className="font-mono">{alert.nodeKey}</span>
                        <span className="font-mono">{alert.businessKey}</span>
                        <Link
                          to={`/runs/${alert.runId}`}
                          className="inline-flex items-center gap-1 font-medium text-indigo-600 hover:text-indigo-800"
                        >
                          execution #{alert.runId}
                          <ExternalLink className="size-3" />
                        </Link>
                        <span className="text-slate-400">{formatRelative(alert.createdAt)}</span>
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
                        Acknowledge
                      </Button>
                    ) : (
                      <span className="shrink-0 pt-1 text-xs text-slate-400">
                        {alert.acknowledgedAt ? formatRelative(alert.acknowledgedAt) : "acked"}
                      </span>
                    )}
                  </li>
                ))}
              </ul>
            )}
          </Card>
        ) : null}
      </PageBody>
    </div>
  );
}
