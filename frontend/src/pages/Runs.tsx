import { useMemo, useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { RefreshCw, Search, X } from "lucide-react";
import { errorMessage, listFlows, listRuns } from "@/lib/api";
import { Button, Select, TextInput } from "@/components/ui/Form";
import { Card, EmptyState, ErrorState, Spinner } from "@/components/ui/primitives";
import { PageBody, PageHeader } from "@/components/ui/PageHeader";
import { RunTable } from "@/components/RunTable";
import { Pagination } from "@/components/ui/Pagination";

const statusOptions = [
  { value: "all", label: "All statuses" },
  { value: "running", label: "Running" },
  { value: "completed", label: "Completed" },
  { value: "failed", label: "Failed" },
  { value: "cancelled", label: "Cancelled" },
  { value: "stuck", label: "Stuck" },
];

export default function Runs() {
  const [flowKey, setFlowKey] = useState("all");
  const [status, setStatus] = useState("all");
  const [businessKey, setBusinessKey] = useState("");
  const [nodeKey, setNodeKey] = useState("");
  const [page, setPage] = useState(1);
  const size = 20;

  const flowsQuery = useQuery({ queryKey: ["flows"], queryFn: listFlows });

  const query = useQuery({
    queryKey: ["runs", flowKey, status, businessKey, nodeKey, page],
    queryFn: () =>
      listRuns({
        flowKey: flowKey === "all" ? undefined : flowKey,
        status: status === "all" ? undefined : status,
        businessKey: businessKey.trim() || undefined,
        nodeKey: nodeKey.trim() || undefined,
        page,
        size,
      }),
    refetchInterval: 5_000,
  });

  const flowOptions = useMemo(
    () => [
      { value: "all", label: "All flows" },
      ...(flowsQuery.data ?? []).map((flow) => ({ value: flow.key, label: flow.name || flow.key })),
    ],
    [flowsQuery.data],
  );

  const reset = () => {
    setFlowKey("all");
    setStatus("all");
    setBusinessKey("");
    setNodeKey("");
    setPage(1);
  };

  const filtered = flowKey !== "all" || status !== "all" || businessKey !== "" || nodeKey !== "";

  return (
    <div className="flex h-full flex-col">
      <PageHeader
        title="Executions"
        subtitle="Every run of every flow, filterable by status, flow and correlation id"
        actions={
          <Button variant="secondary" size="sm" onClick={() => query.refetch()} disabled={query.isFetching}>
            <RefreshCw className={`size-3.5 ${query.isFetching ? "animate-spin" : ""}`} />
            Refresh
          </Button>
        }
      />

      <PageBody>
        <Card className="p-3">
          <div className="grid grid-cols-1 gap-2 sm:grid-cols-2 lg:grid-cols-4">
            <Select
              value={flowKey}
              onChange={(value) => {
                setFlowKey(value);
                setPage(1);
              }}
              options={flowOptions}
            />
            <Select
              value={status}
              onChange={(value) => {
                setStatus(value);
                setPage(1);
              }}
              options={statusOptions}
            />
            <TextInput
              value={businessKey}
              onChange={(value) => {
                setBusinessKey(value);
                setPage(1);
              }}
              placeholder="business key (order id, claim id…)"
            />
            <div className="flex gap-2">
              <TextInput
                value={nodeKey}
                onChange={(value) => {
                  setNodeKey(value);
                  setPage(1);
                }}
                placeholder="node key"
              />
              {filtered ? (
                <button
                  type="button"
                  onClick={reset}
                  className="inline-flex shrink-0 items-center gap-1 rounded-md border border-slate-300 px-2 text-xs font-medium text-slate-600 hover:bg-slate-50"
                >
                  <X className="size-3.5" />
                  Clear
                </button>
              ) : null}
            </div>
          </div>
        </Card>

        <Card>
          {query.isLoading ? <Spinner label="Loading executions…" /> : null}
          {query.isError ? (
            <ErrorState message={errorMessage(query.error)} onRetry={() => query.refetch()} />
          ) : null}
          {query.data ? (
            <>
              {query.data.total === 0 ? (
                <EmptyState
                  title="No executions match these filters"
                  hint={
                    <span className="inline-flex items-center gap-1">
                      <Search className="size-3.5" />
                      Try clearing the filters or widening the search.
                    </span>
                  }
                />
              ) : (
                <RunTable runs={query.data.items} />
              )}
              <div className="px-3">
                <Pagination page={query.data.page} size={query.data.size} total={query.data.total} onPage={setPage} />
              </div>
            </>
          ) : null}
        </Card>
      </PageBody>
    </div>
  );
}
