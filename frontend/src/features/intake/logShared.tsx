import { Badge } from "@/components/ui/misc";

export type LogFilter = "all" | "pending_review" | "routed" | "rejected";

export const LOG_FILTERS: { value: LogFilter; label: string }[] = [
  { value: "all", label: "All" },
  { value: "pending_review", label: "Pending" },
  { value: "routed", label: "Routed" },
  { value: "rejected", label: "Rejected" },
];

export const PAGE_SIZES = [25, 50, 100] as const;

export function statusBadge(status: string) {
  if (status === "pending_review") return <Badge variant="review">Pending</Badge>;
  if (status === "routed") return <Badge variant="distributed">Routed</Badge>;
  if (status === "returned") return <Badge variant="closed">Returned</Badge>;
  if (status === "rejected") return <Badge variant="closed">Rejected</Badge>;
  return <Badge>{status}</Badge>;
}
