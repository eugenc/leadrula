import { useState } from "react";
import { useNavigate } from "react-router-dom";
import { Bell, LogOut } from "lucide-react";
import { useAuthStore } from "@/store/authStore";
import { Avatar } from "@/components/ui/misc";
import { useNotifications, useMarkRead } from "@/hooks/useNotifications";
import { queryClient } from "@/lib/queryClient";
import { cn } from "@/lib/utils";
import { formatDistanceToNow } from "date-fns";

const labels: Record<string, string> = {
  new_lead: "New lead received",
  lead_returned: "A lead was returned",
  dispute_update: "Dispute resolved",
};

export function Topbar({ title }: { title: string }) {
  const user = useAuthStore((s) => s.user);
  const logout = useAuthStore((s) => s.logout);
  const navigate = useNavigate();
  const [open, setOpen] = useState(false);
  const { data: notifs } = useNotifications();
  const markRead = useMarkRead();
  const unread = (notifs ?? []).filter((n) => !n.read_at).length;

  return (
    <header className="flex h-14 shrink-0 items-center justify-between border-b border-pd-border bg-pd-surface px-6">
      <h1 className="text-lg font-bold text-pd-text">{title}</h1>
      <div className="flex items-center gap-4">
        <div className="relative">
          <button
            onClick={() => setOpen((o) => !o)}
            className="relative text-pd-muted hover:text-pd-text"
          >
            <Bell className="h-5 w-5" />
            {unread > 0 && (
              <span className="absolute -right-1 -top-1 flex h-4 min-w-4 items-center justify-center rounded-full bg-pd-red px-1 text-[10px] font-bold text-white">
                {unread}
              </span>
            )}
          </button>
          {open && (
            <div className="absolute right-0 top-8 z-50 w-80 rounded-lg border border-pd-border bg-white shadow-xl">
              <div className="border-b border-pd-border px-4 py-2 text-sm font-semibold">
                Notifications
              </div>
              <div className="max-h-80 overflow-y-auto">
                {(notifs ?? []).length === 0 && (
                  <p className="px-4 py-6 text-center text-sm text-pd-muted">No notifications</p>
                )}
                {(notifs ?? []).map((n) => (
                  <button
                    key={n.id}
                    onClick={() => markRead.mutate(n.id)}
                    className={cn(
                      "flex w-full flex-col items-start border-b border-pd-border px-4 py-2 text-left last:border-0 hover:bg-pd-stage",
                      !n.read_at && "bg-pd-blue/5"
                    )}
                  >
                    <span className="text-sm font-medium">{labels[n.type] ?? n.type}</span>
                    <span className="text-xs text-pd-muted">
                      {formatDistanceToNow(new Date(n.created_at), { addSuffix: true })}
                    </span>
                  </button>
                ))}
              </div>
            </div>
          )}
        </div>
        <div className="flex items-center gap-2">
          <Avatar name={user?.full_name ?? "?"} />
          <div className="hidden text-right sm:block">
            <div className="text-sm font-semibold leading-tight">{user?.full_name}</div>
            <div className="text-xs capitalize text-pd-muted">{user?.role}</div>
          </div>
        </div>
        <button
          onClick={() => {
            logout();
            queryClient.clear();
            navigate("/login");
          }}
          className="text-pd-muted hover:text-pd-red"
          title="Log out"
        >
          <LogOut className="h-5 w-5" />
        </button>
      </div>
    </header>
  );
}
