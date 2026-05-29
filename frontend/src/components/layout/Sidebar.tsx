import { NavLink } from "react-router-dom";
import { useAuthStore } from "@/store/authStore";
import { cn } from "@/lib/utils";
import {
  LayoutDashboard,
  KanbanSquare,
  List,
  Inbox,
  GitBranch,
  Tags,
  Ban,
  Route,
  FileText,
  Users,
  CreditCard,
  Building2,
  Calendar,
  KeyRound,
  Settings,
} from "lucide-react";

interface Item {
  to: string;
  label: string;
  icon: React.ComponentType<{ className?: string }>;
  adminOnly?: boolean;
}

const publisherItems: Item[] = [
  { to: "/p", label: "Dashboard", icon: LayoutDashboard },
  { to: "/p/board", label: "Board", icon: KanbanSquare },
  { to: "/p/leads", label: "Leads", icon: List },
  { to: "/p/intake", label: "Intake Queue", icon: Inbox, adminOnly: true },
  { to: "/p/pipelines", label: "Pipelines", icon: GitBranch, adminOnly: true },
  { to: "/p/fields", label: "Custom Fields", icon: Tags, adminOnly: true },
  { to: "/p/reasons", label: "Disq. Reasons", icon: Ban, adminOnly: true },
  { to: "/p/routing", label: "Routing", icon: Route, adminOnly: true },
  { to: "/p/contracts", label: "Contracts", icon: FileText, adminOnly: true },
  { to: "/p/buyers", label: "Buyers", icon: Building2, adminOnly: true },
  { to: "/p/billing", label: "Billing", icon: CreditCard },
  { to: "/p/users", label: "Users", icon: Users, adminOnly: true },
  { to: "/p/api", label: "API Keys", icon: KeyRound, adminOnly: true },
  { to: "/p/settings", label: "Settings", icon: Settings },
];

const buyerItems: Item[] = [
  { to: "/b", label: "Dashboard", icon: LayoutDashboard },
  { to: "/b/board", label: "Board", icon: KanbanSquare },
  { to: "/b/leads", label: "Leads", icon: List },
  { to: "/b/calendar", label: "Calendar", icon: Calendar },
  { to: "/b/pipelines", label: "Pipelines", icon: GitBranch, adminOnly: true },
  { to: "/b/fields", label: "Custom Fields", icon: Tags, adminOnly: true },
  { to: "/b/reasons", label: "Disq. Reasons", icon: Ban, adminOnly: true },
  { to: "/b/contract", label: "Contract", icon: FileText },
  { to: "/b/billing", label: "Billing", icon: CreditCard },
  { to: "/b/users", label: "Users", icon: Users, adminOnly: true },
  { to: "/b/api", label: "API Keys", icon: KeyRound, adminOnly: true },
  { to: "/b/settings", label: "Settings", icon: Settings },
];

export function Sidebar() {
  const user = useAuthStore((s) => s.user);
  if (!user) return null;
  const items = user.account_type === "publisher" ? publisherItems : buyerItems;
  const visible = items.filter((i) => !i.adminOnly || user.role === "admin");

  return (
    <aside className="flex w-56 shrink-0 flex-col border-r border-pd-border bg-pd-surface">
      <div className="flex h-14 items-center gap-2 border-b border-pd-border px-4">
        <div className="flex h-7 w-7 items-center justify-center rounded bg-pd-green font-bold text-white">
          L
        </div>
        <span className="font-bold text-pd-text">LeadRula</span>
      </div>
      <nav className="flex-1 overflow-y-auto p-2">
        {visible.map((item) => {
          const Icon = item.icon;
          return (
            <NavLink
              key={item.to}
              to={item.to}
              end={item.to === "/p" || item.to === "/b"}
              className={({ isActive }) =>
                cn(
                  "mb-0.5 flex items-center gap-2.5 rounded px-3 py-2 text-sm font-medium",
                  isActive
                    ? "bg-pd-green/10 text-pd-green"
                    : "text-pd-text hover:bg-pd-stage"
                )
              }
            >
              <Icon className="h-4 w-4 shrink-0" />
              {item.label}
            </NavLink>
          );
        })}
      </nav>
    </aside>
  );
}
