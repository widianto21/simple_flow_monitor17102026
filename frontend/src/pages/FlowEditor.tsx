import { useCallback, useEffect, useRef, useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { Link, useNavigate, useParams } from "react-router-dom";
import {
  Background,
  Controls,
  MarkerType,
  MiniMap,
  ReactFlow,
  addEdge,
  useEdgesState,
  useNodesState,
  type Connection,
  type Edge,
} from "@xyflow/react";
import "@xyflow/react/dist/style.css";
import { ArrowLeft, Plus, Save, Trash2, TriangleAlert } from "lucide-react";
import { createFlow, deleteFlow, errorMessage, getFlow, updateFlow } from "@/lib/api";
import { prettyJson } from "@/lib/format";
import type { FlowInput, FlowNodeInput, NodeKind } from "@/lib/types";
import { Button, Field, NumberInput, Select, TextArea, TextInput } from "@/components/ui/Form";
import { EditorProcessNode, type EditorNodeData, type EditorNodeType } from "@/components/diagram/EditorProcessNode";
import { PageHeader } from "@/components/ui/PageHeader";
import { ErrorState, Spinner } from "@/components/ui/primitives";

type EditorEdge = Edge<{ condition: string }>;

const nodeTypes = { editor: EditorProcessNode };

const kindOptions: Array<{ value: NodeKind; label: string }> = [
  { value: "start", label: "start" },
  { value: "task", label: "task" },
  { value: "decision", label: "decision" },
  { value: "gateway", label: "gateway" },
  { value: "external", label: "external" },
  { value: "end", label: "end" },
];

const methodOptions = ["GET", "POST", "PUT", "PATCH", "DELETE"].map((m) => ({ value: m, label: m }));

interface Meta {
  key: string;
  name: string;
  description: string;
  category: string;
  owner: string;
  version: string;
  active: boolean;
}

const emptyMeta: Meta = {
  key: "",
  name: "",
  description: "",
  category: "",
  owner: "",
  version: "1.0.0",
  active: true,
};

function blankInput(kind: NodeKind, x: number, y: number): FlowNodeInput {
  return {
    key: "",
    name: "",
    kind,
    description: "",
    system: "",
    repository: "",
    team: "",
    method: "POST",
    endpoint: "",
    requestSchema: null,
    responseSample: null,
    timeoutMinutes: 5,
    posX: x,
    posY: y,
  };
}

function makeNode(id: string, input: FlowNodeInput, schemaText = "", responseText = ""): EditorNodeType {
  return {
    id,
    type: "editor",
    position: { x: input.posX, y: input.posY },
    data: { input, schemaText, responseText },
  };
}

function makeEdge(id: string, source: string, target: string): EditorEdge {
  return {
    id,
    source,
    target,
    type: "smoothstep",
    markerEnd: { type: MarkerType.ArrowClosed, color: "#94a3b8" },
    style: { stroke: "#94a3b8", strokeWidth: 1.5 },
    data: { condition: "" },
  };
}

/** Create / edit a flow definition (diagram nodes, edges and node contracts). */
export default function FlowEditor() {
  const { idOrKey = "new" } = useParams();
  const isNew = idOrKey === "new";
  const navigate = useNavigate();
  const queryClient = useQueryClient();

  const [meta, setMeta] = useState<Meta>(emptyMeta);
  const [nodes, setNodes, onNodesChange] = useNodesState<EditorNodeType>([]);
  const [edges, setEdges, onEdgesChange] = useEdgesState<EditorEdge>([]);
  const [selectedNode, setSelectedNode] = useState<string | null>(null);
  const [selectedEdge, setSelectedEdge] = useState<string | null>(null);
  const [formError, setFormError] = useState<string | null>(null);
  const [initialized, setInitialized] = useState(false);
  const counter = useRef(0);

  const flowQuery = useQuery({
    queryKey: ["flow", idOrKey],
    queryFn: () => getFlow(idOrKey),
    enabled: !isNew,
  });

  // Load the definition (or a starter template) exactly once.
  useEffect(() => {
    if (initialized) return;
    if (isNew) {
      const start = makeNode("n1", { ...blankInput("start", 80, 160), key: "start", name: "Start" });
      start.data.input.key = "start";
      start.data.input.name = "Start";
      counter.current = 1;
      setNodes([start]);
      setInitialized(true);
      return;
    }
    const detail = flowQuery.data;
    if (!detail) return;

    const idOfKey = new Map<string, string>();
    const loaded = detail.nodes.map((node, index) => {
      const id = `n${index + 1}`;
      idOfKey.set(node.key, id);
      counter.current = index + 1;
      return makeNode(
        id,
        {
          key: node.key,
          name: node.name,
          kind: node.kind,
          description: node.description,
          system: node.system,
          repository: node.repository,
          team: node.team,
          method: node.method || "POST",
          endpoint: node.endpoint,
          requestSchema: node.requestSchema,
          responseSample: node.responseSample,
          timeoutMinutes: node.timeoutMinutes,
          posX: node.posX,
          posY: node.posY,
        },
        prettyJson(node.requestSchema),
        prettyJson(node.responseSample),
      );
    });

    const loadedEdges = detail.edges
      .map((edge, index) => {
        const source = idOfKey.get(edge.sourceKey);
        const target = idOfKey.get(edge.targetKey);
        if (!source || !target) return null;
        const created = makeEdge(`e${index + 1}`, source, target);
        created.label = edge.label || undefined;
        created.data = { condition: edge.condition };
        return created;
      })
      .filter((edge): edge is EditorEdge => edge !== null);

    setMeta({
      key: detail.key,
      name: detail.name,
      description: detail.description,
      category: detail.category,
      owner: detail.owner,
      version: detail.version,
      active: detail.active,
    });
    setNodes(loaded);
    setEdges(loadedEdges);
    setInitialized(true);
  }, [initialized, isNew, flowQuery.data, setNodes, setEdges]);

  const save = useMutation({
    mutationFn: (input: FlowInput) => (isNew ? createFlow(input) : updateFlow(idOrKey, input)),
    onSuccess: (detail) => {
      queryClient.invalidateQueries({ queryKey: ["flows"] });
      navigate(`/flows/${detail.key}`);
    },
  });

  const remove = useMutation({
    mutationFn: () => deleteFlow(idOrKey),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["flows"] });
      navigate("/flows");
    },
  });

  const onConnect = useCallback(
    (connection: Connection) => {
      setEdges((current) => {
        if (current.some((e) => e.source === connection.source && e.target === connection.target)) {
          return current;
        }
        return addEdge(
          {
            ...connection,
            type: "smoothstep",
            markerEnd: { type: MarkerType.ArrowClosed, color: "#94a3b8" },
            style: { stroke: "#94a3b8", strokeWidth: 1.5 },
            data: { condition: "" },
          },
          current,
        );
      });
    },
    [setEdges],
  );

  const updateNodeInput = useCallback(
    (id: string, patch: Partial<FlowNodeInput>) => {
      setNodes((current) =>
        current.map((node) =>
          node.id === id
            ? { ...node, data: { ...node.data, input: { ...node.data.input, ...patch } } }
            : node,
        ),
      );
    },
    [setNodes],
  );

  const updateNodeText = useCallback(
    (id: string, patch: Partial<Pick<EditorNodeData, "schemaText" | "responseText">>) => {
      setNodes((current) =>
        current.map((node) => (node.id === id ? { ...node, data: { ...node.data, ...patch } } : node)),
      );
    },
    [setNodes],
  );

  const updateEdge = useCallback(
    (id: string, patch: { label?: string; condition?: string }) => {
      setEdges((current) =>
        current.map((edge) =>
          edge.id === id
            ? {
                ...edge,
                label: patch.label !== undefined ? patch.label : edge.label,
                data: { condition: patch.condition ?? edge.data?.condition ?? "" },
              }
            : edge,
        ),
      );
    },
    [setEdges],
  );

  const addNode = useCallback(() => {
    const index = ++counter.current;
    const id = `n${index}`;
    const column = nodes.length % 5;
    const position = { x: 120 + column * 220, y: 380 + Math.floor(nodes.length / 5) * 160 };
    setNodes((current) => [...current, makeNode(id, blankInput("task", position.x, position.y))]);
    setSelectedEdge(null);
    setSelectedNode(id);
  }, [nodes.length, setNodes]);

  const deleteNode = useCallback(
    (id: string) => {
      setNodes((current) => current.filter((node) => node.id !== id));
      setEdges((current) => current.filter((edge) => edge.source !== id && edge.target !== id));
      setSelectedNode(null);
    },
    [setNodes, setEdges],
  );

  const deleteEdge = useCallback(
    (id: string) => {
      setEdges((current) => current.filter((edge) => edge.id !== id));
      setSelectedEdge(null);
    },
    [setEdges],
  );

  const buildInput = (): FlowInput => {
    const keyOfId = new Map(nodes.map((node) => [node.id, node.data.input.key.trim()]));

    const parseJson = (text: string, label: string): unknown => {
      const trimmed = text.trim();
      if (trimmed === "") return null;
      try {
        return JSON.parse(trimmed);
      } catch {
        throw new Error(`${label} is not valid JSON`);
      }
    };

    return {
      ...meta,
      nodes: nodes.map((node) => ({
        ...node.data.input,
        key: node.data.input.key.trim(),
        requestSchema: parseJson(node.data.schemaText, `Parameters of "${node.data.input.key || "node"}"`),
        responseSample: parseJson(node.data.responseText, `Sample response of "${node.data.input.key || "node"}"`),
        posX: Math.round(node.position.x),
        posY: Math.round(node.position.y),
      })),
      edges: edges.map((edge) => ({
        sourceKey: keyOfId.get(edge.source) ?? "",
        targetKey: keyOfId.get(edge.target) ?? "",
        label: typeof edge.label === "string" ? edge.label : "",
        condition: edge.data?.condition ?? "",
      })),
    };
  };

  const validate = (input: FlowInput): string | null => {
    if (!input.key.trim()) return "Flow key is required.";
    if (!input.name.trim()) return "Flow name is required.";
    if (input.nodes.length === 0) return "A flow needs at least one node.";
    const seen = new Set<string>();
    for (const node of input.nodes) {
      if (!node.key) return "Every node needs a key.";
      if (seen.has(node.key)) return `Duplicate node key "${node.key}".`;
      seen.add(node.key);
    }
    for (const edge of input.edges) {
      if (!seen.has(edge.sourceKey) || !seen.has(edge.targetKey)) {
        return "A connection references a missing node.";
      }
    }
    return null;
  };

  const onSave = () => {
    setFormError(null);
    try {
      const input = buildInput();
      const problem = validate(input);
      if (problem) {
        setFormError(problem);
        return;
      }
      save.mutate(input);
    } catch (err) {
      setFormError(errorMessage(err));
    }
  };

  const selected = nodes.find((node) => node.id === selectedNode) ?? null;
  const selectedEdgeObj = edges.find((edge) => edge.id === selectedEdge) ?? null;

  if (!isNew && flowQuery.isLoading) {
    return <Spinner label="Loading flow…" />;
  }
  if (!isNew && flowQuery.isError) {
    return <ErrorState message={errorMessage(flowQuery.error)} onRetry={() => flowQuery.refetch()} />;
  }

  return (
    <div className="flex h-full flex-col">
      <PageHeader
        title={isNew ? "New flow" : `Edit: ${meta.name || meta.key}`}
        subtitle="Drag to arrange · connect handles to add steps · click a node to edit its contract"
        actions={
          <>
            <Link to={isNew ? "/flows" : `/flows/${meta.key}`}>
              <Button variant="secondary" size="sm">
                <ArrowLeft className="size-3.5" />
                {isNew ? "Cancel" : "Monitor"}
              </Button>
            </Link>
            {!isNew ? (
              <Button
                variant="danger"
                size="sm"
                onClick={() => {
                  if (window.confirm(`Delete flow "${meta.name}" and all its telemetry?`)) {
                    remove.mutate();
                  }
                }}
                disabled={remove.isPending}
              >
                <Trash2 className="size-3.5" />
                Delete
              </Button>
            ) : null}
            <Button size="sm" onClick={onSave} disabled={save.isPending}>
              <Save className="size-3.5" />
              {save.isPending ? "Saving…" : "Save"}
            </Button>
          </>
        }
      />

      {formError || save.isError || remove.isError ? (
        <div className="flex items-center gap-2 border-b border-rose-200 bg-rose-50 px-6 py-2 text-sm text-rose-700">
          <TriangleAlert className="size-4" />
          {formError ?? errorMessage(save.error ?? remove.error)}
        </div>
      ) : null}

      <div className="flex min-h-0 flex-1">
        <div className="relative min-w-0 flex-1">
          <ReactFlow
            nodes={nodes}
            edges={edges}
            nodeTypes={nodeTypes}
            onNodesChange={onNodesChange}
            onEdgesChange={onEdgesChange}
            onConnect={onConnect}
            onNodeClick={(_event, node) => {
              setSelectedEdge(null);
              setSelectedNode(node.id);
            }}
            onEdgeClick={(_event, edge) => {
              setSelectedNode(null);
              setSelectedEdge(edge.id);
            }}
            onPaneClick={() => {
              setSelectedNode(null);
              setSelectedEdge(null);
            }}
            fitView
            proOptions={{ hideAttribution: true }}
          >
            <Background gap={18} color="#e2e8f0" />
            <Controls showInteractive={false} />
            <MiniMap pannable zoomable className="!bg-slate-100" nodeColor="#c7d2fe" />
          </ReactFlow>

          <button
            type="button"
            onClick={addNode}
            className="absolute left-3 top-3 z-10 inline-flex items-center gap-1.5 rounded-md border border-slate-300 bg-white px-2.5 py-1.5 text-xs font-medium text-slate-700 shadow-sm hover:bg-slate-50"
          >
            <Plus className="size-3.5" />
            Add node
          </button>
        </div>

        <aside className="w-80 shrink-0 overflow-auto border-l border-slate-200 bg-white">
          {selected ? (
            <NodeForm
              node={selected}
              onInput={(patch) => updateNodeInput(selected.id, patch)}
              onText={(patch) => updateNodeText(selected.id, patch)}
              onDelete={() => deleteNode(selected.id)}
              onBack={() => setSelectedNode(null)}
            />
          ) : selectedEdgeObj ? (
            <EdgeForm
              edge={selectedEdgeObj}
              onChange={(patch) => updateEdge(selectedEdgeObj.id, patch)}
              onDelete={() => deleteEdge(selectedEdgeObj.id)}
              onBack={() => setSelectedEdge(null)}
            />
          ) : (
            <FlowSettings meta={meta} onChange={setMeta} nodeCount={nodes.length} edgeCount={edges.length} />
          )}
        </aside>
      </div>
    </div>
  );
}

