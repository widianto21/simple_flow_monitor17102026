import type { ReactNode } from "react";
import { AlertTriangle, CircleCheck, CircleDashed, CircleSlash, Clock, XCircle } from "lucide-react";
import { cx } from "@/lib/cx";
import type { AlertSeverity, NodeKind, RunStatus, StepStatus } from "@/lib/types";

// ---------------------------------------------------------------------------
// Badge
// ---------------------------------------------------------------------------

export type Tone =
  | "slate"
  | "indigo"
  | "emerald"
  | "amber"
  | "rose"
  | "violet"
  | "cyan"
  | "sky";

const toneClasses: Record<Tone, string> = {
  slate: "bg-slate-100 text-slate-700 ring-slate-200",
  indigo: "bg-indigo-50 text-indigo-700 ring-indigo-200",
  emerald: "bg-emerald-50 text-emerald-700 ring-emerald-200",
  amber: "bg-amber-50 text-amber-700 ring-amber-200",
  rose: "bg-rose-50 text-rose-700 ring-rose-200",
  violet: "bg-violet-50 text-violet-700 ring-violet-200",
  cyan: "bg-cyan-50 text-cyan-700 ring-cyan-200",
  sky: "bg-sky-50 text-sky-700 ring-sky-200",
};

export function Badge({
  children,
  tone = "slate",
  className,
  icon,
}: {
  children: ReactNode;
  tone?: Tone;
  className?: string;
  icon?: ReactNode;
}) {
  return (
    <span
      className={cx(
        "inline-flex items-center gap-1 rounded-full px-2 py-0.5 text-xs font-medium ring-1 ring-inset",
        toneClasses[tone],
        className,
      )}
    >
      {icon}
      {children}
    </span>
  );
}

// ---------------------------------------------------------------------------
// status
// ---------------------------------------------------------------------------

const runTone: Record<string, Tone> = {
  running: "indigo",
  completed: "emerald",
  failed: "rose",
  cancelled: "slate",
};

const runIcon: Record<string, ReactNode> = {
  running: <CircleDashed className="size-3" />,
  completed: <CircleCheck className="size-3" />,
  failed: <XCircle className="size-3" />,
  cancelled: <CircleSlash className="size-3" />,
};

export function StatusBadge({ status }: { status: RunStatus | StepStatus | string }) {
  const tone = runTone[status] ?? "slate";
  return (
    <Badge tone={tone} icon={runIcon[status]}>
      {status}
    </Badge>
  );
}

export function StuckBadge({ overdueMs, compact }: { overdueMs?: number; compact?: boolean }) {
  return (
    <Badge tone="amber" icon={<Clock className="size-3" />}>
      {compact ? "stuck" : overdueMs ? `stuck · +${Math.round(overdueMs / 60000)}m` : "stuck"}
    </Badge>
  );
}

// ---------------------------------------------------------------------------
// node kind
// ---------------------------------------------------------------------------

const kindTone: Record<NodeKind, Tone> = {
  start: "emerald",
  task: "sky",
  decision: "amber",
  gateway: "violet",
  external: "cyan",
  end: "rose",
};

export function KindBadge({ kind }: { kind: NodeKind | string }) {
  const tone = kindTone[kind as NodeKind] ?? "slate";
  return <Badge tone={tone}>{kind}</Badge>;
}

// ---------------------------------------------------------------------------
// alert severity / type
// ---------------------------------------------------------------------------

const severityTone: Record<AlertSeverity, Tone> = {
  info: "slate",
  warning: "amber",
  critical: "rose",
};

export function SeverityBadge({ severity }: { severity: AlertSeverity | string }) {
  const tone = severityTone[severity as AlertSeverity] ?? "slate";
  return (
    <Badge tone={tone} icon={severity === "critical" ? <AlertTriangle className="size-3" /> : undefined}>
      {severity}
    </Badge>
  );
}

export function AlertTypeBadge({ type }: { type: string }) {
  return <Badge tone={type === "failure" ? "rose" : "amber"}>{type}</Badge>;
}
