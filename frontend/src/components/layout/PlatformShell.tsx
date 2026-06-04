import { Outlet, useLocation } from "react-router-dom";
import { Sidebar } from "./Sidebar";
import { Topbar } from "./Topbar";
import { ImpersonationBanner } from "./ImpersonationBanner";

const titles: Record<string, string> = {
  publishers: "Publishers",
  buyers: "Buyers",
  users: "Users",
  settings: "Settings",
};

export function PlatformShell() {
  const loc = useLocation();
  const seg = loc.pathname.split("/").filter(Boolean);
  const last = seg[seg.length - 1];
  const title = titles[last] ?? "Dashboard";

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
    </div>
  );
}
