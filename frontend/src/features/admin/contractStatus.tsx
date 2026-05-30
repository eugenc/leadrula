import { Badge } from "@/components/ui/misc";

export const CONTRACT_STATUSES = [
  { value: "active", label: "Active" },
  { value: "paused", label: "Paused" },
  { value: "terminated", label: "Terminated" },
] as const;

const LABELS = Object.fromEntries(CONTRACT_STATUSES.map((s) => [s.value, s.label]));

export function formatContractStatus(status: string): string {
  return LABELS[status] ?? status.charAt(0).toUpperCase() + status.slice(1);
}

function statusVariant(status: string) {
  if (status === "active") return "distributed" as const;
  if (status === "paused") return "review" as const;
  return "closed" as const;
}

export function ContractStatusBadge({ status }: { status: string }) {
  return <Badge variant={statusVariant(status)}>{formatContractStatus(status)}</Badge>;
}
