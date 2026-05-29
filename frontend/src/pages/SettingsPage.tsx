import { useAuthStore } from "@/store/authStore";
import { PageHeader } from "@/components/layout/PageHeader";
import { Card } from "@/components/ui/misc";

function Row({ label, value }: { label: string; value: string }) {
  return (
    <div className="flex justify-between border-b border-pd-border py-2 last:border-0">
      <span className="text-sm text-pd-muted">{label}</span>
      <span className="text-sm font-medium">{value}</span>
    </div>
  );
}

export function SettingsPage() {
  const user = useAuthStore((s) => s.user);
  return (
    <div className="max-w-xl">
      <PageHeader title="Settings" subtitle="Your account profile." />
      <Card className="p-5">
        <Row label="Name" value={user?.full_name ?? "—"} />
        <Row label="Email" value={user?.email ?? "—"} />
        <Row label="Role" value={user?.role ?? "—"} />
        <Row label="Account type" value={user?.account_type ?? "—"} />
      </Card>
    </div>
  );
}
