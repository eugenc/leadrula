import { Navigate, Outlet, useLocation } from "react-router-dom";
import { useAuthStore } from "@/store/authStore";
import { homePath } from "@/lib/homePath";
import type { AccountType } from "@/types";

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
