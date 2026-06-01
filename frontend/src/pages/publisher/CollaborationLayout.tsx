import { NavLink, Outlet } from "react-router-dom";
import { PageBody } from "@/components/layout/PageBody";
import { cn } from "@/lib/utils";

const tabs = [
  { to: "/p/collaboration", label: "Access", end: true },
  { to: "/p/collaboration/activity", label: "Activity Log", end: false },
] as const;

export function PublisherCollaborationLayout() {
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
