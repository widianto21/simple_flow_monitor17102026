import { memo } from "react";
import { Handle, Position, type Node, type NodeProps } from "@xyflow/react";
import type { FlowNodeInput, NodeKind } from "@/lib/types";
import { cx } from "@/lib/cx";

export interface EditorNodeData extends Record<string, unknown> {
  input: FlowNodeInput;
  schemaText: string;
  responseText: string;
}
export type EditorNodeType = Node<EditorNodeData, "editor">;

const kindBorder: Record<NodeKind, string> = {
  start: "border-emerald-300",
  task: "border-sky-300",
  decision: "border-amber-300",
  gateway: "border-violet-300",
  external: "border-cyan-300",
  end: "border-rose-300",
};

function EditorProcessNodeInner({ data, selected }: NodeProps<EditorNodeType>) {
  const input = data.input;
  return (
    <div
      className={cx(
        "w-52 rounded-xl border-2 bg-white p-3 shadow-sm",
        kindBorder[input.kind] ?? kindBorder.task,
        selected ? "ring-2 ring-indigo-400 ring-offset-1" : "",
      )}
    >
      <Handle type="target" position={Position.Left} className="!size-2 !bg-slate-400" />
      <Handle type="source" position={Position.Right} className="!size-2 !bg-slate-400" />
      <div className="truncate text-sm font-semibold text-slate-800">
        {input.name || input.key || "unnamed"}
      </div>
      <div className="mt-0.5 flex items-center gap-1 text-[11px] text-slate-500">
        <span className="rounded bg-slate-100 px-1.5 py-0.5 font-medium text-slate-600">
          {input.kind}
        </span>
        <span className="truncate">{input.system || "unassigned"}</span>
      </div>
      <div className="mt-1 truncate font-mono text-[10px] text-slate-400">
        {input.key || "no-key"}
      </div>
    </div>
  );
}

export const EditorProcessNode = memo(EditorProcessNodeInner);
