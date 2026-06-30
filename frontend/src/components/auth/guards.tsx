import { Navigate, Outlet, useLocation } from "react-router-dom";
import { useAuthStore } from "@/store/authStore";
import { homePath } from "@/lib/homePath";
import { canNav } from "@/lib/permissions";
import type { AccountType } from "@/types";

const routeNavKey: Record<string, string> = {
  fields: "fields",
  appointments: "appointments",
  calendar: "calendars",
  calls: "calls",
  board: "board",
  pipelines: "pipelines",
  buyers: "buyers",
  publishers: "publishers",
  contracts: "contracts",
  contract: "contracts",
  collaboration: "collaboration",
  sources: "sources",
  webhooks: "webhooks",
  routing: "routing",
  log: "logs",
  logs: "logs",
  routes: "routes",
  billing: "billing",
  integrations: "integrations",
  users: "settings",
  business: "settings",
  api: "settings",
  "api-docs": "settings",
  notifications: "settings",
};

export function RequireAuth() {
  const token = useAuthStore((s) => s.accessToken);
  const location = useLocation();
  if (!token) {
    const next = encodeURIComponent(location.pathname + location.search);
    return <Navigate to={`/login?next=${next}`} replace />;
  }
  return <Outlet />;
}

export function RequireAccountType({ type }: { type: AccountType }) {
  const user = useAuthStore((s) => s.user);
  if (!user) return <Navigate to="/login" replace />;
  if (user.account_type !== type) {
    return <Navigate to={homePath(user.account_type)} replace />;
  }
  return <Outlet />;
}

export function RequireNavAccess() {
  const user = useAuthStore((s) => s.user);
  const { pathname } = useLocation();
  if (!user) return <Navigate to="/login" replace />;

  if (user.account_type === "platform") return <Outlet />;

  const parts = pathname.split("/").filter(Boolean);
  const seg = parts[parts.length - 1] ?? "";
  if (parts.length <= 1 || seg === "settings") return <Outlet />;

  const navKey = routeNavKey[seg];
  if (navKey && !canNav(user, navKey)) {
    return <Navigate to={homePath(user.account_type)} replace />;
  }
  return <Outlet />;
}
