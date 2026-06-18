import { Button } from "@/components/ui/button";
import { FilterSelect } from "@/components/ui/input";
import type { WebhookDelivery } from "@/types";
import { pipelineStage } from "@/features/routing/routeFormatters";

export type LogTypeFilter = "intake" | "webhooks" | "integrations" | "routes" | "all";

export const LOG_TYPE_FILTERS: { value: LogTypeFilter; label: string }[] = [
  { value: "all", label: "All" },
  { value: "routes", label: "Routes" },
  { value: "intake", label: "Sources" },
  { value: "webhooks", label: "Webhooks" },
  { value: "integrations", label: "Integrations" },
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

export function integrationDeliveryStatusLabel(status: string) {
  if (status === "failed") return "Failed";
  if (status === "dead") return "Dead";
  return status.charAt(0).toUpperCase() + status.slice(1);
}

export function statusLabel(status: string) {
  if (status === "pending_review") return "Pending";
  if (status === "routed") return "Routed";
  if (status === "returned") return "Returned";
  if (status === "rejected") return "Rejected";
  return status;
}

export function routeTriggerLabel(triggerType: string) {
  switch (triggerType) {
    case "source_ingest":
      return "Source ingest";
    case "stage":
      return "Stage trigger";
    case "webhook":
      return "Webhook origin";
    case "integration":
      return "Integration origin";
    case "manual":
      return "Manual route";
    case "preassigned":
      return "Pre-assigned buyer";
    case "legacy_buyer":
      return "Manual distribution";
    default:
      return triggerType || "—";
  }
}

export function routeDestinationLabel(destination?: string) {
  if (!destination) return "—";
  if (destination === "contract") return "Contract";
  if (destination === "pipeline") return "Pipeline";
  return destination.charAt(0).toUpperCase() + destination.slice(1);
}

export function routePipelineDestinationLabel(
  destination?: string,
  delivery?: string,
  pipelineName?: string | null,
  stageName?: string | null,
) {
  if (destination !== "pipeline") return routeDestinationLabel(destination);
  if (delivery === "leads") return "Pipeline (lead)";
  if (pipelineName) return `Pipeline: ${pipelineStage(pipelineName, stageName)}`;
  return routeDestinationLabel("pipeline");
}

export function statusText(status: string) {
  return <span className="text-sm text-gray-700">{statusLabel(status)}</span>;
}

export function webhookDeliveryStatusText(status: string) {
  return <span className="text-sm text-gray-700">{webhookDeliveryStatusLabel(status)}</span>;
}

export function integrationDeliveryStatusText(status: string) {
  return <span className="text-sm text-gray-700">{integrationDeliveryStatusLabel(status)}</span>;
}

/** @deprecated use statusText */
export function statusBadge(status: string) {
  return statusText(status);
}

/** @deprecated use webhookDeliveryStatusText */
export function webhookDeliveryStatusBadge(status: string) {
  return webhookDeliveryStatusText(status);
}

export function canReplayDelivery(d: WebhookDelivery) {
  return d.status === "skipped" && !d.lead_id;
}

export function leadDisplayName(
  firstName?: string,
  lastName?: string,
  fallback?: string | null
): string {
  const name = `${firstName ?? ""} ${lastName ?? ""}`.trim();
  return name || fallback || "—";
}

export function LogLeadLink({
  leadId,
  firstName,
  lastName,
  fallback,
  onClick,
}: {
  leadId?: number | null;
  firstName?: string;
  lastName?: string;
  fallback?: string | null;
  onClick: (leadId: number) => void;
}) {
  const label = leadDisplayName(firstName, lastName, fallback);
  if (leadId == null) {
    return <span className="font-medium text-gray-800">{label}</span>;
  }
  return (
    <button
      type="button"
      className="font-medium text-jade-600 hover:underline"
      onClick={(e) => {
        e.stopPropagation();
        onClick(leadId);
      }}
    >
      {label}
    </button>
  );
}

export function LogPagination({
  page,
  limit,
  total,
  onPageChange,
  onLimitChange,
}: {
  page: number;
  limit: number;
  total: number;
  onPageChange: (page: number) => void;
  onLimitChange: (limit: number) => void;
}) {
  const totalPages = Math.max(1, Math.ceil(total / limit));

  return (
    <div className="mt-4 flex flex-wrap items-center justify-between gap-3 text-sm text-gray-500">
      <span>
        {total === 0
          ? "No results"
          : `${(page - 1) * limit + 1}–${Math.min(page * limit, total)} of ${total}`}
      </span>
      <div className="flex items-center gap-3">
        <FilterSelect value={limit} onChange={(e) => onLimitChange(Number(e.target.value))} className="w-24">
          {PAGE_SIZES.map((n) => (
            <option key={n} value={n}>
              {n} / page
            </option>
          ))}
        </FilterSelect>
        <Button variant="secondary" size="sm" disabled={page <= 1} onClick={() => onPageChange(page - 1)}>
          Previous
        </Button>
        <span>
          Page {page} of {totalPages}
        </span>
        <Button
          variant="secondary"
          size="sm"
          disabled={page >= totalPages}
          onClick={() => onPageChange(page + 1)}
        >
          Next
        </Button>
      </div>
    </div>
  );
}
