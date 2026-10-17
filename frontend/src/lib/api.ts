import axios, { AxiosError } from "axios";
import type {
  Alert,
  BatchResult,
  Dashboard,
  FlowDetail,
  FlowInput,
  FlowSummary,
  Health,
  IngestResult,
  ListResponse,
  Manifest,
  NodeDetail,
  NodeStats,
  RunDetail,
  RunSummary,
  SimStats,
  SliceResponse,
  StepDetail,
  StepRun,
} from "./types";

/** Shared axios instance — `/api/*` is proxied to the Go backend in dev. */
export const api = axios.create({
  baseURL: "/api",
  timeout: 15_000,
});

/** Turn an axios/HTTP error into a human-readable message. */
export function errorMessage(err: unknown): string {
  if (axios.isAxiosError(err)) {
    const axiosErr = err as AxiosError<{ error?: string }>;
    const body = axiosErr.response?.data;
    if (body && typeof body === "object" && "error" in body && body.error) {
      return body.error;
    }
    if (axiosErr.response) {
      return `${axiosErr.response.status} ${axiosErr.response.statusText}`;
    }
    return axiosErr.message;
  }
  if (err instanceof Error) return err.message;
  return String(err);
}

/** Drop empty/undefined query params so the backend sees clean filters. */
function clean(params?: object): Record<string, unknown> {
  const out: Record<string, unknown> = {};
  if (!params) return out;
  for (const [key, value] of Object.entries(params)) {
    if (value === undefined || value === null || value === "") continue;
    if (value === false) continue;
    out[key] = value;
  }
  return out;
}

// ---------------------------------------------------------------------------
// health & dashboard
// ---------------------------------------------------------------------------

export async function getHealth(): Promise<Health> {
  const { data } = await api.get<Health>("/health");
  return data;
}

export async function getDashboard(): Promise<Dashboard> {
  const { data } = await api.get<Dashboard>("/dashboard");
  return data;
}

export async function runDemoSeed(): Promise<SimStats> {
  const { data } = await api.post<SimStats>("/demo/seed", {});
  return data;
}

// ---------------------------------------------------------------------------
// flows
// ---------------------------------------------------------------------------

export async function listFlows(): Promise<FlowSummary[]> {
  const { data } = await api.get<FlowSummary[]>("/flows");
  return data;
}

export async function getFlow(idOrKey: string | number): Promise<FlowDetail> {
  const { data } = await api.get<FlowDetail>(`/flows/${encodeURIComponent(String(idOrKey))}`);
  return data;
}

export async function createFlow(input: FlowInput): Promise<FlowDetail> {
  const { data } = await api.post<FlowDetail>("/flows", input);
  return data;
}

export async function updateFlow(idOrKey: string | number, input: FlowInput): Promise<FlowDetail> {
  const { data } = await api.put<FlowDetail>(`/flows/${encodeURIComponent(String(idOrKey))}`, input);
  return data;
}

export async function deleteFlow(idOrKey: string | number): Promise<void> {
  await api.delete(`/flows/${encodeURIComponent(String(idOrKey))}`);
}

export async function getFlowStats(idOrKey: string | number): Promise<NodeStats[]> {
  const { data } = await api.get<NodeStats[]>(`/flows/${encodeURIComponent(String(idOrKey))}/stats`);
  return data;
}

export async function getNodeDetail(
  idOrKey: string | number,
  nodeKey: string,
  limit = 25,
): Promise<NodeDetail> {
  const { data } = await api.get<NodeDetail>(
    `/flows/${encodeURIComponent(String(idOrKey))}/nodes/${encodeURIComponent(nodeKey)}`,
    { params: { limit } },
  );
  return data;
}

export async function getManifest(flowKey: string): Promise<Manifest> {
  const { data } = await api.get<Manifest>(`/ingest/manifests/${encodeURIComponent(flowKey)}`);
  return data;
}

// ---------------------------------------------------------------------------
// runs, tasks, alerts
// ---------------------------------------------------------------------------

export interface RunQuery {
  flowKey?: string;
  status?: string;
  businessKey?: string;
  nodeKey?: string;
  from?: string;
  to?: string;
  page?: number;
  size?: number;
}

export async function listRuns(query: RunQuery = {}): Promise<ListResponse<RunSummary>> {
  const { data } = await api.get<ListResponse<RunSummary>>("/runs", { params: clean(query) });
  return data;
}

export async function getRun(id: number | string): Promise<RunDetail> {
  const { data } = await api.get<RunDetail>(`/runs/${id}`);
  return data;
}

export async function getStep(runId: number | string, stepId: number | string): Promise<StepDetail> {
  const { data } = await api.get<StepDetail>(`/runs/${runId}/steps/${stepId}`);
  return data;
}

export interface StepQuery {
  flowKey?: string;
  nodeKey?: string;
  status?: string;
  businessKey?: string;
  stuck?: boolean;
  from?: string;
  to?: string;
  page?: number;
  size?: number;
}

export async function listSteps(query: StepQuery = {}): Promise<ListResponse<StepRun>> {
  const { data } = await api.get<ListResponse<StepRun>>("/steps", { params: clean(query) });
  return data;
}

export async function listStuck(limit = 50): Promise<SliceResponse<StepRun>> {
  const { data } = await api.get<SliceResponse<StepRun>>("/tasks/stuck", { params: { limit } });
  return data;
}

export async function businessKeyHistory(key: string, limit = 20): Promise<RunSummary[]> {
  const { data } = await api.get<RunSummary[]>(`/business-keys/${encodeURIComponent(key)}/runs`, {
    params: { limit },
  });
  return data;
}

export async function listAlerts(status = "open", limit = 100): Promise<SliceResponse<Alert>> {
  const { data } = await api.get<SliceResponse<Alert>>("/alerts", { params: { status, limit } });
  return data;
}

export async function ackAlert(id: number): Promise<Alert> {
  const { data } = await api.post<Alert>(`/alerts/${id}/ack`);
  return data;
}

// ---------------------------------------------------------------------------
// ingestion (used by the /integrate playground)
// ---------------------------------------------------------------------------

export async function postIngest(
  body: unknown,
  apiKey?: string,
): Promise<IngestResult | BatchResult> {
  const headers = apiKey ? { "X-API-Key": apiKey } : undefined;
  const endpoint = Array.isArray(body) ? "/ingest/events/batch" : "/ingest/events";
  const { data } = await api.post<IngestResult | BatchResult>(endpoint, body, { headers });
  return data;
}
