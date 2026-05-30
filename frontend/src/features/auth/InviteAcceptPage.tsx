import { useState } from "react";
import { Link, useNavigate, useSearchParams } from "react-router-dom";
import { api, get } from "@/lib/api";
import { useAuthStore } from "@/store/authStore";
import { queryClient } from "@/lib/queryClient";
import { Button } from "@/components/ui/button";
import { Input, Label } from "@/components/ui/input";
import { Spinner } from "@/components/ui/misc";
import { Logo } from "@/components/layout/Logo";
import type { Me } from "@/types";

export function InviteAcceptPage() {
  const navigate = useNavigate();
  const [params] = useSearchParams();
  const token = params.get("token") ?? "";
  const setAuth = useAuthStore((s) => s.setAuth);
  const setTokens = useAuthStore((s) => s.setTokens);
  const [fullName, setFullName] = useState("");
  const [password, setPassword] = useState("");
  const [confirm, setConfirm] = useState("");
  const [error, setError] = useState("");
  const [loading, setLoading] = useState(false);

  async function submit(e: React.FormEvent) {
    e.preventDefault();
    setError("");
    if (!token) {
      setError("Invalid invite link");
      return;
    }
    if (password.length < 8) {
      setError("Password must be at least 8 characters");
      return;
    }
    if (password !== confirm) {
      setError("Passwords do not match");
      return;
    }
    setLoading(true);
    try {
      const res = await api.post("/auth/invite/accept", {
        token,
        full_name: fullName.trim(),
        password,
      });
      const { access, refresh } = res.data.data;
      queryClient.clear();
      setTokens(access, refresh);
      const me = await get<Me>("/auth/me");
      setAuth(access, refresh, {
        id: me.user.id,
        email: me.user.email,
        full_name: me.user.full_name,
        role: me.user.role,
        account_type: me.account.type,
        account_id: me.account.id,
        avatar_url: me.user.avatar_url,
      });
      navigate(me.account.type === "publisher" ? "/p" : "/b");
    } catch {
      setError("Invite link is invalid or expired");
      useAuthStore.getState().logout();
    } finally {
      setLoading(false);
    }
  }

  return (
    <div className="flex min-h-screen items-center justify-center bg-gray-50">
      <div className="w-full max-w-sm rounded-lg border border-gray-100 bg-white p-8 shadow-sm">
        <div className="mb-6 flex items-center gap-2.5">
          <Logo className="h-10 w-auto" />
        </div>
        <h1 className="mb-1 text-lg font-semibold text-gray-800">Accept your invite</h1>
        <p className="mb-5 text-base text-gray-400">Create your password to join LeadRula.</p>
        <form onSubmit={submit} className="space-y-4">
          <div>
            <Label>Full Name</Label>
            <Input value={fullName} onChange={(e) => setFullName(e.target.value)} required />
          </div>
          <div>
            <Label>Password</Label>
            <Input
              type="password"
              value={password}
              onChange={(e) => setPassword(e.target.value)}
              required
              minLength={8}
            />
          </div>
          <div>
            <Label>Confirm Password</Label>
            <Input
              type="password"
              value={confirm}
              onChange={(e) => setConfirm(e.target.value)}
              required
              minLength={8}
            />
          </div>
          {error && <p className="text-sm text-danger">{error}</p>}
          <Button type="submit" className="w-full" disabled={loading || !token}>
            {loading ? <Spinner className="text-white" /> : "Create account"}
          </Button>
          <p className="text-center text-sm text-gray-400">
            Already have an account?{" "}
            <Link to="/login" className="text-jade-600 hover:underline">
              Sign in
            </Link>
          </p>
        </form>
      </div>
    </div>
  );
}
