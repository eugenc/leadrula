import { useEffect } from "react";
import { Outlet, useLocation } from "react-router-dom";
import { Sidebar } from "./Sidebar";
import { Topbar } from "./Topbar";
import { ImpersonationBanner } from "./ImpersonationBanner";
import { AccountSwitcher } from "./AccountSwitcher";
import { LeadDetailDrawer } from "@/features/leads/LeadDetailDrawer";
import { useMe } from "@/features/leads/hooks";
import { useAuthStore } from "@/store/authStore";

const titles: Record<string, string> = {
  board: "Pipeline",
  leads: "Leads",
  log: "Log",
  review: "Review",
  pipelines: "Pipelines",
  fields: "Custom Fields",
  reasons: "Disqualification Reasons",
  routing: "Routing",
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
  calendar: "Calendar",
  settings: "Settings",
  collaboration: "Collaboration",
  activity: "Activity Log",
};

export function AppShell() {
  const loc = useLocation();
  const seg = loc.pathname.split("/").filter(Boolean);
  const last = seg[seg.length - 1];
  const title = titles[last] ?? "Dashboard";
  const { data: me } = useMe();
  const syncUserProfile = useAuthStore((s) => s.syncUserProfile);
  const syncFromMe = useAuthStore((s) => s.syncFromMe);

  useEffect(() => {
    if (!me) return;
    syncUserProfile({
      full_name: me.user.full_name,
      email: me.user.email,
      role: me.user.role,
      avatar_url: me.user.avatar_url,
    });
    syncFromMe({
      is_switched: me.is_switched,
      switched_from: me.switched_from,
      account_name: me.account.name,
      impersonating: me.impersonating,
      buyer_account_name: me.buyer_account_name,
    });
  }, [me, syncUserProfile, syncFromMe]);

  return (
    <div className="flex h-screen overflow-hidden bg-surface-app">
      <Sidebar />
      <div className="flex flex-1 flex-col overflow-hidden">
        <ImpersonationBanner />
        <Topbar title={title} />
        <main className="flex flex-1 flex-col overflow-y-auto">
          <Outlet />
        </main>
      </div>
      <LeadDetailDrawer />
    </div>
  );
}
