import { useEffect, type ReactNode } from "react";
import { X } from "lucide-react";
import { cx } from "@/lib/cx";

/** Right-hand side panel used for node / step / run drill-downs. */
export function Drawer({
  open,
  onClose,
  title,
  subtitle,
  children,
  footer,
  widthClass = "max-w-2xl",
}: {
  open: boolean;
  onClose: () => void;
  title: ReactNode;
  subtitle?: ReactNode;
  children: ReactNode;
  footer?: ReactNode;
  widthClass?: string;
}) {
  useEffect(() => {
    if (!open) return;
    const onKey = (event: KeyboardEvent) => {
      if (event.key === "Escape") onClose();
    };
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, [open, onClose]);

  if (!open) return null;

  return (
    <div className="fixed inset-0 z-40 flex justify-end" role="dialog" aria-modal="true">
      <div
        className="absolute inset-0 bg-slate-900/30 backdrop-blur-[1px]"
        onClick={onClose}
        aria-hidden="true"
      />
      <div
        className={cx(
          "relative flex h-full w-full flex-col border-l border-slate-200 bg-white shadow-xl",
          widthClass,
        )}
      >
        <div className="flex items-start justify-between gap-4 border-b border-slate-200 px-4 py-3">
          <div className="min-w-0">
            <div className="truncate text-sm font-semibold text-slate-900">{title}</div>
            {subtitle ? <div className="mt-0.5 text-xs text-slate-500">{subtitle}</div> : null}
          </div>
          <button
            type="button"
            onClick={onClose}
            className="rounded-md p-1 text-slate-400 hover:bg-slate-100 hover:text-slate-700"
            aria-label="Close"
          >
            <X className="size-4" />
          </button>
        </div>
        <div className="min-h-0 flex-1 overflow-auto">{children}</div>
        {footer ? (
          <div className="border-t border-slate-200 px-4 py-3 text-xs text-slate-500">{footer}</div>
        ) : null}
      </div>
    </div>
  );
}
