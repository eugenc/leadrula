import { useEffect } from "react";
import { Outlet, useLocation } from "react-router-dom";
import { Sidebar } from "./Sidebar";
import { Topbar } from "./Topbar";
import { BottomTabBar } from "./BottomTabBar";
import { ImpersonationBanner } from "./ImpersonationBanner";
import { LeadDetailDrawer } from "@/features/leads/LeadDetailDrawer";
import { ChatWidget } from "@/features/messaging/ChatWidget";
import { useMe } from "@/features/leads/hooks";
import { useAuthStore } from "@/store/authStore";
import { useUIStore } from "@/store/uiStore";

const titles: Record<string, string> = {
  board: "Pipeline",
  leads: "Leads",
  calls: "Calls",
  log: "Logs",
  review: "Review",
  pipelines: "Pipelines",
  fields: "Custom Fields",
  reasons: "Disqualification Reasons",
  routing: "Routing",
  webhooks: "Webhooks",
  sources: "Sources",
  contracts: "Contracts",
  contract: "Contracts",
  publishers: "Publishers",
  routes: "Routes",
  logs: "Logs",
  buyers: "Buyers",
  billing: "Billing",
  users: "Users",
  api: "API Keys",
  "api-docs": "API Documentation",
  calendar: "Calendar",
  appointments: "Appointments",
  settings: "Settings",
  collaboration: "Collaboration",
  activity: "Activity Log",
};

export function AppShell() {
  const loc = useLocation();
  const seg = loc.pathname.split("/").filter(Boolean);
  const last = seg[seg.length - 1];
  const title = seg.includes("settings")
    ? (titles.settings ?? "Settings")
    : (titles[last] ?? "Dashboard");
  const { data: me } = useMe();
  const syncUserProfile = useAuthStore((s) => s.syncUserProfile);
  const syncFromMe = useAuthStore((s) => s.syncFromMe);
  const sidebarOpen = useUIStore((s) => s.sidebarOpen);
  const closeSidebar = useUIStore((s) => s.closeSidebar);

  useEffect(() => {
    if (!me) return;
    syncUserProfile({
      full_name: me.user.full_name,
      email: me.user.email,
      role: me.user.role,
      avatar_url: me.user.avatar_url,
      effective_permissions: me.user.effective_permissions,
    });
    syncFromMe({
      is_switched: me.is_switched,
      switched_from: me.switched_from,
      account_name: me.account.name,
      impersonating: me.impersonating,
      buyer_account_name: me.buyer_account_name,
    });
  }, [me, syncUserProfile, syncFromMe]);

  useEffect(() => {
    closeSidebar();
  }, [loc.pathname, closeSidebar]);

  return (
    <div className="flex h-screen overflow-hidden bg-surface-app">
      {sidebarOpen && (
        <button
          type="button"
          aria-label="Close menu"
          className="fixed inset-0 z-40 bg-[var(--surface-overlay)] lg:hidden"
          onClick={closeSidebar}
        />
      )}
      <Sidebar />
      <div className="flex min-w-0 flex-1 flex-col overflow-hidden">
        <ImpersonationBanner />
        <Topbar title={title} />
        <main className="flex flex-1 flex-col overflow-y-auto pb-14 lg:pb-0">
          <Outlet />
        </main>
      </div>
      <BottomTabBar />
      <LeadDetailDrawer />
      <ChatWidget />
    </div>
  );
}
