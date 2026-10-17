import { memo } from "react";
import { Handle, Position, type Node, type NodeProps } from "@xyflow/react";
import {
  Cloud,
  Cog,
  Flag,
  GitMerge,
  PlayCircle,
  Split,
  Timer,
  TriangleAlert,
} from "lucide-react";
import type { FlowNode, NodeKind } from "@/lib/types";
import { cx } from "@/lib/cx";
import { formatDurationShort } from "@/lib/format";

export type ProcessNodeData = Record<string, unknown> & { node: FlowNode };
export type ProcessNodeType = Node<ProcessNodeData, "process">;

const kindStyles: Record<NodeKind, { border: string; chip: string; icon: string }> = {
  start: { border: "border-emerald-300", chip: "bg-emerald-100 text-emerald-700", icon: "text-emerald-600" },
  task: { border: "border-sky-300", chip: "bg-sky-100 text-sky-700", icon: "text-sky-600" },
  decision: { border: "border-amber-300", chip: "bg-amber-100 text-amber-700", icon: "text-amber-600" },
  gateway: { border: "border-violet-300", chip: "bg-violet-100 text-violet-700", icon: "text-violet-600" },
  external: { border: "border-cyan-300", chip: "bg-cyan-100 text-cyan-700", icon: "text-cyan-600" },
  end: { border: "border-rose-300", chip: "bg-rose-100 text-rose-700", icon: "text-rose-600" },
};

function KindIcon({ kind, className }: { kind: NodeKind; className?: string }) {
  const props = { className };
  switch (kind) {
    case "start":
      return <PlayCircle {...props} />;
    case "decision":
      return <Split {...props} />;
    case "gateway":
      return <GitMerge {...props} />;
    case "external":
      return <Cloud {...props} />;
    case "end":
      return <Flag {...props} />;
    case "task":
    default:
      return <Cog {...props} />;
  }
}

function ProcessNodeInner({ data, selected }: NodeProps<ProcessNodeType>) {
  const node = data.node;
  const stats = node.stats;
  const style = kindStyles[node.kind] ?? kindStyles.task;

  const liveTone = stats.stuck > 0 ? "stuck" : stats.running > 0 ? "running" : "idle";

  return (
    <div
      className={cx(
        "w-56 rounded-xl border-2 bg-white shadow-sm transition-shadow",
        style.border,
        selected ? "ring-2 ring-indigo-400 ring-offset-1" : "",
        liveTone === "stuck" ? "shadow-amber-200/70" : "",
      )}
    >
      <Handle type="target" position={Position.Left} className="!size-2 !bg-slate-400" />
      <Handle type="source" position={Position.Right} className="!size-2 !bg-slate-400" />

      <div className="flex items-center gap-2 px-3 pt-2.5">
        <KindIcon kind={node.kind} className={cx("size-4", style.icon)} />
        <div className="min-w-0 flex-1 truncate text-sm font-semibold text-slate-800">
          {node.name || node.key}
        </div>
        {liveTone === "running" ? (
          <span className="relative flex size-2.5" title={`${stats.running} running`}>
            <span className="absolute inline-flex size-full animate-ping rounded-full bg-indigo-400 opacity-75" />
            <span className="relative inline-flex size-2.5 rounded-full bg-indigo-500" />
          </span>
        ) : null}
        {liveTone === "stuck" ? (
          <span className="relative flex size-2.5" title={`${stats.stuck} stuck`}>
            <span className="absolute inline-flex size-full animate-ping rounded-full bg-amber-400 opacity-75" />
            <span className="relative inline-flex size-2.5 rounded-full bg-amber-500" />
          </span>
        ) : null}
      </div>

      <div className="px-3 pb-2 pt-0.5">
        <div className="flex items-center gap-1.5 text-xs text-slate-500">
          <span className={cx("rounded px-1.5 py-0.5 text-[10px] font-medium", style.chip)}>
            {node.kind}
          </span>
          <span className="truncate">{node.system || "unassigned"}</span>
        </div>
        {node.endpoint ? (
          <div className="mt-1 truncate font-mono text-[11px] text-slate-400">
            {node.method ? `${node.method} ` : ""}
            {node.endpoint}
          </div>
        ) : null}
      </div>

      <div className="flex items-center gap-1 border-t border-slate-100 px-3 py-1.5 text-[11px]">
        <span
          className={cx(
            "inline-flex items-center gap-1 rounded px-1.5 py-0.5 font-medium",
            stats.running > 0 ? "bg-indigo-50 text-indigo-700" : "text-slate-400",
          )}
          title="tasks currently running"
        >
          <PlayCircle className="size-3" />
          {stats.running}
        </span>
        {stats.stuck > 0 ? (
          <span
            className="inline-flex items-center gap-1 rounded bg-amber-50 px-1.5 py-0.5 font-medium text-amber-700"
            title="tasks over SLA"
          >
            <TriangleAlert className="size-3" />
            {stats.stuck}
          </span>
        ) : null}
        <span
          className="ml-auto inline-flex items-center gap-1 text-slate-400"
          title={`avg ${formatDurationShort(stats.avgDurationMs)} · p95 ${formatDurationShort(
            stats.p95DurationMs,
          )} · fail ${(stats.failureRate * 100).toFixed(0)}%`}
        >
          <Timer className="size-3" />
          {formatDurationShort(stats.avgDurationMs)}
        </span>
      </div>
    </div>
  );
}

export const ProcessNode = memo(ProcessNodeInner);
