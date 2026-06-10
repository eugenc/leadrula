import { useMemo } from "react";
import { useAuthStore } from "@/store/authStore";
import { useLeads } from "@/features/leads/hooks";
import { useBalance } from "@/features/admin/hooks";
import { StatCard, Spinner } from "@/components/ui/misc";
import { PageHeader } from "@/components/layout/PageHeader";
import { PageBody } from "@/components/layout/PageBody";
import { formatMoney } from "@/lib/utils";
import {
  DashboardStatCards,
  DashboardWideWidgets,
  useDashboardStats,
} from "@/features/dashboard/DashboardWidgets";
import {
  DashboardViewPicker,
  useResolvedDashboardView,
} from "@/features/dashboard/DashboardViewPicker";
import { useDashboardPeriod, inPeriod } from "@/features/dashboard/dashboardViews";

export function Dashboard() {
  const user = useAuthStore((s) => s.user);
  const { data: leadsData } = useLeads({ all: true });
  const isBuyer = user?.account_type === "buyer";
  const { data: balance } = useBalance();
  const { view, setView, viewsLoading } = useResolvedDashboardView(!!isBuyer);
  const { periodState, isLoading: periodLoading } = useDashboardPeriod();
  const { stats, loading: statsLoading } = useDashboardStats(view, !!isBuyer, periodState);

  const fixedStats = useMemo(() => {
    const all = leadsData?.items ?? [];
    const inRange = all.filter((l) => inPeriod(l.created_at, periodState));
    return {
      total: inRange.length,
      distributed: inRange.filter((l) => l.status === "distributed").length,
      returned: inRange.filter((l) => l.status === "returned").length,
      review: inRange.filter((l) => l.status === "review").length,
    };
  }, [leadsData, periodState]);

  const ready = !viewsLoading && !statsLoading && !periodLoading;

  return (
    <>
      <PageHeader
        title={`Welcome, ${user?.full_name ?? ""}`}
        subtitle="Here's your overview."
      />
      <PageBody>
        {!viewsLoading && !periodLoading && (
          <DashboardViewPicker view={view} onViewChange={setView} />
        )}

        {!ready ? (
          <div className="mt-8 flex justify-center py-12">
            <Spinner className="h-6 w-6" />
          </div>
        ) : (
          <>
            <div className="mt-4 grid grid-cols-2 gap-4 md:grid-cols-4">
              <StatCard label="Total Leads" value={fixedStats.total.toLocaleString()} />
              <StatCard label="Distributed" value={fixedStats.distributed.toLocaleString()} />
              {isBuyer ? (
                <StatCard label="Balance" value={formatMoney(balance?.balance)} />
              ) : (
                <StatCard label="In Review" value={fixedStats.review.toLocaleString()} />
              )}
              <StatCard label="Returned" value={fixedStats.returned.toLocaleString()} />
              <DashboardStatCards view={view} stats={stats} />
            </div>

            <div className="mt-4">
              <DashboardWideWidgets view={view} isBuyer={!!isBuyer} stats={stats} />
            </div>
          </>
        )}
      </PageBody>
    </>
  );
}