// ---------------------------------------------------------------------------
// panels
// ---------------------------------------------------------------------------

function PanelHeader({ title, onBack }: { title: string; onBack: () => void }) {
  return (
    <div className="flex items-center justify-between border-b border-slate-100 px-4 py-3">
      <span className="text-sm font-semibold text-slate-800">{title}</span>
      <button type="button" onClick={onBack} className="text-xs text-slate-500 hover:text-slate-800">
        ← flow settings
      </button>
    </div>
  );
}

function FlowSettings({
  meta,
  onChange,
  nodeCount,
  edgeCount,
}: {
  meta: Meta;
  onChange: (meta: Meta) => void;
  nodeCount: number;
  edgeCount: number;
}) {
  const set = (patch: Partial<Meta>) => onChange({ ...meta, ...patch });
  return (
    <div className="space-y-3 p-4">
      <h2 className="text-sm font-semibold text-slate-800">Flow settings</h2>
      <Field label="Key" hint="used by instrumenting systems (flowKey)">
        <TextInput value={meta.key} onChange={(value) => set({ key: value })} placeholder="order-fulfillment" />
      </Field>
      <Field label="Name">
        <TextInput value={meta.name} onChange={(value) => set({ name: value })} placeholder="Order Fulfillment" />
      </Field>
      <Field label="Description">
        <TextArea value={meta.description} onChange={(value) => set({ description: value })} rows={3} />
      </Field>
      <div className="grid grid-cols-2 gap-2">
        <Field label="Category">
          <TextInput value={meta.category} onChange={(value) => set({ category: value })} />
        </Field>
        <Field label="Owner">
          <TextInput value={meta.owner} onChange={(value) => set({ owner: value })} />
        </Field>
      </div>
      <div className="grid grid-cols-2 gap-2">
        <Field label="Version">
          <TextInput value={meta.version} onChange={(value) => set({ version: value })} />
        </Field>
        <Field label="Active">
          <Select
            value={meta.active ? "yes" : "no"}
            onChange={(value) => set({ active: value === "yes" })}
            options={[
              { value: "yes", label: "active" },
              { value: "no", label: "inactive" },
            ]}
          />
        </Field>
      </div>
      <p className="border-t border-slate-100 pt-3 text-xs text-slate-400">
        {nodeCount} node(s), {edgeCount} connection(s). Click a node or a connection to edit it.
      </p>
    </div>
  );
}

