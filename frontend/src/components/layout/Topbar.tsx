import { useState } from "react";
import { useNavigate } from "react-router-dom";
import { Bell, LogOut, Menu } from "lucide-react";
import { useAuthStore } from "@/store/authStore";
import { Avatar } from "@/components/ui/misc";
import { Dropdown, DropdownItem } from "@/components/ui/dropdown";
import { useNotifications, useMarkRead } from "@/hooks/useNotifications";
import { queryClient } from "@/lib/queryClient";
import { cn, formatMoney, formatRole } from "@/lib/utils";
import { formatDistanceToNow } from "date-fns";
import type { NotificationItem } from "@/types";
import { useUIStore } from "@/store/uiStore";
import { AccountSwitcher } from "./AccountSwitcher";
import { SwitchSessionIndicator } from "./SwitchSessionIndicator";
import { ThemeToggle } from "./ThemeToggle";

const labels: Record<string, string> = {
  new_lead: "New lead received",
  lead_returned: "A lead was returned",
  collaboration_request: "Collaboration request",
  partnership_request: "Partnership request",
  partnership_accepted: "Partnership accepted",
};

function accountPrefix(accountType: string | undefined) {
  if (accountType === "buyer") return "/b";
  if (accountType === "publisher") return "/p";
  return null;
}

function notifLabel(n: NotificationItem) {
  if (n.type === "dispute_update") {
    const outcome = n.payload.outcome as string | undefined;
    if (outcome === "accepted") return "Dispute accepted";
    if (outcome === "rejected") return "Dispute rejected";
    return "Dispute resolved";
  }
  if (n.type === "new_invoice") {
    const amount = n.payload.amount as number | undefined;
    if (typeof amount === "number") return `New invoice for ${formatMoney(amount)}`;
    return "New invoice received";
  }
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
  if (n.type === "contract_participation_pending") {
    const pub = n.payload.publisher_name as string | undefined;
    const contract = n.payload.contract_name as string | undefined;
    if (pub && contract) return `${pub} invited you to ${contract}`;
    if (pub) return `${pub} sent a contract invitation`;
    return "New contract invitation";
  }
  if (n.type === "contract_forked") {
    return "Counter-offer accepted — review contract";
  }
  if (n.type === "contract_participation_accepted") {
    const buyer = n.payload.buyer_name as string | undefined;
    if (buyer) return `${buyer} accepted your contract`;
    return "Buyer accepted contract";
  }
  if (n.type === "contract_participation_declined") {
    const buyer = n.payload.buyer_name as string | undefined;
    if (buyer) return `${buyer} declined your contract`;
    return "Buyer declined contract";
  }
  if (n.type === "contract_counter_pending") {
    const buyer = n.payload.buyer_name as string | undefined;
    if (buyer) return `${buyer} submitted a counter-offer`;
    return "Buyer submitted a counter-offer";
  }
  return labels[n.type] ?? n.type;
}

function notifPath(n: NotificationItem, accountType: string | undefined) {
  const prefix = accountPrefix(accountType);
  if (n.type === "new_lead" || n.type === "lead_returned") {
    if (prefix && typeof n.payload.lead_id === "number") return `${prefix}/leads`;
  }
  if (n.type === "dispute_update" || n.type === "new_invoice") {
    if (accountType === "buyer") return "/b/billing";
  }
  if (n.type === "collaboration_request") {
    if (accountType === "buyer") return "/b/collaboration";
    if (accountType === "publisher") return "/p/collaboration";
  }
  if (n.type === "partnership_request" || n.type === "partnership_accepted") {
    if (accountType === "buyer") return "/b/publishers";
    if (accountType === "publisher") return "/p/buyers";
  }
  if (
    n.type === "contract_participation_pending" ||
    n.type === "contract_forked"
  ) {
    if (accountType === "buyer") return "/b/contract";
  }
  if (
    n.type === "contract_participation_accepted" ||
    n.type === "contract_participation_declined" ||
    n.type === "contract_counter_pending"
  ) {
    if (accountType === "publisher") return "/p/contracts";
  }
  return null;
}

export function Topbar({ title }: { title: string }) {
  const user = useAuthStore((s) => s.user);
  const logout = useAuthStore((s) => s.logout);
  const navigate = useNavigate();
  const toggleSidebar = useUIStore((s) => s.toggleSidebar);
  const [open, setOpen] = useState(false);
  const { data: notifs } = useNotifications();
  const markRead = useMarkRead();
  const unread = (notifs ?? []).filter((n) => !n.read_at).length;

  return (
    <header className="sticky top-0 z-30 flex h-13 shrink-0 items-center justify-between gap-2 border-b border-gray-100 bg-surface-card px-4 lg:px-6">
      <div className="flex min-w-0 flex-1 items-center gap-2">
        <button
          type="button"
          onClick={toggleSidebar}
          className="flex h-9 w-9 shrink-0 items-center justify-center rounded-md text-gray-600 hover:bg-gray-100 lg:hidden"
          aria-label="Open menu"
        >
          <Menu className="h-5 w-5" />
        </button>
        <h1 className="truncate text-xl font-semibold text-gray-800">{title}</h1>
      </div>
      <div className="flex shrink-0 items-center gap-2 sm:gap-4">
        <div className="hidden md:block">
          <SwitchSessionIndicator />
        </div>
        <AccountSwitcher compact />
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
          className="w-[calc(100vw-2rem)] max-w-80 p-0 sm:w-80"
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
                  const leadId =
                    typeof n.payload.lead_id === "number" ? n.payload.lead_id : null;
                  if (path || leadId != null) setOpen(false);
                  if (path) navigate(path);
                  if (leadId != null) useUIStore.getState().openDetail(leadId);
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
        <div className="hidden items-center gap-2 lg:flex">
          <Avatar name={user?.full_name ?? "?"} src={user?.avatar_url} />
          <div className="text-right">
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
          className="hidden text-gray-400 hover:text-danger lg:flex"
          title="Log out"
        >
          <LogOut className="h-5 w-5" />
        </button>
      </div>
    </header>
  );
}
