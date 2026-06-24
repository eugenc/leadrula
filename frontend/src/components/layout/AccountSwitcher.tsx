import { useState } from "react";
import { useNavigate } from "react-router-dom";
import { ArrowRightLeft } from "lucide-react";
import { Button } from "@/components/ui/button";
import { DropdownSearch } from "@/components/ui/dropdown";
import { useSwitchable, useSwitchAccount } from "@/features/auth/switchHooks";
import { useImpersonateBuyer } from "@/features/admin/hooks";
import { useAuthStore } from "@/store/authStore";
import { useMe } from "@/features/leads/hooks";
import { errorMessage } from "@/lib/api";
import { toast } from "@/store/toastStore";
import { Avatar } from "@/components/ui/misc";
import type { CurrentUser } from "@/types";

export function AccountSwitcher({ compact = false }: { compact?: boolean }) {
  const user = useAuthStore((s) => s.user);
  const impersonation = useAuthStore((s) => s.impersonation);
  const startImpersonation = useAuthStore((s) => s.startImpersonation);
  const { data: me } = useMe();
  const { data: switchable } = useSwitchable();
  const switchAccount = useSwitchAccount();
  const impersonate = useImpersonateBuyer();
  const navigate = useNavigate();
  const [open, setOpen] = useState(false);
  const [query, setQuery] = useState("");

  const count = me?.switchable_count ?? switchable?.length ?? 0;
  if (impersonation) return null;
  if (count === 0 && user?.account_type !== "platform") return null;

  const pending = switchAccount.isPending || impersonate.isPending;
  const q = query.trim().toLowerCase();
  const filtered = (switchable ?? []).filter(
    (a) =>
      !q ||
      a.name.toLowerCase().includes(q) ||
      a.handler_id.toLowerCase().includes(q)
  );

  function close() {
    setQuery("");
    setOpen(false);
  }

  function selectAccount(account: (typeof filtered)[number]) {
    close();
    if (account.access_via === "impersonate") {
      impersonate.mutate(account.id, {
        onSuccess: (res) => {
          const u = res.user as unknown as CurrentUser & { buyer_account_name?: string };
          startImpersonation(res.access, u, u.buyer_account_name ?? account.name);
          navigate("/b");
        },
        onError: (e) => toast.error(errorMessage(e)),
      });
      return;
    }
    switchAccount.mutate(account.id);
  }

  return (
    <div className="relative">
      {compact ? (
        <button
          type="button"
          onClick={() => setOpen((o) => !o)}
          className="flex h-9 w-9 items-center justify-center rounded-full hover:bg-gray-100 lg:hidden"
          aria-label="Switch account"
        >
          <Avatar name={user?.full_name ?? "?"} src={user?.avatar_url} className="h-8 w-8" />
        </button>
      ) : null}
      <Button
        variant="secondary"
        size="sm"
        onClick={() => setOpen((o) => !o)}
        className={compact ? "hidden lg:inline-flex" : undefined}
      >
        <ArrowRightLeft className="h-3.5 w-3.5" /> Switch account
      </Button>
      {open ? (
        <div className="absolute right-0 top-full z-50 mt-1 max-h-80 w-72 overflow-y-auto rounded-md border border-gray-100 bg-surface-card py-1 shadow-lg">
          <div className="sticky top-0 z-10 bg-surface-card">
            <DropdownSearch
              value={query}
              onChange={setQuery}
              placeholder="Search by name or ID…"
              autoFocus
            />
          </div>
          {filtered.length === 0 ? (
            <p className="px-3 py-4 text-center text-sm text-gray-400">No matching accounts</p>
          ) : (
            filtered.map((a) => (
              <button
                key={a.id}
                type="button"
                className="block w-full px-3 py-2 text-left text-sm hover:bg-gray-100"
                disabled={pending}
                onClick={() => selectAccount(a)}
              >
                <span className="font-medium text-gray-800">{a.name}</span>
                <span className="block text-xs text-gray-400">
                  {a.handler_id} · {a.type}
                  {a.access_via === "impersonate" ? " · Collaboration" : ""}
                </span>
              </button>
            ))
          )}
          {user?.account_type === "platform" ? (
            <p className="border-t border-gray-50 px-3 py-2 text-xs text-gray-400">
              Or open the platform home to create publishers.
            </p>
          ) : null}
        </div>
      ) : null}
    </div>
  );
}
