import { Navigate, Outlet } from "react-router-dom";
import { useAuthStore } from "@/store/authStore";
import type { AccountType } from "@/types";

export function RequireAuth() {
  const token = useAuthStore((s) => s.accessToken);
  if (!token) return <Navigate to="/login" replace />;
  return <Outlet />;
}

export function RequireAccountType({ type }: { type: AccountType }) {
  const user = useAuthStore((s) => s.user);
  if (!user) return <Navigate to="/login" replace />;
  if (user.account_type !== type) {
    return <Navigate to={user.account_type === "publisher" ? "/p" : "/b"} replace />;
  }
  return <Outlet />;
}
