import { NavLink, useLocation } from "react-router-dom";
import { useAuthStore } from "@/store/authStore";
import { cn } from "@/lib/utils";
import { Logo } from "@/components/layout/Logo";
import {
  LayoutDashboard,
  KanbanSquare,
  List,
  Inbox,
  GitBranch,
  Tags,
  Ban,
  Route,
  Webhook,
  FileText,
  Users,
  CreditCard,
  Building2,
  Calendar,
  KeyRound,
  Settings,
  Handshake,
  type LucideIcon,
} from "lucide-react";

interface Item {
  to: string;
  label: string;
  icon: LucideIcon;
  adminOnly?: boolean;
}

interface NavGroup {
  label?: string;
  items: Item[];
}

const publisherNav: NavGroup[] = [
  { items: [{ to: "/p", label: "Dashboard", icon: LayoutDashboard }] },
  {
    label: "Leads",
    items: [
      { to: "/p/leads", label: "Leads", icon: List },
      { to: "/p/fields", label: "Custom Fields", icon: Tags, adminOnly: true },
    ],
  },
  {
    label: "Pipeline",
    items: [
      { to: "/p/board", label: "Pipeline", icon: KanbanSquare },
      { to: "/p/pipelines", label: "Pipelines", icon: GitBranch, adminOnly: true },
      { to: "/p/reasons", label: "Disqualification", icon: Ban, adminOnly: true },
    ],
  },
  {
    label: "Buyers",
    items: [
      { to: "/p/buyers", label: "Buyers List", icon: Building2, adminOnly: true },
      { to: "/p/contracts", label: "Contracts", icon: FileText, adminOnly: true },
    ],
  },
  {
    label: "Routing",
    items: [
      { to: "/p/sources", label: "Sources", icon: Webhook, adminOnly: true },
      { to: "/p/routing", label: "Routing", icon: Route, adminOnly: true },
      { to: "/p/intake", label: "Intake Queue", icon: Inbox, adminOnly: true },
    ],
  },
];

const publisherSettings: NavGroup = {
  label: "Settings",
  items: [
    { to: "/p/settings", label: "Profile", icon: Settings },
    { to: "/p/billing", label: "Billing", icon: CreditCard },
    { to: "/p/users", label: "Users", icon: Users, adminOnly: true },
    { to: "/p/api", label: "API Keys", icon: KeyRound, adminOnly: true },
  ],
};

const buyerNav: NavGroup[] = [
  { items: [{ to: "/b", label: "Dashboard", icon: LayoutDashboard }] },
  {
    label: "Leads",
    items: [
      { to: "/b/leads", label: "Leads", icon: List },
      { to: "/b/calendar", label: "Calendar", icon: Calendar },
      { to: "/b/fields", label: "Custom Fields", icon: Tags, adminOnly: true },
    ],
  },
  {
    label: "Pipeline",
    items: [
      { to: "/b/board", label: "Pipeline", icon: KanbanSquare },
      { to: "/b/pipelines", label: "Pipelines", icon: GitBranch, adminOnly: true },
      { to: "/b/reasons", label: "Disqualification", icon: Ban, adminOnly: true },
    ],
  },
];

const buyerSettings: NavGroup = {
  label: "Settings",
  items: [
    { to: "/b/settings", label: "Profile", icon: Settings },
    { to: "/b/settings/collaboration", label: "Collaboration", icon: Handshake, adminOnly: true },
    { to: "/b/billing", label: "Billing", icon: CreditCard },
    { to: "/b/contract", label: "Contract", icon: FileText },
    { to: "/b/users", label: "Users", icon: Users, adminOnly: true },
    { to: "/b/api", label: "API Keys", icon: KeyRound, adminOnly: true },
  ],
};

function NavItem({ item }: { item: Item }) {
  const Icon = item.icon;
  return (
    <NavLink
      to={item.to}
      end={item.to === "/p" || item.to === "/b"}
      className={({ isActive }) =>
        cn(
          "mb-0.5 flex h-8 items-center gap-2 rounded-md pl-7 pr-2.5 text-base transition-colors",
          isActive
            ? "bg-jade-100 font-semibold text-jade-700"
            : "font-normal text-gray-600 hover:bg-jade-50 hover:text-gray-800"
        )
      }
    >
      <Icon className="h-4 w-4 shrink-0 opacity-75 [.active_&]:opacity-100" />
      {item.label}
    </NavLink>
  );
}

function NavGroupSection({
  group,
  isAdmin,
  pathname,
}: {
  group: NavGroup;
  isAdmin: boolean;
  pathname: string;
}) {
  const visible = group.items.filter((i) => !i.adminOnly || isAdmin);
  if (visible.length === 0) return null;

  const childActive = visible.some(
    (i) => pathname === i.to || (i.to !== "/p" && i.to !== "/b" && pathname.startsWith(i.to))
  );

  return (
    <div>
      {group.label && (
        <div
          className={cn(
            "px-3 pb-1 pt-3 text-xs font-semibold uppercase tracking-wide",
            childActive ? "text-jade-600" : "text-gray-400"
          )}
        >
          {group.label}
        </div>
      )}
      {visible.map((item) => (
        <NavItem key={item.to} item={item} />
      ))}
    </div>
  );
}

export function Sidebar() {
  const user = useAuthStore((s) => s.user);
  const { pathname } = useLocation();
  if (!user) return null;

  const isAdmin = user.role === "admin";
  const groups = user.account_type === "publisher" ? publisherNav : buyerNav;
  const settings = user.account_type === "publisher" ? publisherSettings : buyerSettings;

  return (
    <aside className="flex w-sidebar shrink-0 flex-col border-r border-gray-100 bg-white">
      <div className="mb-2 flex items-center gap-2.5 p-3">
        <Logo />
      </div>
      <nav className="flex-1 overflow-y-auto px-2">
        {groups.map((group, i) => (
          <NavGroupSection key={group.label ?? i} group={group} isAdmin={isAdmin} pathname={pathname} />
        ))}
      </nav>
      <div className="mt-auto border-t border-gray-100 px-2 pb-2 pt-2">
        <NavGroupSection group={settings} isAdmin={isAdmin} pathname={pathname} />
      </div>
    </aside>
  );
}
