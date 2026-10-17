import { useQuery } from "@tanstack/react-query";
import { Activity, Bell, GitBranch, ListChecks, Plug, Timer } from "lucide-react";
import { NavLink, Outlet } from "react-router-dom";
import { getHealth } from "@/lib/api";
import { cx } from "@/lib/cx";

const nav = [
  { to: "/dashboard", label: "Dashboard", icon: Activity },
  { to: "/flows", label: "Flows", icon: GitBranch },
  { to: "/runs", label: "Executions", icon: ListChecks },
  { to: "/stuck", label: "Stuck tasks", icon: Timer },
  { to: "/alerts", label: "Alerts", icon: Bell },
  { to: "/integrate", label: "Integrate", icon: Plug },
];

export default function AppLayout() {
  const { data: health, isError } = useQuery({
    queryKey: ["health"],
    queryFn: getHealth,
    refetchInterval: 15_000,
  });

  const ok = !isError && health?.status === "ok";

  return (
    <div className="flex h-full">
      <aside className="flex w-56 shrink-0 flex-col border-r border-slate-200 bg-white">
        <div className="flex items-center gap-2 border-b border-slate-200 px-4 py-4">
          <GitBranch className="size-5 text-indigo-600" />
          <div className="leading-tight">
            <div className="text-sm font-semibold tracking-tight text-slate-900">FlowMonitor</div>
            <div className="text-[10px] uppercase tracking-wide text-slate-400">
              business flow telemetry
            </div>
          </div>
        </div>

        <nav className="flex flex-1 flex-col gap-1 p-2">
          {nav.map(({ to, label, icon: Icon }) => (
            <NavLink
              key={to}
              to={to}
              className={({ isActive }) =>
                cx(
                  "flex items-center gap-2 rounded-md px-3 py-2 text-sm transition-colors",
                  isActive
                    ? "bg-indigo-50 font-medium text-indigo-700"
                    : "text-slate-600 hover:bg-slate-100",
                )
              }
            >
              <Icon className="size-4" />
              {label}
            </NavLink>
          ))}
        </nav>

        <div className="border-t border-slate-100 p-3 text-xs">
          <div className="flex items-center gap-2 text-slate-500">
            <span
              className={cx(
                "inline-block size-2 rounded-full",
                isError ? "bg-rose-500" : ok ? "bg-emerald-500" : "bg-amber-400",
              )}
            />
            {isError ? "backend offline" : ok ? "backend online" : "connecting…"}
          </div>
          {health ? (
            <div className="mt-1 text-[10px] text-slate-400">
              ingest auth {health.ingestAuth ? "on" : "off"} · auto-register{" "}
              {health.autoRegister ? "on" : "off"}
            </div>
          ) : null}
        </div>
      </aside>

      <main className="min-w-0 flex-1 overflow-auto bg-slate-50">
        <Outlet />
      </main>
    </div>
  );
}
