import { useState } from "react";
import { Check, Copy } from "lucide-react";
import { hasJson, prettyJson } from "@/lib/format";
import { cx } from "@/lib/cx";

/** Pretty-printed JSON with a copy button. Renders a muted placeholder when empty. */
export function JsonBlock({
  value,
  label,
  emptyLabel = "no payload",
  className,
  maxHeight = "22rem",
}: {
  value: unknown;
  label?: string;
  emptyLabel?: string;
  className?: string;
  maxHeight?: string;
}) {
  const [copied, setCopied] = useState(false);
  const present = hasJson(value);
  const text = present ? prettyJson(value) : "";

  const copy = async () => {
    try {
      await navigator.clipboard.writeText(text);
      setCopied(true);
      window.setTimeout(() => setCopied(false), 1500);
    } catch {
      /* clipboard may be unavailable; ignore */
    }
  };

  return (
    <div className={cx("overflow-hidden rounded-lg border border-slate-200", className)}>
      {label ? (
        <div className="flex items-center justify-between border-b border-slate-100 bg-slate-50 px-3 py-1.5">
          <span className="text-xs font-medium text-slate-600">{label}</span>
          {present ? (
            <button
              type="button"
              onClick={copy}
              className="inline-flex items-center gap-1 text-xs text-slate-500 hover:text-slate-800"
            >
              {copied ? <Check className="size-3" /> : <Copy className="size-3" />}
              {copied ? "copied" : "copy"}
            </button>
          ) : null}
        </div>
      ) : null}
      {present ? (
        <pre
          className="overflow-auto bg-slate-900 px-3 py-2 text-xs leading-relaxed text-slate-100"
          style={{ maxHeight }}
        >
          {text}
        </pre>
      ) : (
        <div className="bg-slate-50 px-3 py-2 text-xs italic text-slate-400">{emptyLabel}</div>
      )}
    </div>
  );
}
