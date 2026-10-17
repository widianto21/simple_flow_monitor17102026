import { Link } from "react-router-dom";
import type { RunSummary } from "@/lib/types";
import { formatDuration, formatRelative } from "@/lib/format";
import { StatusBadge, StuckBadge } from "@/components/ui/badges";
import { EmptyState } from "@/components/ui/primitives";

/** Compact execution table shared by the dashboard and the runs page. */
export function RunTable({
  runs,
  empty = "No executions found.",
  showFlow = true,
}: {
  runs: RunSummary[];
  empty?: string;
  showFlow?: boolean;
}) {
  if (runs.length === 0) {
    return <EmptyState title={empty} />;
  }
  return (
    <div className="overflow-x-auto">
      <table className="w-full text-left text-sm">
        <thead className="border-b border-slate-100 text-xs text-slate-500">
          <tr>
            <th className="px-4 py-2 font-medium">Business key</th>
            {showFlow ? <th className="px-4 py-2 font-medium">Flow</th> : null}
            <th className="px-4 py-2 font-medium">Status</th>
            <th className="px-4 py-2 font-medium">Current step</th>
            <th className="px-4 py-2 text-right font-medium">Duration</th>
            <th className="px-4 py-2 font-medium">Started</th>
          </tr>
        </thead>
        <tbody className="divide-y divide-slate-100">
          {runs.map((run) => (
            <tr key={run.id} className="hover:bg-slate-50">
              <td className="px-4 py-2">
                <Link
                  to={`/runs/${run.id}`}
                  className="font-mono text-xs font-medium text-indigo-600 hover:text-indigo-800"
                >
                  {run.businessKey}
                </Link>
                <div className="text-[11px] text-slate-400">#{run.id}</div>
              </td>
              {showFlow ? (
                <td className="px-4 py-2">
                  <Link
                    to={`/flows/${run.flowKey}`}
                    className="text-slate-700 hover:text-indigo-700"
                  >
                    {run.flowName || run.flowKey}
                  </Link>
                </td>
              ) : null}
              <td className="px-4 py-2">
                <span className="flex items-center gap-1.5">
                  <StatusBadge status={run.status} />
                  {run.stuck ? <StuckBadge compact /> : null}
                </span>
              </td>
              <td className="px-4 py-2 text-slate-600">
                {run.currentNodeName || run.currentNodeKey || "—"}
              </td>
              <td className="px-4 py-2 text-right tabular-nums text-slate-600">
                {formatDuration(run.completedAt ? run.durationMs : run.elapsedMs)}
              </td>
              <td className="px-4 py-2 text-xs text-slate-500">{formatRelative(run.startedAt)}</td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}
