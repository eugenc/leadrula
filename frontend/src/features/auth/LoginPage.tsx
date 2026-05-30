import { useState } from "react";
import { Eye, EyeOff } from "lucide-react";
import { useNavigate } from "react-router-dom";
import { api, get } from "@/lib/api";
import { useAuthStore } from "@/store/authStore";
import { queryClient } from "@/lib/queryClient";
import { Button } from "@/components/ui/button";
import { Input, Label } from "@/components/ui/input";
import { Spinner } from "@/components/ui/misc";
import { Logo } from "@/components/layout/Logo";
import type { Me } from "@/types";

export function LoginPage() {
  const navigate = useNavigate();
  const setAuth = useAuthStore((s) => s.setAuth);
  const setTokens = useAuthStore((s) => s.setTokens);
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [showPassword, setShowPassword] = useState(false);
  const [error, setError] = useState("");
  const [loading, setLoading] = useState(false);

  async function submit(e: React.FormEvent) {
    e.preventDefault();
    setError("");
    setLoading(true);
    try {
      const res = await api.post("/auth/login", { email, password });
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
      setError("Invalid email or password");
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
        <h1 className="mb-1 text-lg font-semibold text-gray-800">Sign in</h1>
        <p className="mb-5 text-base text-gray-400">Welcome back. Enter your credentials.</p>
        <form onSubmit={submit} className="space-y-4">
          <div>
            <Label>Email</Label>
            <Input type="email" value={email} onChange={(e) => setEmail(e.target.value)} required />
          </div>
          <div>
            <Label>Password</Label>
            <div className="relative">
              <Input
                type={showPassword ? "text" : "password"}
                value={password}
                onChange={(e) => setPassword(e.target.value)}
                className="pr-10"
                required
              />
              <button
                type="button"
                onClick={() => setShowPassword((v) => !v)}
                className="absolute right-2 top-1/2 -translate-y-1/2 rounded p-1 text-gray-400 hover:text-gray-600"
                aria-label={showPassword ? "Hide password" : "Show password"}
              >
                {showPassword ? <EyeOff className="h-4 w-4" /> : <Eye className="h-4 w-4" />}
              </button>
            </div>
          </div>
          {error && <p className="text-sm text-danger">{error}</p>}
          <Button type="submit" className="w-full" disabled={loading}>
            {loading ? <Spinner className="text-white" /> : "Sign in"}
          </Button>
        </form>
      </div>
    </div>
  );
}
