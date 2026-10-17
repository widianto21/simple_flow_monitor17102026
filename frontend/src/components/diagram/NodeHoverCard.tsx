import { Clock, Cpu, Database, GitBranch, Timer, TriangleAlert, Users, Zap } from "lucide-react";
import type { FlowNode } from "@/lib/types";
import { formatDuration, formatPercent, formatRelative } from "@/lib/format";
import { KindBadge, StuckBadge } from "@/components/ui/badges";

/** Compact tooltip shown while hovering a diagram node. */
export function NodeHoverCard({ node }: { node: FlowNode }) {
  const stats = node.stats;
  return (
    <div className="w-72 rounded-lg border border-slate-200 bg-white/98 p-3 text-xs shadow-lg backdrop-blur">
      <div className="flex items-center justify-between gap-2">
        <div className="truncate text-sm font-semibold text-slate-900">{node.name || node.key}</div>
        <KindBadge kind={node.kind} />
      </div>
      {node.description ? (
        <p className="mt-1 line-clamp-2 text-slate-500">{node.description}</p>
      ) : null}

      <dl className="mt-2 space-y-1 text-slate-600">
        <Row icon={<Cpu className="size-3.5" />} label="System" value={node.system || "—"} />
        <Row
          icon={<GitBranch className="size-3.5" />}
          label="Endpoint"
          value={node.endpoint ? `${node.method} ${node.endpoint}` : "—"}
          mono
        />
        <Row icon={<Database className="size-3.5" />} label="Repository" value={node.repository || "—"} mono />
        <Row icon={<Users className="size-3.5" />} label="Team" value={node.team || "—"} />
        <Row icon={<Clock className="size-3.5" />} label="SLA" value={`${node.timeoutMinutes} min`} />
      </dl>

      <div className="mt-2 flex flex-wrap items-center gap-2 border-t border-slate-100 pt-2">
        <span className="inline-flex items-center gap-1 font-medium text-indigo-700">
          <Zap className="size-3.5" />
          {stats.running} running
        </span>
        {stats.stuck > 0 ? <StuckBadge compact /> : null}
        <span className="inline-flex items-center gap-1 text-slate-500">
          <Timer className="size-3.5" />
          avg {formatDuration(stats.avgDurationMs)} · p95 {formatDuration(stats.p95DurationMs)}
        </span>
      </div>
      <div className="mt-1 flex items-center justify-between text-slate-400">
        <span>{formatPercent(stats.failureRate, 1)} fail (24h)</span>
        <span>last {formatRelative(stats.lastActivityAt)}</span>
      </div>
      {stats.stuck > 0 ? (
        <div className="mt-2 inline-flex items-center gap-1 rounded bg-amber-50 px-2 py-1 text-amber-700">
          <TriangleAlert className="size-3.5" />
          {stats.stuck} task{stats.stuck === 1 ? "" : "s"} over SLA
        </div>
      ) : null}
    </div>
  );
}

function Row({
  icon,
  label,
  value,
  mono,
}: {
  icon: React.ReactNode;
  label: string;
  value: string;
  mono?: boolean;
}) {
  return (
    <div className="flex items-start gap-2">
      <span className="mt-0.5 text-slate-400">{icon}</span>
      <span className="w-16 shrink-0 text-slate-400">{label}</span>
      <span className={`min-w-0 flex-1 break-words text-slate-700 ${mono ? "font-mono" : ""}`}>
        {value}
      </span>
    </div>
  );
}
