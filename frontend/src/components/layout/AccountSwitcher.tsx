import { useState } from "react";
import { ArrowRightLeft } from "lucide-react";
import { Button } from "@/components/ui/button";
import { DropdownSearch } from "@/components/ui/dropdown";
import { useSwitchable, useSwitchAccount } from "@/features/auth/switchHooks";
import { useAuthStore } from "@/store/authStore";
import { useMe } from "@/features/leads/hooks";

export function AccountSwitcher() {
  const user = useAuthStore((s) => s.user);
  const { data: me } = useMe();
  const { data: switchable } = useSwitchable();
  const switchAccount = useSwitchAccount();
  const [open, setOpen] = useState(false);
  const [query, setQuery] = useState("");

  const count = me?.switchable_count ?? switchable?.length ?? 0;
  if (count === 0 && user?.account_type !== "platform") return null;

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

  return (
    <div className="relative">
      <Button variant="secondary" size="sm" onClick={() => setOpen((o) => !o)}>
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
                disabled={switchAccount.isPending}
                onClick={() => {
                  close();
                  switchAccount.mutate(a.id);
                }}
              >
                <span className="font-medium text-gray-800">{a.name}</span>
                <span className="block text-xs text-gray-400">
                  {a.handler_id} · {a.type}
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
