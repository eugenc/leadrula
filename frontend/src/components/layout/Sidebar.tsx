import { NavLink, useLocation, useNavigate } from "react-router-dom";
import { useAuthStore } from "@/store/authStore";
import { useUIStore } from "@/store/uiStore";
import { cn } from "@/lib/utils";
import { canNav } from "@/lib/permissions";
import { queryClient } from "@/lib/queryClient";
import { Logo } from "@/components/layout/Logo";
import {
  LayoutDashboard,
  KanbanSquare,
  List,
  GitBranch,
  Tags,
  Route,
  Webhook,
  FileText,
  Users,
  CreditCard,
  Building2,
  Calendar,
  Plug,
  Settings,
  Handshake,
  Import,
  Logs,
  Phone,
  CalendarClock,
  LogOut,
  type LucideIcon,
} from "lucide-react";

interface Item {
  to: string;
  label: string;
  icon: LucideIcon;
  navKey: string;
}

interface NavGroup {
  label?: string;
  items: Item[];
}

const publisherNav: NavGroup[] = [
  { items: [{ to: "/p", label: "Dashboard", icon: LayoutDashboard, navKey: "dashboard" }] },
  {
    label: "Leads",
    items: [
      { to: "/p/leads", label: "Leads", icon: List, navKey: "leads" },
      { to: "/p/fields", label: "Custom Fields", icon: Tags, navKey: "fields" },
    ],
  },
  {
    label: "Appointments",
    items: [
      { to: "/p/appointments", label: "Appointments", icon: CalendarClock, navKey: "appointments" },
      { to: "/p/calendar", label: "Calendars", icon: Calendar, navKey: "calendars" },
    ],
  },
  {
    label: "Calls",
    items: [{ to: "/p/calls", label: "Calls", icon: Phone, navKey: "calls" }],
  },
  {
    label: "Pipeline",
    items: [
      { to: "/p/board", label: "Pipeline", icon: KanbanSquare, navKey: "board" },
      { to: "/p/pipelines", label: "Pipelines", icon: GitBranch, navKey: "pipelines" },
    ],
  },
  {
    label: "Buyers",
    items: [
      { to: "/p/buyers", label: "Buyers", icon: Building2, navKey: "buyers" },
      { to: "/p/contracts", label: "Contracts", icon: FileText, navKey: "contracts" },
      { to: "/p/collaboration", label: "Collaboration", icon: Handshake, navKey: "collaboration" },
    ],
  },
  {
    label: "Routing",
    items: [
      { to: "/p/sources", label: "Sources", icon: Import, navKey: "sources" },
      { to: "/p/webhooks", label: "Webhooks", icon: Webhook, navKey: "webhooks" },
      { to: "/p/routing", label: "Routing", icon: Route, navKey: "routing" },
      { to: "/p/log", label: "Logs", icon: Logs, navKey: "logs" },
    ],
  },
];

const publisherSettings: NavGroup = {
  items: [
    { to: "/p/settings", label: "Settings", icon: Settings, navKey: "settings" },
    { to: "/p/billing", label: "Billing", icon: CreditCard, navKey: "billing" },
  ],
};

const publisherBottomNav: NavGroup = {
  items: [{ to: "/p/integrations", label: "Integrations", icon: Plug, navKey: "integrations" }],
};

const buyerNav: NavGroup[] = [
  { items: [{ to: "/b", label: "Dashboard", icon: LayoutDashboard, navKey: "dashboard" }] },
  {
    label: "Leads",
    items: [
      { to: "/b/leads", label: "Leads", icon: List, navKey: "leads" },
      { to: "/b/fields", label: "Custom Fields", icon: Tags, navKey: "fields" },
    ],
  },
  {
    label: "Appointments",
    items: [
      { to: "/b/appointments", label: "Appointments", icon: CalendarClock, navKey: "appointments" },
      { to: "/b/calendar", label: "Calendars", icon: Calendar, navKey: "calendars" },
    ],
  },
  {
    label: "Calls",
    items: [{ to: "/b/calls", label: "Calls", icon: Phone, navKey: "calls" }],
  },
  {
    label: "Pipeline",
    items: [
      { to: "/b/board", label: "Pipeline", icon: KanbanSquare, navKey: "board" },
      { to: "/b/pipelines", label: "Pipelines", icon: GitBranch, navKey: "pipelines" },
    ],
  },
  {
    label: "Publishers",
    items: [
      { to: "/b/publishers", label: "Publishers", icon: Building2, navKey: "publishers" },
      { to: "/b/contract", label: "Contracts", icon: FileText, navKey: "contracts" },
      { to: "/b/collaboration", label: "Collaboration", icon: Handshake, navKey: "collaboration" },
    ],
  },
  {
    label: "Routing",
    items: [
      { to: "/b/routes", label: "Routes", icon: Route, navKey: "routes" },
      { to: "/b/webhooks", label: "Webhooks", icon: Webhook, navKey: "webhooks" },
      { to: "/b/logs", label: "Logs", icon: Logs, navKey: "logs" },
    ],
  },
];

