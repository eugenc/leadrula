import { useAuthStore } from "@/store/authStore";
import { useLeads } from "@/features/leads/hooks";
import { useBalance } from "@/features/admin/hooks";
import { StatCard } from "@/components/ui/misc";
import { PageHeader } from "@/components/layout/PageHeader";
import { PageBody } from "@/components/layout/PageBody";
import { formatMoney } from "@/lib/utils";

export function Dashboard() {
  const user = useAuthStore((s) => s.user);
  const { data: leadsData } = useLeads({ all: true });
  const isBuyer = user?.account_type === "buyer";
  const { data: balance } = useBalance();

  const all = leadsData?.items ?? [];
  const distributed = all.filter((l) => l.status === "distributed").length;
  const returned = all.filter((l) => l.status === "returned").length;
  const review = all.filter((l) => l.status === "review").length;

  return (
    <>
      <PageHeader title={`Welcome, ${user?.full_name ?? ""}`} subtitle="Here's your overview." />
      <PageBody>
        <div className="grid grid-cols-2 gap-4 md:grid-cols-4">
          <StatCard label="Total Leads" value={all.length} />
          <StatCard label="Distributed" value={distributed} />
          {isBuyer ? (
            <StatCard label="Balance" value={formatMoney(balance?.balance)} />
          ) : (
            <StatCard label="In Review" value={review} />
          )}
          <StatCard label="Returned" value={returned} />
        </div>
      </PageBody>
    </>
  );
}
