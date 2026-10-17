import { Link } from "react-router-dom";
import { Pencil, Workflow } from "lucide-react";
import type { FlowSummary } from "@/lib/types";
import { formatDurationShort, formatPercent, formatRelative } from "@/lib/format";
import { Badge } from "@/components/ui/badges";
import { EmptyState } from "@/components/ui/primitives";

/** Flow list with live counters, shared by the dashboard and the flows page. */
export function FlowTable({
  flows,
  empty = "No flows defined yet.",
  showActions = true,
}: {
  flows: FlowSummary[];
  empty?: string;
  showActions?: boolean;
}) {
  if (flows.length === 0) {
    return <EmptyState title={empty} />;
  }
  return (
    <div className="overflow-x-auto">
      <table className="w-full text-left text-sm">
        <thead className="border-b border-slate-100 text-xs text-slate-500">
          <tr>
            <th className="px-4 py-2 font-medium">Flow</th>
            <th className="px-4 py-2 font-medium">Owner</th>
            <th className="px-4 py-2 text-right font-medium">Nodes</th>
            <th className="px-4 py-2 text-right font-medium">Running</th>
            <th className="px-4 py-2 text-right font-medium">Stuck</th>
            <th className="px-4 py-2 text-right font-medium">Success 24h</th>
            <th className="px-4 py-2 text-right font-medium">Avg duration</th>
            <th className="px-4 py-2 font-medium">Last activity</th>
            {showActions ? <th className="px-4 py-2" /> : null}
          </tr>
        </thead>
        <tbody className="divide-y divide-slate-100">
          {flows.map((flow) => (
            <tr key={flow.id} className="hover:bg-slate-50">
              <td className="px-4 py-2">
                <Link to={`/flows/${flow.key}`} className="flex items-center gap-2">
                  <Workflow className="size-4 text-indigo-500" />
                  <span>
                    <span className="font-medium text-slate-800">{flow.name}</span>
                    <span className="block font-mono text-[11px] text-slate-400">{flow.key}</span>
                  </span>
                </Link>
              </td>
              <td className="px-4 py-2 text-slate-600">
                {flow.owner || "—"}
                {flow.category ? (
                  <span className="block text-[11px] text-slate-400">{flow.category}</span>
                ) : null}
              </td>
              <td className="px-4 py-2 text-right tabular-nums text-slate-600">{flow.nodeCount}</td>
              <td className="px-4 py-2 text-right tabular-nums">
                <span className="text-slate-800">{flow.running}</span>
                <span className="text-slate-400"> exec · </span>
                <span className="text-indigo-700">{flow.runningTasks}</span>
                <span className="text-slate-400"> tasks</span>
              </td>
              <td className="px-4 py-2 text-right">
                {flow.stuckTasks > 0 ? (
                  <Badge tone="amber">{flow.stuckTasks}</Badge>
                ) : (
                  <span className="text-slate-400">0</span>
                )}
              </td>
              <td className="px-4 py-2 text-right tabular-nums text-slate-600">
                {formatPercent(flow.successRate)}
                <span className="block text-[11px] text-slate-400">
                  {flow.completed24h} ok · {flow.failed24h} fail
                </span>
              </td>
              <td className="px-4 py-2 text-right tabular-nums text-slate-600">
                {formatDurationShort(flow.avgDurationMs)}
              </td>
              <td className="px-4 py-2 text-xs text-slate-500">
                {formatRelative(flow.lastActivityAt)}
              </td>
              {showActions ? (
                <td className="px-4 py-2 text-right">
                  <Link
                    to={`/flows/${flow.key}/edit`}
                    className="inline-flex items-center gap-1 rounded-md border border-slate-200 px-2 py-1 text-xs font-medium text-slate-600 hover:bg-slate-50"
                  >
                    <Pencil className="size-3" />
                    Edit
                  </Link>
                </td>
              ) : null}
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}
