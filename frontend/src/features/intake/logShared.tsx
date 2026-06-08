import { Badge } from "@/components/ui/misc";

export type LogTypeFilter = "intake" | "webhooks" | "all";

export const LOG_TYPE_FILTERS: { value: LogTypeFilter; label: string }[] = [
  { value: "all", label: "All" },
  { value: "intake", label: "Sources" },
  { value: "webhooks", label: "Webhooks" },
];

export type LogFilter = "all" | "pending_review" | "routed" | "rejected";

export const LOG_FILTERS: { value: LogFilter; label: string }[] = [
  { value: "all", label: "All" },
  { value: "pending_review", label: "Pending" },
  { value: "routed", label: "Routed" },
  { value: "rejected", label: "Rejected" },
];

export const PAGE_SIZES = [25, 50, 100] as const;

export type WebhookDeliveryStatusFilter = "" | "success" | "error" | "skipped";

export const WEBHOOK_DELIVERY_FILTERS: { value: WebhookDeliveryStatusFilter; label: string }[] = [
  { value: "", label: "All" },
  { value: "success", label: "Success" },
  { value: "error", label: "Error" },
  { value: "skipped", label: "Captured" },
];

export function webhookDeliveryStatusLabel(status: string) {
  return status === "skipped" ? "Captured" : status;
}

export function webhookDeliveryStatusBadge(status: string) {
  return <Badge>{webhookDeliveryStatusLabel(status)}</Badge>;
}

export function statusBadge(status: string) {
  if (status === "pending_review") return <Badge variant="review">Pending</Badge>;
  if (status === "routed") return <Badge variant="distributed">Routed</Badge>;
  if (status === "returned") return <Badge variant="closed">Returned</Badge>;
  if (status === "rejected") return <Badge variant="closed">Rejected</Badge>;
  return <Badge>{status}</Badge>;
}
