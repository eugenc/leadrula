export const CONTRACT_STATUSES = [
  { value: "draft", label: "Draft" },
  { value: "active", label: "Active" },
  { value: "paused", label: "Paused" },
  { value: "terminated", label: "Terminated" },
] as const;

const LABELS = Object.fromEntries(CONTRACT_STATUSES.map((s) => [s.value, s.label]));

export function formatContractStatus(status: string): string {
  return LABELS[status] ?? status.charAt(0).toUpperCase() + status.slice(1);
}

export function ContractStatusBadge({ status }: { status: string }) {
  return <>{formatContractStatus(status)}</>;
}
