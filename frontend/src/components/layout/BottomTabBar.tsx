import { NavLink } from "react-router-dom";
import { LayoutDashboard, List, KanbanSquare } from "lucide-react";
import { useAuthStore } from "@/store/authStore";
import { cn } from "@/lib/utils";

const tabs = [
  { label: "Dashboard", icon: LayoutDashboard, segment: "" },
  { label: "Leads", icon: List, segment: "leads" },
  { label: "Pipeline", icon: KanbanSquare, segment: "board" },
] as const;

export function BottomTabBar() {
  const user = useAuthStore((s) => s.user);
  if (!user || user.account_type === "platform") return null;

  const prefix = user.account_type === "buyer" ? "/b" : "/p";

  return (
    <nav
      className="fixed bottom-0 left-0 right-0 z-40 border-t border-gray-100 bg-surface-card lg:hidden"
      style={{ paddingBottom: "env(safe-area-inset-bottom, 0px)" }}
    >
      <div className="flex h-14 items-stretch">
        {tabs.map(({ label, icon: Icon, segment }) => {
          const to = segment ? `${prefix}/${segment}` : prefix;
          return (
            <NavLink
              key={to}
              to={to}
              end={!segment}
              className={({ isActive }) =>
                cn(
                  "flex flex-1 flex-col items-center justify-center gap-0.5 text-xs transition-colors",
                  isActive ? "font-semibold text-jade-700" : "font-normal text-gray-500"
                )
              }
            >
              <Icon className="h-5 w-5" />
              {label}
            </NavLink>
          );
        })}
      </div>
    </nav>
  );
}
