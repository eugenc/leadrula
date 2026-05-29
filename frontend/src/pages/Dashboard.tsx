import { useAuthStore } from "@/store/authStore";
import { useLeads } from "@/features/leads/hooks";
import { useBalance } from "@/features/admin/hooks";
import { Card } from "@/components/ui/misc";
import { PageHeader } from "@/components/layout/PageHeader";
import { formatMoney } from "@/lib/utils";

function Stat({ label, value }: { label: string; value: string | number }) {
  return (
    <Card className="p-4">
      <div className="text-xs font-semibold uppercase tracking-wide text-pd-muted">{label}</div>
      <div className="mt-1 text-2xl font-bold text-pd-text">{value}</div>
    </Card>
  );
}

export function Dashboard() {
  const user = useAuthStore((s) => s.user);
  const { data: leads } = useLeads();
  const isBuyer = user?.account_type === "buyer";
  const { data: balance } = useBalance();

  const all = leads ?? [];
  const distributed = all.filter((l) => l.status === "distributed").length;
  const returned = all.filter((l) => l.status === "returned").length;
  const review = all.filter((l) => l.status === "review").length;

  return (
    <div>
      <PageHeader title={`Welcome, ${user?.full_name ?? ""}`} subtitle="Here's your overview." />
      <div className="grid grid-cols-2 gap-4 md:grid-cols-4">
        <Stat label="Total Leads" value={all.length} />
        <Stat label="Distributed" value={distributed} />
        {isBuyer ? (
          <Stat label="Balance" value={formatMoney(balance?.balance)} />
        ) : (
          <Stat label="In Review" value={review} />
        )}
        <Stat label="Returned" value={returned} />
      </div>
    </div>
  );
}
