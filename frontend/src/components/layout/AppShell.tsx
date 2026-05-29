import { Outlet, useLocation } from "react-router-dom";
import { Sidebar } from "./Sidebar";
import { Topbar } from "./Topbar";
import { LeadDetailDrawer } from "@/features/leads/LeadDetailDrawer";

const titles: Record<string, string> = {
  board: "Board",
  leads: "Leads",
  intake: "Intake Queue",
  pipelines: "Pipelines",
  fields: "Custom Fields",
  reasons: "Disqualification Reasons",
  routing: "Routing",
  contracts: "Contracts",
  contract: "Contract",
  buyers: "Buyers",
  billing: "Billing",
  users: "Users",
  api: "API Keys",
  calendar: "Calendar",
  settings: "Settings",
};

export function AppShell() {
  const loc = useLocation();
  const seg = loc.pathname.split("/").filter(Boolean);
  const last = seg[seg.length - 1];
  const title = titles[last] ?? "Dashboard";

  return (
    <div className="flex h-screen overflow-hidden">
      <Sidebar />
      <div className="flex flex-1 flex-col overflow-hidden">
        <Topbar title={title} />
        <main className="flex-1 overflow-y-auto p-6">
          <Outlet />
        </main>
      </div>
      <LeadDetailDrawer />
    </div>
  );
}