const buyerSettings: NavGroup = {
  items: [
    { to: "/b/settings", label: "Settings", icon: Settings, navKey: "settings" },
    { to: "/b/billing", label: "Billing", icon: CreditCard, navKey: "billing" },
  ],
};

const buyerBottomNav: NavGroup = {
  items: [{ to: "/b/integrations", label: "Integrations", icon: Plug, navKey: "integrations" }],
};

const platformNav: NavGroup[] = [
  { items: [{ to: "/platform", label: "Dashboard", icon: LayoutDashboard, navKey: "dashboard" }] },
  {
    label: "Accounts",
    items: [
      { to: "/platform/publishers", label: "Publishers", icon: Building2, navKey: "publishers" },
      { to: "/platform/buyers", label: "Buyers", icon: Users, navKey: "buyers" },
    ],
  },
];

const platformSettings: NavGroup = {
  items: [{ to: "/platform/settings", label: "Settings", icon: Settings, navKey: "settings" }],
};

const dashboardPaths = new Set(["/p", "/b", "/platform"]);

function NavItem({ item }: { item: Item }) {
  const Icon = item.icon;
  const closeSidebar = useUIStore((s) => s.closeSidebar);
  return (
    <NavLink
      to={item.to}
      end={dashboardPaths.has(item.to)}
      onClick={() => closeSidebar()}
      className={({ isActive }) =>
        cn(
          "mb-0.5 flex h-8 lg:h-7 items-center gap-2 rounded-md pl-7 lg:pl-6 pr-2.5 text-base lg:text-sm transition-colors",
          isActive
            ? "bg-jade-100 font-semibold text-jade-700"
            : "font-normal text-gray-600 hover:bg-jade-50 hover:text-gray-800"
        )
      }
    >
      <Icon className="h-4 w-4 lg:h-3.5 lg:w-3.5 shrink-0 opacity-75 [.active_&]:opacity-100" />
      {item.label}
    </NavLink>
  );
}

function NavGroupSection({
  group,
  user,
  pathname,
}: {
  group: NavGroup;
  user: NonNullable<ReturnType<typeof useAuthStore.getState>["user"]>;
  pathname: string;
}) {
  const visible = group.items.filter((i) => canNav(user, i.navKey));
  if (visible.length === 0) return null;

  const childActive = visible.some(
    (i) => pathname === i.to || (!dashboardPaths.has(i.to) && pathname.startsWith(i.to))
  );

  return (
    <div>
      {group.label && (
        <div
          className={cn(
            "px-3 pb-2 pt-3 lg:pt-2 lg:pb-1.5 text-xs font-semibold uppercase tracking-wide",
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
  const logout = useAuthStore((s) => s.logout);
  const sidebarOpen = useUIStore((s) => s.sidebarOpen);
  const closeSidebar = useUIStore((s) => s.closeSidebar);
  const navigate = useNavigate();
  const { pathname } = useLocation();
  if (!user) return null;

  const groups =
    user.account_type === "platform"
      ? platformNav
      : user.account_type === "publisher"
        ? publisherNav
        : buyerNav;
  const settings =
    user.account_type === "platform"
      ? platformSettings
      : user.account_type === "publisher"
        ? publisherSettings
        : buyerSettings;
  const bottomNav =
    user.account_type === "publisher"
      ? publisherBottomNav
      : user.account_type === "buyer"
        ? buyerBottomNav
        : null;

  return (
    <aside
      className={cn(
        "flex w-sidebar shrink-0 flex-col border-r border-gray-100 bg-surface-card transition-transform duration-200 ease-out",
        "fixed inset-y-0 left-0 z-50 lg:relative lg:z-auto",
        sidebarOpen ? "translate-x-0" : "-translate-x-full",
        "lg:translate-x-0"
      )}
    >
      <div className="mb-2 flex items-center gap-2.5 p-3">
        <Logo />
      </div>
      <nav className="flex-1 overflow-y-auto px-2">
        {groups.map((group, i) => (
          <NavGroupSection key={group.label ?? i} group={group} user={user} pathname={pathname} />
        ))}
      </nav>
      <div className="mt-auto border-t border-gray-100 px-2 pb-2 pt-2">
        <NavGroupSection group={settings} user={user} pathname={pathname} />
        {bottomNav && <NavGroupSection group={bottomNav} user={user} pathname={pathname} />}
        <div className="mt-2 border-t border-gray-100 pt-2 lg:hidden">
          <button
            type="button"
            onClick={() => {
              logout();
              queryClient.clear();
              navigate("/login");
              closeSidebar();
            }}
            className="mb-0.5 flex h-8 w-full items-center gap-2 rounded-md pl-7 pr-2.5 text-base font-normal text-gray-600 transition-colors hover:bg-jade-50 hover:text-gray-800"
          >
            <LogOut className="h-4 w-4 shrink-0 opacity-75" />
            Log out
          </button>
        </div>
      </div>
    </aside>
  );
}
