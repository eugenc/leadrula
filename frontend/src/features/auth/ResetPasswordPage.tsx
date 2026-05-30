import { useState } from "react";
import { Link, useNavigate, useSearchParams } from "react-router-dom";
import { api } from "@/lib/api";
import { Button } from "@/components/ui/button";
import { Input, Label } from "@/components/ui/input";
import { Spinner } from "@/components/ui/misc";
import { Logo } from "@/components/layout/Logo";
import { toast } from "@/store/toastStore";

export function ResetPasswordPage() {
  const navigate = useNavigate();
  const [params] = useSearchParams();
  const token = params.get("token") ?? "";
  const [password, setPassword] = useState("");
  const [confirm, setConfirm] = useState("");
  const [error, setError] = useState("");
  const [loading, setLoading] = useState(false);

  async function submit(e: React.FormEvent) {
    e.preventDefault();
    setError("");
    if (!token) {
      setError("Invalid reset link");
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
      await api.post("/auth/password-reset/confirm", {
        token,
        new_password: password,
      });
      toast.success("Password updated");
      navigate("/login");
    } catch {
      setError("Reset link is invalid or expired");
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
        <h1 className="mb-0.5 text-base font-semibold text-gray-800">Reset your password</h1>
        <p className="mb-5 text-xs text-gray-400">Choose a new password for your account.</p>
        <form
          onSubmit={submit}
          className="space-y-4 [&_input]:!h-8 [&_input]:!text-sm [&_label]:!mb-1 [&_label]:!text-sm"
        >
          <div>
            <Label>New Password</Label>
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
            {loading ? <Spinner className="text-white" /> : "Update password"}
          </Button>
          <p className="text-center text-sm text-gray-400">
            <Link to="/login" className="text-jade-600 hover:underline">
              Back to sign in
            </Link>
          </p>
        </form>
      </div>
    </div>
  );
}
