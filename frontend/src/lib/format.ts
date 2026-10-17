import dayjs from "dayjs";
import relativeTime from "dayjs/plugin/relativeTime";
import utc from "dayjs/plugin/utc";

dayjs.extend(relativeTime);
dayjs.extend(utc);

/** Human-friendly duration from milliseconds. */
export function formatDuration(ms: number | null | undefined): string {
  if (ms === null || ms === undefined) return "—";
  if (ms < 0) return "—";
  if (ms < 1000) return `${ms}ms`;
  const seconds = ms / 1000;
  if (seconds < 60) return `${seconds.toFixed(1)}s`;
  const minutes = Math.floor(seconds / 60);
  if (minutes < 60) return `${minutes}m ${Math.floor(seconds % 60)}s`;
  const hours = Math.floor(minutes / 60);
  if (hours < 24) return `${hours}h ${minutes % 60}m`;
  const days = Math.floor(hours / 24);
  return `${days}d ${hours % 24}h`;
}

/** Compact duration for dense tables (no sub-second precision). */
export function formatDurationShort(ms: number | null | undefined): string {
  if (ms === null || ms === undefined || ms < 0) return "—";
  if (ms < 1000) return `${ms}ms`;
  const seconds = Math.floor(ms / 1000);
  if (seconds < 60) return `${seconds}s`;
  const minutes = Math.floor(seconds / 60);
  if (minutes < 60) return `${minutes}m`;
  const hours = Math.floor(minutes / 60);
  if (hours < 24) return `${hours}h`;
  return `${Math.floor(hours / 24)}d`;
}

export function formatDateTime(iso: string | null | undefined): string {
  if (!iso) return "—";
  return dayjs(iso).format("YYYY-MM-DD HH:mm:ss");
}

export function formatTime(iso: string | null | undefined): string {
  if (!iso) return "—";
  return dayjs(iso).format("HH:mm:ss");
}

export function formatDay(iso: string | null | undefined): string {
  if (!iso) return "—";
  return dayjs(iso).format("MMM D");
}

export function formatHour(iso: string | null | undefined): string {
  if (!iso) return "—";
  return dayjs(iso).format("HH:00");
}

/** "3 minutes ago" / "in 2 hours". */
export function formatRelative(iso: string | null | undefined): string {
  if (!iso) return "—";
  return dayjs(iso).fromNow();
}

/** A fraction in [0,1] rendered as a percentage. */
export function formatPercent(fraction: number | null | undefined, digits = 0): string {
  if (fraction === null || fraction === undefined || Number.isNaN(fraction)) return "—";
  return `${(fraction * 100).toFixed(digits)}%`;
}

export function formatNumber(value: number | null | undefined): string {
  if (value === null || value === undefined) return "—";
  return value.toLocaleString();
}

/** Pretty-print an arbitrary JSON value; returns "" for nullish/empty. */
export function prettyJson(value: unknown): string {
  if (value === null || value === undefined) return "";
  if (typeof value === "string") {
    const trimmed = value.trim();
    if (trimmed === "") return "";
    // raw JSON stored as a string? try to reformat, else show as-is
    try {
      return JSON.stringify(JSON.parse(trimmed), null, 2);
    } catch {
      return trimmed;
    }
  }
  try {
    return JSON.stringify(value, null, 2);
  } catch {
    return String(value);
  }
}

/** True when a JSON value carries something worth rendering. */
export function hasJson(value: unknown): boolean {
  if (value === null || value === undefined) return false;
  if (typeof value === "string") return value.trim() !== "" && value.trim() !== "null";
  if (Array.isArray(value)) return value.length > 0;
  if (typeof value === "object") return Object.keys(value as object).length > 0;
  return true;
}

export function truncate(text: string, max = 60): string {
  if (!text) return "";
  return text.length <= max ? text : `${text.slice(0, max - 1)}…`;
}

/** ms since an ISO timestamp (used to live-update running task elapsed time). */
export function msSince(iso: string | null | undefined): number {
  if (!iso) return 0;
  return Math.max(0, Date.now() - dayjs(iso).valueOf());
}

export { dayjs };
