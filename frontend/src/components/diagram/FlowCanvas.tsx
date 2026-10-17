import { useCallback, useMemo, useRef, useState } from "react";
import {
  Background,
  Controls,
  MarkerType,
  MiniMap,
  ReactFlow,
  type Edge,
  type Node,
  type NodeMouseHandler,
} from "@xyflow/react";
import "@xyflow/react/dist/style.css";
import type { FlowEdge, FlowNode } from "@/lib/types";
import { ProcessNode, type ProcessNodeData } from "./ProcessNode";
import { NodeHoverCard } from "./NodeHoverCard";

const nodeTypes = { process: ProcessNode };

interface HoverState {
  node: FlowNode;
  x: number;
  y: number;
}

/** Read-only live diagram: hover → tooltip, click → drill-down. */
export function FlowCanvas({
  nodes,
  edges,
  onSelectNode,
  selectedKey,
}: {
  nodes: FlowNode[];
  edges: FlowEdge[];
  onSelectNode: (node: FlowNode) => void;
  selectedKey?: string;
}) {
  const containerRef = useRef<HTMLDivElement>(null);
  const [hover, setHover] = useState<HoverState | null>(null);

  const flowNodes: Node<ProcessNodeData>[] = useMemo(
    () =>
      nodes.map((node) => ({
        id: node.key,
        type: "process",
        position: { x: node.posX, y: node.posY },
        data: { node },
        selected: node.key === selectedKey,
        draggable: false,
      })),
    [nodes, selectedKey],
  );

  const runningKeys = useMemo(
    () => new Set(nodes.filter((node) => node.stats.running > 0).map((node) => node.key)),
    [nodes],
  );

  const flowEdges: Edge[] = useMemo(
    () =>
      edges.map((edge) => {
        const active = runningKeys.has(edge.sourceKey);
        return {
          id: `e-${edge.id}`,
          source: edge.sourceKey,
          target: edge.targetKey,
          label: edge.label || undefined,
          type: "smoothstep",
          animated: active,
          style: { stroke: active ? "#6366f1" : "#94a3b8", strokeWidth: active ? 2 : 1.5 },
          labelStyle: { fontSize: 10, fill: "#475569" },
          labelBgStyle: { fill: "#f8fafc" },
          markerEnd: { type: MarkerType.ArrowClosed, color: active ? "#6366f1" : "#94a3b8" },
        };
      }),
    [edges, runningKeys],
  );

  const localPoint = useCallback((event: React.MouseEvent) => {
    const rect = containerRef.current?.getBoundingClientRect();
    if (!rect) return { x: event.clientX, y: event.clientY };
    return { x: event.clientX - rect.left, y: event.clientY - rect.top };
  }, []);

  const onNodeMouseEnter: NodeMouseHandler = useCallback(
    (event, node) => {
      const domain = (node.data as ProcessNodeData).node;
      setHover({ node: domain, ...localPoint(event) });
    },
    [localPoint],
  );

  const onNodeMouseMove: NodeMouseHandler = useCallback(
    (event, node) => {
      const domain = (node.data as ProcessNodeData).node;
      setHover({ node: domain, ...localPoint(event) });
    },
    [localPoint],
  );

  const onNodeMouseLeave: NodeMouseHandler = useCallback(() => setHover(null), []);

  const onNodeClick: NodeMouseHandler = useCallback(
    (_event, node) => {
      onSelectNode((node.data as ProcessNodeData).node);
    },
    [onSelectNode],
  );

  return (
    <div ref={containerRef} className="relative h-full w-full">
      <ReactFlow
        nodes={flowNodes}
        edges={flowEdges}
        nodeTypes={nodeTypes}
        onNodeClick={onNodeClick}
        onNodeMouseEnter={onNodeMouseEnter}
        onNodeMouseMove={onNodeMouseMove}
        onNodeMouseLeave={onNodeMouseLeave}
        onPaneClick={() => setHover(null)}
        fitView
        minZoom={0.2}
        maxZoom={1.6}
        proOptions={{ hideAttribution: true }}
        nodesConnectable={false}
        elementsSelectable
      >
        <Background gap={18} color="#e2e8f0" />
        <Controls showInteractive={false} />
        <MiniMap pannable zoomable className="!bg-slate-100" nodeColor="#c7d2fe" />
      </ReactFlow>

      {hover ? (
        <div
          className="pointer-events-none absolute z-20"
          style={{
            left: Math.min(hover.x + 16, (containerRef.current?.clientWidth ?? 800) - 300),
            top: Math.max(8, hover.y + 16),
          }}
        >
          <NodeHoverCard node={hover.node} />
        </div>
      ) : null}
    </div>
  );
}
