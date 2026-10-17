import type { ReactNode } from "react";
import { cx } from "@/lib/cx";

export interface MetaItem {
  label: string;
  value: ReactNode;
  mono?: boolean;
}

/** Compact key/value grid used across drill-down panels. */
export function MetaGrid({
  items,
  columns = 2,
  className,
}: {
  items: MetaItem[];
  columns?: 1 | 2 | 3;
  className?: string;
}) {
  const colClass = columns === 1 ? "sm:grid-cols-1" : columns === 3 ? "sm:grid-cols-3" : "sm:grid-cols-2";
  return (
    <dl className={cx("grid grid-cols-1 gap-x-6 gap-y-3", colClass, className)}>
      {items.map((item) => (
        <div key={item.label} className="min-w-0">
          <dt className="text-xs font-medium uppercase tracking-wide text-slate-400">{item.label}</dt>
          <dd
            className={cx(
              "mt-0.5 break-words text-sm text-slate-800",
              item.mono ? "font-mono text-xs" : "",
            )}
          >
            {item.value ?? "—"}
          </dd>
        </div>
      ))}
    </dl>
  );
}
