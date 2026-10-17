import { ChevronLeft, ChevronRight } from "lucide-react";
import { formatNumber } from "@/lib/format";

/** Prev/next pager for the paginated list endpoints. */
export function Pagination({
  page,
  size,
  total,
  onPage,
}: {
  page: number;
  size: number;
  total: number;
  onPage: (page: number) => void;
}) {
  const pages = Math.max(1, Math.ceil(total / Math.max(1, size)));
  const from = total === 0 ? 0 : (page - 1) * size + 1;
  const to = Math.min(total, page * size);

  return (
    <div className="flex items-center justify-between gap-3 px-1 py-3 text-xs text-slate-500">
      <span>
        {formatNumber(from)}–{formatNumber(to)} of {formatNumber(total)}
      </span>
      <div className="flex items-center gap-1">
        <button
          type="button"
          disabled={page <= 1}
          onClick={() => onPage(page - 1)}
          className="inline-flex items-center gap-1 rounded-md border border-slate-300 px-2 py-1 font-medium text-slate-700 enabled:hover:bg-slate-50 disabled:opacity-40"
        >
          <ChevronLeft className="size-3.5" />
          Prev
        </button>
        <span className="px-1 tabular-nums">
          {page} / {pages}
        </span>
        <button
          type="button"
          disabled={page >= pages}
          onClick={() => onPage(page + 1)}
          className="inline-flex items-center gap-1 rounded-md border border-slate-300 px-2 py-1 font-medium text-slate-700 enabled:hover:bg-slate-50 disabled:opacity-40"
        >
          Next
          <ChevronRight className="size-3.5" />
        </button>
      </div>
    </div>
  );
}
