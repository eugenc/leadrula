import { useState } from "react";
import { useNavigate } from "react-router-dom";
import { api, get } from "@/lib/api";
import { useAuthStore } from "@/store/authStore";
import { queryClient } from "@/lib/queryClient";
import { Button } from "@/components/ui/button";
import { Input, Label } from "@/components/ui/input";
import { Spinner } from "@/components/ui/misc";
import type { Me } from "@/types";

export function LoginPage() {
  const navigate = useNavigate();
  const setAuth = useAuthStore((s) => s.setAuth);
  const setTokens = useAuthStore((s) => s.setTokens);
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [error, setError] = useState("");
  const [loading, setLoading] = useState(false);

  async function submit(e: React.FormEvent) {
    e.preventDefault();
    setError("");
    setLoading(true);
    try {
      const res = await api.post("/auth/login", { email, password });
      const { access, refresh } = res.data.data;
      queryClient.clear(); // drop any cached data from a prior session
      setTokens(access, refresh);
      const me = await get<Me>("/auth/me");
      setAuth(access, refresh, {
        id: me.user.id,
        email: me.user.email,
        full_name: me.user.full_name,
        role: me.user.role,
        account_type: me.account.type,
        account_id: me.account.id,
      });
      navigate(me.account.type === "publisher" ? "/p" : "/b");
    } catch {
      setError("Invalid email or password");
      useAuthStore.getState().logout();
    } finally {
      setLoading(false);
    }
  }

  return (
    <div className="flex min-h-screen items-center justify-center bg-pd-bg">
      <div className="w-full max-w-sm rounded-lg border border-pd-border bg-white p-8 shadow-sm">
        <div className="mb-6 flex items-center gap-2">
          <div className="flex h-9 w-9 items-center justify-center rounded bg-pd-green text-lg font-bold text-white">
            L
          </div>
          <span className="text-xl font-bold">LeadRula</span>
        </div>
        <h1 className="mb-1 text-lg font-bold">Sign in</h1>
        <p className="mb-5 text-sm text-pd-muted">Welcome back. Enter your credentials.</p>
        <form onSubmit={submit} className="space-y-4">
          <div>
            <Label>Email</Label>
            <Input type="email" value={email} onChange={(e) => setEmail(e.target.value)} required />
          </div>
          <div>
            <Label>Password</Label>
            <Input
              type="password"
              value={password}
              onChange={(e) => setPassword(e.target.value)}
              required
            />
          </div>
          {error && <p className="text-sm text-pd-red">{error}</p>}
          <Button type="submit" className="w-full" disabled={loading}>
            {loading ? <Spinner className="text-white" /> : "Sign in"}
          </Button>
        </form>
      </div>
    </div>
  );
}
