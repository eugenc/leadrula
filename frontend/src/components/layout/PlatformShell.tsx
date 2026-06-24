import { useEffect } from "react";
import { Outlet, useLocation } from "react-router-dom";
import { Sidebar } from "./Sidebar";
import { Topbar } from "./Topbar";
import { ImpersonationBanner } from "./ImpersonationBanner";
import { useUIStore } from "@/store/uiStore";

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
  const sidebarOpen = useUIStore((s) => s.sidebarOpen);
  const closeSidebar = useUIStore((s) => s.closeSidebar);

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
        <main className="flex flex-1 flex-col overflow-y-auto">
          <Outlet />
        </main>
      </div>
    </div>
  );
}
