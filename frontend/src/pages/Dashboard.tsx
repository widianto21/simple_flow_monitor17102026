import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import {
  Bar,
  CartesianGrid,
  ComposedChart,
  Legend,
  Line,
  ResponsiveContainer,
  Tooltip,
  XAxis,
  YAxis,
} from "recharts";
import {
  Activity,
  AlertTriangle,
  Bell,
  CheckCircle2,
  Clock,
  Database,
  Gauge,
  ListChecks,
  PlayCircle,
  RefreshCw,
  Timer,
  Workflow,
  XCircle,
} from "lucide-react";
import { Link } from "react-router-dom";
import { errorMessage, getDashboard, runDemoSeed } from "@/lib/api";
import { formatDay, formatDuration, formatHour, formatNumber, formatPercent, formatRelative } from "@/lib/format";
import { Button } from "@/components/ui/Form";
import { Card, CardBody, CardHeader, ErrorState, Spinner, StatCard } from "@/components/ui/primitives";
import { AlertTypeBadge, SeverityBadge } from "@/components/ui/badges";
import { PageBody, PageHeader } from "@/components/ui/PageHeader";
import { FlowTable } from "@/components/FlowTable";
import { RunTable } from "@/components/RunTable";

export default function Dashboard() {
  const queryClient = useQueryClient();

  const query = useQuery({
    queryKey: ["dashboard"],
    queryFn: getDashboard,
    refetchInterval: 5_000,
  });

  const seed = useMutation({
    mutationFn: runDemoSeed,
    onSuccess: () => {
      queryClient.invalidateQueries();
    },
  });

  const board = query.data;

  return (
    <div className="flex h-full flex-col">
      <PageHeader
        title="Dashboard"
        subtitle={
          board
            ? `Live business-flow telemetry · updated ${formatRelative(board.generatedAt)}`
            : "Live business-flow telemetry"
        }
        actions={
          <>
            <Button
              variant="secondary"
              size="sm"
              onClick={() => query.refetch()}
              disabled={query.isFetching}
            >
              <RefreshCw className={`size-3.5 ${query.isFetching ? "animate-spin" : ""}`} />
              Refresh
            </Button>
            <Button
              variant="secondary"
              size="sm"
              onClick={() => seed.mutate()}
              disabled={seed.isPending}
            >
              <Database className="size-3.5" />
              {seed.isPending ? "Seeding…" : "Seed demo traffic"}
            </Button>
          </>
        }
      />

      <PageBody>
        {query.isLoading ? <Spinner label="Loading dashboard…" /> : null}
        {query.isError ? (
          <ErrorState message={errorMessage(query.error)} onRetry={() => query.refetch()} />
        ) : null}
        {seed.isError ? <ErrorState message={errorMessage(seed.error)} /> : null}

        {board ? (
          <>
            <div className="grid grid-cols-2 gap-3 lg:grid-cols-4">
              <StatCard
                label="Running executions"
                value={formatNumber(board.kpis.runningExecutions)}
                tone="info"
                icon={<Activity className="size-4" />}
                hint={`${formatNumber(board.kpis.runningTasks)} tasks in flight`}
              />
              <StatCard
                label="Stuck tasks"
                value={formatNumber(board.kpis.stuckTasks)}
                tone={board.kpis.stuckTasks > 0 ? "warn" : "good"}
                icon={<Timer className="size-4" />}
                hint="over SLA right now"
              />
              <StatCard
                label="Open alerts"
                value={formatNumber(board.kpis.openAlerts)}
                tone={board.kpis.openAlerts > 0 ? "bad" : "good"}
                icon={<Bell className="size-4" />}
                hint="unacknowledged"
              />
              <StatCard
                label="Success rate"
                value={formatPercent(board.kpis.successRate, 1)}
                tone="good"
                icon={<Gauge className="size-4" />}
                hint="last 24 hours"
              />
              <StatCard
                label="Completed 24h"
                value={formatNumber(board.kpis.completed24h)}
                tone="good"
                icon={<CheckCircle2 className="size-4" />}
              />
              <StatCard
                label="Failed 24h"
                value={formatNumber(board.kpis.failed24h)}
                tone={board.kpis.failed24h > 0 ? "bad" : "good"}
                icon={<XCircle className="size-4" />}
              />
              <StatCard
                label="Avg cycle time"
                value={formatDuration(board.kpis.avgCycleTimeMs)}
                icon={<Clock className="size-4" />}
              />
              <StatCard
                label="Flows"
                value={formatNumber(board.kpis.flows)}
                icon={<Workflow className="size-4" />}
              />
            </div>

            <div className="grid grid-cols-1 gap-4 xl:grid-cols-2">
              <Card>
                <CardHeader
                  title="Throughput (24h)"
                  subtitle="executions started / completed / failed per hour"
                />
                <CardBody>
                  <div className="h-64">
                    <ResponsiveContainer width="100%" height="100%">
                      <ComposedChart data={board.throughput} margin={{ top: 8, right: 8, bottom: 0, left: -18 }}>
                        <CartesianGrid strokeDasharray="3 3" stroke="#eef2f7" />
                        <XAxis
                          dataKey="hour"
                          tickFormatter={(value) => formatHour(value as string)}
                          tick={{ fontSize: 11, fill: "#94a3b8" }}
                          interval="preserveStartEnd"
                        />
                        <YAxis tick={{ fontSize: 11, fill: "#94a3b8" }} allowDecimals={false} />
                        <Tooltip
                          labelFormatter={(value) => formatHour(value as string)}
                          contentStyle={{ fontSize: 12, borderRadius: 8, border: "1px solid #e2e8f0" }}
                        />
                        <Legend wrapperStyle={{ fontSize: 12 }} />
                        <Bar dataKey="started" name="started" fill="#818cf8" radius={[3, 3, 0, 0]} />
                        <Bar dataKey="completed" name="completed" fill="#34d399" radius={[3, 3, 0, 0]} />
                        <Bar dataKey="failed" name="failed" fill="#fb7185" radius={[3, 3, 0, 0]} />
                      </ComposedChart>
                    </ResponsiveContainer>
                  </div>
                </CardBody>
              </Card>

              <Card>
                <CardHeader title="Success trend (7d)" subtitle="completed vs failed, success rate" />
                <CardBody>
                  <div className="h-64">
                    <ResponsiveContainer width="100%" height="100%">
                      <ComposedChart data={board.trend} margin={{ top: 8, right: 8, bottom: 0, left: -18 }}>
                        <CartesianGrid strokeDasharray="3 3" stroke="#eef2f7" />
                        <XAxis
                          dataKey="day"
                          tickFormatter={(value) => formatDay(value as string)}
                          tick={{ fontSize: 11, fill: "#94a3b8" }}
                        />
                        <YAxis yAxisId="count" tick={{ fontSize: 11, fill: "#94a3b8" }} allowDecimals={false} />
                        <YAxis
                          yAxisId="rate"
                          orientation="right"
                          domain={[0, 1]}
                          tickFormatter={(value) => `${Math.round((value as number) * 100)}%`}
                          tick={{ fontSize: 11, fill: "#94a3b8" }}
                        />
                        <Tooltip
                          labelFormatter={(value) => formatDay(value as string)}
                          formatter={(value, name) =>
                            name === "success rate"
                              ? [`${Math.round((value as number) * 100)}%`, name]
                              : [value, name]
                          }
                          contentStyle={{ fontSize: 12, borderRadius: 8, border: "1px solid #e2e8f0" }}
                        />
                        <Legend wrapperStyle={{ fontSize: 12 }} />
                        <Bar yAxisId="count" dataKey="completed" name="completed" fill="#34d399" radius={[3, 3, 0, 0]} />
                        <Bar yAxisId="count" dataKey="failed" name="failed" fill="#fb7185" radius={[3, 3, 0, 0]} />
                        <Line
                          yAxisId="rate"
                          type="monotone"
                          dataKey="successRate"
                          name="success rate"
                          stroke="#6366f1"
                          strokeWidth={2}
                          dot={{ r: 2 }}
                        />
                      </ComposedChart>
                    </ResponsiveContainer>
                  </div>
                </CardBody>
              </Card>
            </div>

            <Card>
              <CardHeader
                title="Business flows"
                subtitle="live counters per flow"
                actions={
                  <Link to="/flows" className="text-xs font-medium text-indigo-600 hover:text-indigo-800">
                    Manage flows
                  </Link>
                }
              />
              <FlowTable flows={board.flows} showActions={false} />
            </Card>

            <div className="grid grid-cols-1 gap-4 xl:grid-cols-2">
              <Card>
                <CardHeader
                  title="Stuck tasks"
                  subtitle="running over SLA"
                  actions={
                    <Link to="/stuck" className="text-xs font-medium text-indigo-600 hover:text-indigo-800">
                      View all
                    </Link>
                  }
                />
                {board.stuck.length === 0 ? (
                  <CardBody>
                    <p className="py-4 text-center text-sm text-slate-400">
                      Nothing is over SLA right now.
                    </p>
                  </CardBody>
                ) : (
                  <ul className="divide-y divide-slate-100">
                    {board.stuck.slice(0, 6).map((task) => (
                      <li key={task.id} className="flex items-center justify-between gap-3 px-4 py-2 text-sm">
                        <div className="min-w-0">
                          <div className="truncate font-medium text-slate-800">
                            {task.nodeName || task.nodeKey}
                          </div>
                          <div className="truncate text-xs text-slate-400">
                            {task.flowName} · <span className="font-mono">{task.businessKey}</span>
                          </div>
                        </div>
                        <div className="text-right">
                          <span className="text-xs font-medium text-amber-600">
                            +{formatDuration(task.overdueMs)} over
                          </span>
                          <div className="text-[11px] text-slate-400">SLA {task.timeoutMinutes}m</div>
                        </div>
                      </li>
                    ))}
                  </ul>
                )}
              </Card>

              <Card>
                <CardHeader
                  title="Recent alerts"
                  actions={
                    <Link to="/alerts" className="text-xs font-medium text-indigo-600 hover:text-indigo-800">
                      View all
                    </Link>
                  }
                />
                {board.recentAlerts.length === 0 ? (
                  <CardBody>
                    <p className="py-4 text-center text-sm text-slate-400">No alerts.</p>
                  </CardBody>
                ) : (
                  <ul className="divide-y divide-slate-100">
                    {board.recentAlerts.slice(0, 6).map((alert) => (
                      <li key={alert.id} className="flex items-start justify-between gap-3 px-4 py-2 text-sm">
                        <div className="min-w-0">
                          <div className="flex items-center gap-1.5">
                            <AlertTypeBadge type={alert.type} />
                            <SeverityBadge severity={alert.severity} />
                            {alert.resolved ? (
                              <span className="text-[11px] text-emerald-600">resolved</span>
                            ) : null}
                          </div>
                          <div className="mt-0.5 truncate text-xs text-slate-500">
                            {alert.message || `${alert.flowName} / ${alert.nodeKey}`}
                          </div>
                        </div>
                        <span className="shrink-0 text-[11px] text-slate-400">
                          {formatRelative(alert.createdAt)}
                        </span>
                      </li>
                    ))}
                  </ul>
                )}
              </Card>
            </div>

            <Card>
              <CardHeader
                title="Recent executions"
                actions={
                  <Link to="/runs" className="text-xs font-medium text-indigo-600 hover:text-indigo-800">
                    All executions
                  </Link>
                }
              />
              <RunTable runs={board.recentRuns} />
            </Card>

            {board.kpis.stuckTasks > 0 ? (
              <p className="flex items-center gap-1.5 px-1 text-xs text-amber-600">
                <AlertTriangle className="size-3.5" />
                {board.kpis.stuckTasks} task(s) breached their SLA — check the stuck tasks view.
              </p>
            ) : null}
            <p className="flex items-center gap-1.5 px-1 text-xs text-slate-400">
              <ListChecks className="size-3.5" />
              <PlayCircle className="size-3.5" />
              data refreshes every 5 seconds
            </p>
          </>
        ) : null}
      </PageBody>
    </div>
  );
}
