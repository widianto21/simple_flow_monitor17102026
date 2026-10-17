import type { StepRun } from "@/lib/types";
import { formatDateTime, formatDuration } from "@/lib/format";
import { KindBadge, StatusBadge, StuckBadge } from "@/components/ui/badges";
import { EmptyState } from "@/components/ui/primitives";

/** Task (step) table shared by the tasks/stuck views. */
export function TaskTable({
  tasks,
  empty = "No tasks found.",
  showFlow = true,
  onSelect,
}: {
  tasks: StepRun[];
  empty?: string;
  showFlow?: boolean;
  onSelect?: (runId: number, stepId: number) => void;
}) {
  if (tasks.length === 0) {
    return <EmptyState title={empty} />;
  }
  return (
    <div className="overflow-x-auto">
      <table className="w-full text-left text-sm">
        <thead className="border-b border-slate-100 text-xs text-slate-500">
          <tr>
            <th className="px-4 py-2 font-medium">Process</th>
            {showFlow ? <th className="px-4 py-2 font-medium">Flow</th> : null}
            <th className="px-4 py-2 font-medium">Business key</th>
            <th className="px-4 py-2 font-medium">Status</th>
            <th className="px-4 py-2 text-right font-medium">Duration</th>
            <th className="px-4 py-2 font-medium">Started</th>
          </tr>
        </thead>
        <tbody className="divide-y divide-slate-100">
          {tasks.map((task) => (
            <tr
              key={task.id}
              onClick={onSelect ? () => onSelect(task.runId, task.id) : undefined}
              className={onSelect ? "cursor-pointer hover:bg-slate-50" : "hover:bg-slate-50"}
            >
              <td className="px-4 py-2">
                <div className="flex items-center gap-1.5">
                  <span className="font-medium text-slate-800">{task.nodeName || task.nodeKey}</span>
                  <KindBadge kind={task.nodeKind} />
                </div>
                {task.system ? (
                  <div className="text-[11px] text-slate-400">{task.system}</div>
                ) : null}
              </td>
              {showFlow ? (
                <td className="px-4 py-2 text-slate-600">{task.flowName || task.flowKey}</td>
              ) : null}
              <td className="px-4 py-2 font-mono text-xs text-slate-600">{task.businessKey}</td>
              <td className="px-4 py-2">
                <span className="flex items-center gap-1.5">
                  <StatusBadge status={task.status} />
                  {task.stuck ? <StuckBadge compact /> : null}
                </span>
              </td>
              <td className="px-4 py-2 text-right tabular-nums text-slate-600">
                {formatDuration(task.completedAt ? task.durationMs : task.elapsedMs)}
              </td>
              <td className="px-4 py-2 text-xs text-slate-500">{formatDateTime(task.startedAt)}</td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}
