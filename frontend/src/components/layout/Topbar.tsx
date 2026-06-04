import { useState } from "react";
import { useNavigate } from "react-router-dom";
import { Bell, LogOut } from "lucide-react";
import { useAuthStore } from "@/store/authStore";
import { Avatar } from "@/components/ui/misc";
import { Dropdown, DropdownItem } from "@/components/ui/dropdown";
import { useNotifications, useMarkRead } from "@/hooks/useNotifications";
import { queryClient } from "@/lib/queryClient";
import { cn, formatRole } from "@/lib/utils";
import { formatDistanceToNow } from "date-fns";
import type { NotificationItem } from "@/types";
import { AccountSwitcher } from "./AccountSwitcher";
import { ThemeToggle } from "./ThemeToggle";

const labels: Record<string, string> = {
  new_lead: "New lead received",
  lead_returned: "A lead was returned",
  dispute_update: "Dispute resolved",
  collaboration_request: "Collaboration request",
  partnership_request: "Partnership request",
  partnership_accepted: "Partnership accepted",
};

function notifLabel(n: NotificationItem) {
  if (n.type === "collaboration_request") {
    const dir = n.payload.direction as string | undefined;
    if (dir === "publisher_to_buyer") {
      return `${n.payload.publisher_name ?? "Publisher"} requested collaboration`;
    }
    if (dir === "buyer_to_publisher") {
      return `${n.payload.buyer_name ?? "Buyer"} invited you to collaborate`;
    }
  }
  if (n.type === "partnership_request") {
    const dir = n.payload.direction as string | undefined;
    if (dir === "publisher_to_buyer") {
      return `${n.payload.publisher_name ?? "Publisher"} requested a partnership`;
    }
    if (dir === "buyer_to_publisher") {
      return `${n.payload.buyer_name ?? "Buyer"} requested a partnership`;
    }
  }
  if (n.type === "partnership_accepted") {
    const by = n.payload.accepted_by as string | undefined;
    if (by === "publisher") {
      return `${n.payload.publisher_name ?? "Publisher"} accepted your partnership request`;
    }
    if (by === "buyer") {
      return `${n.payload.buyer_name ?? "Buyer"} accepted your partnership request`;
    }
  }
  return labels[n.type] ?? n.type;
}

function notifPath(n: NotificationItem, accountType: string | undefined) {
  if (n.type === "collaboration_request") {
    if (accountType === "buyer") return "/b/collaboration";
    if (accountType === "publisher") return "/p/collaboration";
  }
  if (n.type === "partnership_request" || n.type === "partnership_accepted") {
    if (accountType === "buyer") return "/b/publishers";
    if (accountType === "publisher") return "/p/buyers";
  }
  return null;
}

export function Topbar({ title }: { title: string }) {
  const user = useAuthStore((s) => s.user);
  const logout = useAuthStore((s) => s.logout);
  const navigate = useNavigate();
  const [open, setOpen] = useState(false);
  const { data: notifs } = useNotifications();
  const markRead = useMarkRead();
  const unread = (notifs ?? []).filter((n) => !n.read_at).length;

  return (
    <header className="sticky top-0 z-30 flex h-13 shrink-0 items-center justify-between border-b border-gray-100 bg-surface-card px-6">
      <h1 className="text-xl font-semibold text-gray-800">{title}</h1>
      <div className="flex items-center gap-4">
        <AccountSwitcher />
        <ThemeToggle />
        <Dropdown
          open={open}
          onClose={() => setOpen(false)}
          trigger={
            <button
              onClick={() => setOpen((o) => !o)}
              className="relative flex h-9 w-9 items-center justify-center rounded-full text-gray-500 hover:bg-gray-100 hover:text-gray-800"
            >
              <Bell className="h-5 w-5" />
              {unread > 0 && (
                <span className="absolute -right-0.5 -top-0.5 flex h-4 min-w-4 items-center justify-center rounded-full bg-danger px-1 text-[10px] font-bold text-white">
                  {unread}
                </span>
              )}
            </button>
          }
          className="w-80 p-0"
        >
          <div className="border-b border-gray-100 px-4 py-2 text-sm font-semibold text-gray-800">
            Notifications
          </div>
          <div className="max-h-80 overflow-y-auto p-1">
            {(notifs ?? []).length === 0 && (
              <p className="px-3 py-6 text-center text-sm text-gray-400">No notifications</p>
            )}
            {(notifs ?? []).map((n) => (
              <DropdownItem
                key={n.id}
                onClick={() => {
                  markRead.mutate(n.id);
                  const path = notifPath(n, user?.account_type);
                  if (path) {
                    setOpen(false);
                    navigate(path);
                  }
                }}
                className={cn(
                  "h-auto flex-col items-start py-2",
                  !n.read_at && "bg-info-bg/50"
                )}
              >
                <span className="text-sm font-medium">{notifLabel(n)}</span>
                <span className="text-xs text-gray-400">
                  {formatDistanceToNow(new Date(n.created_at), { addSuffix: true })}
                </span>
              </DropdownItem>
            ))}
          </div>
        </Dropdown>
        <div className="flex items-center gap-2">
          <Avatar name={user?.full_name ?? "?"} src={user?.avatar_url} />
          <div className="hidden text-right sm:block">
            <div className="text-base font-semibold leading-tight text-gray-800">
              {user?.full_name}
            </div>
            <div className="text-xs text-gray-400">{user?.role ? formatRole(user.role) : ""}</div>
          </div>
        </div>
        <button
          onClick={() => {
            logout();
            queryClient.clear();
            navigate("/login");
          }}
          className="text-gray-400 hover:text-danger"
          title="Log out"
        >
          <LogOut className="h-5 w-5" />
        </button>
      </div>
    </header>
  );
}
