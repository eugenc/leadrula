import { NavLink, Outlet, useLocation } from "react-router-dom";
import { useAuthStore } from "@/store/authStore";
import { PageBody } from "@/components/layout/PageBody";
import { cn } from "@/lib/utils";

type Tab = { to: string; label: string; end: boolean; adminOnly?: boolean };

function prefixFromPath(pathname: string): string {
  if (pathname.startsWith("/platform/")) return "/platform";
  if (pathname.startsWith("/p/")) return "/p";
  return "/b";
}

function tabsForAccount(accountType: string, prefix: string): Tab[] {
  if (accountType === "platform") {
    return [
      { to: `${prefix}/settings`, label: "Profile", end: true },
      { to: `${prefix}/settings/users`, label: "Users", end: false, adminOnly: true },
    ];
  }
  return [
    { to: `${prefix}/settings`, label: "Profile", end: true },
    { to: `${prefix}/settings/notifications`, label: "Notifications", end: false },
    { to: `${prefix}/settings/business`, label: "Business", end: false, adminOnly: true },
    { to: `${prefix}/settings/users`, label: "Users", end: false, adminOnly: true },
    { to: `${prefix}/settings/api`, label: "API Keys", end: false, adminOnly: true },
  ];
}

export function SettingsLayout() {
  const { pathname } = useLocation();
  const user = useAuthStore((s) => s.user);
  const isAdmin = user?.role === "admin";
  const prefix = prefixFromPath(pathname);
  const accountType = user?.account_type ?? "publisher";
  const tabs = tabsForAccount(accountType, prefix).filter((t) => !t.adminOnly || isAdmin);

  return (
    <PageBody>
      <div className="mb-4 flex border-b border-gray-100">
        {tabs.map((tab) => (
          <NavLink
            key={tab.to}
            to={tab.to}
            end={tab.end}
            className={({ isActive }) =>
              cn(
                "-mb-px border-b-2 px-4 py-2 text-base font-semibold transition-colors",
                isActive ? "border-jade-500 text-jade-700" : "border-transparent text-gray-400"
              )
            }
          >
            {tab.label}
          </NavLink>
        ))}
      </div>
      <Outlet />
    </PageBody>
  );
}
