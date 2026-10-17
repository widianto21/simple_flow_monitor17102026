import type { ReactNode } from "react";
import { cx } from "@/lib/cx";

export interface TabItem {
  id: string;
  label: ReactNode;
  badge?: ReactNode;
}

/** Simple controlled tab strip. */
export function Tabs({
  tabs,
  active,
  onChange,
  className,
}: {
  tabs: TabItem[];
  active: string;
  onChange: (id: string) => void;
  className?: string;
}) {
  return (
    <div className={cx("flex items-center gap-1 border-b border-slate-200", className)}>
      {tabs.map((tab) => {
        const isActive = tab.id === active;
        return (
          <button
            key={tab.id}
            type="button"
            onClick={() => onChange(tab.id)}
            className={cx(
              "-mb-px flex items-center gap-1.5 border-b-2 px-3 py-2 text-sm font-medium transition-colors",
              isActive
                ? "border-indigo-500 text-indigo-700"
                : "border-transparent text-slate-500 hover:text-slate-800",
            )}
          >
            {tab.label}
            {tab.badge ? (
              <span className="rounded-full bg-slate-100 px-1.5 text-xs text-slate-600">
                {tab.badge}
              </span>
            ) : null}
          </button>
        );
      })}
    </div>
  );
}