function NodeForm({
  node,
  onInput,
  onText,
  onDelete,
  onBack,
}: {
  node: EditorNodeType;
  onInput: (patch: Partial<FlowNodeInput>) => void;
  onText: (patch: Partial<Pick<EditorNodeData, "schemaText" | "responseText">>) => void;
  onDelete: () => void;
  onBack: () => void;
}) {
  const input = node.data.input;
  return (
    <div>
      <PanelHeader title="Process step" onBack={onBack} />
      <div className="space-y-3 p-4">
        <div className="grid grid-cols-2 gap-2">
          <Field label="Key" hint="nodeKey">
            <TextInput value={input.key} onChange={(value) => onInput({ key: value })} mono />
          </Field>
          <Field label="Kind">
            <Select value={input.kind} onChange={(value) => onInput({ kind: value })} options={kindOptions} />
          </Field>
        </div>
        <Field label="Name">
          <TextInput value={input.name} onChange={(value) => onInput({ name: value })} />
        </Field>
        <Field label="Description">
          <TextArea value={input.description} onChange={(value) => onInput({ description: value })} rows={2} />
        </Field>
        <Field label="System" hint="which system / service runs this step">
          <TextInput value={input.system} onChange={(value) => onInput({ system: value })} />
        </Field>
        <Field label="Repository">
          <TextInput value={input.repository} onChange={(value) => onInput({ repository: value })} />
        </Field>
        <Field label="Team">
          <TextInput value={input.team} onChange={(value) => onInput({ team: value })} />
        </Field>
        <div className="grid grid-cols-3 gap-2">
          <Field label="Method" className="col-span-1">
            <Select value={input.method} onChange={(value) => onInput({ method: value })} options={methodOptions} />
          </Field>
          <Field label="Endpoint" className="col-span-2">
            <TextInput value={input.endpoint} onChange={(value) => onInput({ endpoint: value })} placeholder="/payments/authorize" />
          </Field>
        </div>
        <Field label="SLA (minutes)" hint="running beyond this makes the task stuck">
          <NumberInput value={input.timeoutMinutes} min={0} onChange={(value) => onInput({ timeoutMinutes: value })} />
        </Field>
        <Field label="Accepted parameters (JSON)">
          <TextArea
            value={node.data.schemaText}
            onChange={(value) => onText({ schemaText: value })}
            mono
            rows={5}
            placeholder='{"orderId": "ORD-1", "amount": 129.9}'
          />
        </Field>
        <Field label="Sample response (JSON)">
          <TextArea
            value={node.data.responseText}
            onChange={(value) => onText({ responseText: value })}
            mono
            rows={5}
            placeholder='{"authorized": true}'
          />
        </Field>
        <Button variant="danger" size="sm" onClick={onDelete} className="w-full">
          <Trash2 className="size-3.5" />
          Delete node
        </Button>
      </div>
    </div>
  );
}

function EdgeForm({
  edge,
  onChange,
  onDelete,
  onBack,
}: {
  edge: EditorEdge;
  onChange: (patch: { label?: string; condition?: string }) => void;
  onDelete: () => void;
  onBack: () => void;
}) {
  return (
    <div>
      <PanelHeader title="Connection" onBack={onBack} />
      <div className="space-y-3 p-4">
        <Field label="Branch label" hint="shown on the diagram, e.g. yes / no / approved">
          <TextInput
            value={typeof edge.label === "string" ? edge.label : ""}
            onChange={(value) => onChange({ label: value })}
          />
        </Field>
        <Field label="Condition" hint="human description of when this branch is taken">
          <TextInput value={edge.data?.condition ?? ""} onChange={(value) => onChange({ condition: value })} />
        </Field>
        <Button variant="danger" size="sm" onClick={onDelete} className="w-full">
          <Trash2 className="size-3.5" />
          Delete connection
        </Button>
      </div>
    </div>
  );
}
