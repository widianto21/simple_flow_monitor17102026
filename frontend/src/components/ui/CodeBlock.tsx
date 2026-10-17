import { useState } from "react";
import { Check, Copy } from "lucide-react";
import { cx } from "@/lib/cx";

/** Read-only code block with a copy button. */
export function CodeBlock({
  code,
  language,
  className,
  maxHeight = "24rem",
}: {
  code: string;
  language?: string;
  className?: string;
  maxHeight?: string;
}) {
  const [copied, setCopied] = useState(false);

  const copy = async () => {
    try {
      await navigator.clipboard.writeText(code);
      setCopied(true);
      window.setTimeout(() => setCopied(false), 1500);
    } catch {
      /* ignore */
    }
  };

  return (
    <div className={cx("overflow-hidden rounded-lg border border-slate-800", className)}>
      <div className="flex items-center justify-between border-b border-slate-700 bg-slate-800 px-3 py-1.5">
        <span className="text-xs font-medium text-slate-300">{language ?? "code"}</span>
        <button
          type="button"
          onClick={copy}
          className="inline-flex items-center gap-1 text-xs text-slate-400 hover:text-white"
        >
          {copied ? <Check className="size-3" /> : <Copy className="size-3" />}
          {copied ? "copied" : "copy"}
        </button>
      </div>
      <pre
        className="overflow-auto bg-slate-900 px-3 py-2 text-xs leading-relaxed text-slate-100"
        style={{ maxHeight }}
      >
        {code}
      </pre>
    </div>
  );
}
