import { Fragment, useEffect, useState } from "react";
import {
  useAccountWebhookDeliveries,
  useReplayWebhookDelivery,
  useWebhookDelivery,
  useWebhooks,
} from "@/features/webhooks/hooks";
import { Table, THead, TH, TBody, TR, TD } from "@/components/ui/table";
import { Button } from "@/components/ui/button";
import { FilterSelect, Label, Select } from "@/components/ui/input";
import { Spinner, EmptyState } from "@/components/ui/misc";
import { format } from "date-fns";
import { toast } from "@/store/toastStore";
import { errorMessage } from "@/lib/api";
import type { WebhookDelivery } from "@/types";
import {
  PAGE_SIZES,
  WEBHOOK_DELIVERY_FILTERS,
  webhookDeliveryStatusBadge,
  type WebhookDeliveryStatusFilter,
} from "./logShared";

interface WebhookDeliveriesLogSectionProps {
  canReplay?: boolean;
  sectionTitle?: string;
}

function canReplayDelivery(d: WebhookDelivery) {
  return d.status === "skipped" && !d.lead_id;
}

export function WebhookDeliveriesLogSection({
  canReplay = false,
  sectionTitle,
}: WebhookDeliveriesLogSectionProps) {
  const [statusFilter, setStatusFilter] = useState<WebhookDeliveryStatusFilter>("");
  const [webhookId, setWebhookId] = useState<number | "">("");
  const [page, setPage] = useState(1);
  const [limit, setLimit] = useState<number>(25);
  const [expandedKey, setExpandedKey] = useState<string | null>(null);

  const { data: webhooks } = useWebhooks();
  const replay = useReplayWebhookDelivery();

  useEffect(() => {
    setPage(1);
  }, [statusFilter, webhookId, limit]);

  const filters = {
    status: statusFilter || undefined,
    webhookId: webhookId === "" ? undefined : webhookId,
    page,
    limit,
  };

  const { data, isLoading, refetch } = useAccountWebhookDeliveries(filters);
  const items = data?.items ?? [];
  const total = data?.total ?? 0;
  const totalPages = Math.max(1, Math.ceil(total / limit));
  const hasFilters = statusFilter !== "" || webhookId !== "";

  const expanded = expandedKey ? expandedKey.split(":").map(Number) : null;
  const expandedWebhookId = expanded?.[0] ?? null;
  const expandedDeliveryId = expanded?.[1] ?? null;
  const { data: expandedDelivery } = useWebhookDelivery(expandedWebhookId, expandedDeliveryId);

  if (isLoading) return <Spinner className="h-6 w-6" />;

  return (
    <div className={sectionTitle ? "space-y-3" : undefined}>
      {sectionTitle && (
        <p className="text-xs font-semibold uppercase tracking-wide text-gray-600">{sectionTitle}</p>
      )}

      <div className="mb-4 flex flex-col gap-3 sm:flex-row sm:items-end sm:justify-between">
        <div className="flex flex-wrap items-end gap-3">
          <div className="min-w-[10rem]">
            <Label>Webhook</Label>
            <Select
              value={webhookId === "" ? "" : String(webhookId)}
              onChange={(e) => setWebhookId(e.target.value === "" ? "" : Number(e.target.value))}
            >
              <option value="">All</option>
              {(webhooks ?? []).map((w) => (
                <option key={w.id} value={w.id}>
                  {w.name}
                </option>
              ))}
            </Select>
          </div>
        </div>
        <div className="flex flex-wrap gap-2">
          {WEBHOOK_DELIVERY_FILTERS.map((f) => (
            <Button
              key={f.value}
              size="sm"
              variant={statusFilter === f.value ? "primary" : "secondary"}
              onClick={() => setStatusFilter(f.value)}
            >
              {f.label}
            </Button>
          ))}
        </div>
      </div>

      {items.length === 0 ? (
        <EmptyState title={hasFilters ? "No results." : "No webhook deliveries yet."} />
      ) : (
        <>
          <Table>
            <THead>
              <tr>
                <TH>Time</TH>
                <TH>Webhook</TH>
                <TH>Status</TH>
                <TH>Lead</TH>
                <TH />
              </tr>
            </THead>
            <TBody>
              {items.map((d) => {
                const rowKey = `${d.webhook_id}:${d.id}`;
                const isExpanded = expandedKey === rowKey;
                return (
                  <Fragment key={rowKey}>
                    <TR>
                      <TD className="text-xs">{format(new Date(d.created_at), "MMM d, h:mma")}</TD>
                      <TD>
                        <span className="font-medium text-gray-800">{d.webhook_name ?? "—"}</span>
                        {d.webhook_slug && (
                          <span className="ml-1 font-mono text-xs text-gray-400">{d.webhook_slug}</span>
                        )}
                      </TD>
                      <TD>{webhookDeliveryStatusBadge(d.status)}</TD>
                      <TD className="font-mono text-xs">{d.lead_public_id ?? "—"}</TD>
                      <TD>
                        <div className="flex justify-end gap-1">
                          <Button
                            size="sm"
                            variant="secondary"
                            onClick={() => setExpandedKey(isExpanded ? null : rowKey)}
                          >
                            {isExpanded ? "Hide" : "View"}
                          </Button>
                          {canReplay && canReplayDelivery(d) && (
                            <Button
                              size="sm"
                              disabled={replay.isPending}
                              onClick={() =>
                                replay.mutate(
                                  { webhookId: d.webhook_id, deliveryId: d.id },
                                  {
                                    onSuccess: () => {
                                      toast.success("Replayed");
                                      refetch();
                                    },
                                    onError: (e) => toast.error(errorMessage(e)),
                                  }
                                )
                              }
                            >
                              Run again
                            </Button>
                          )}
                        </div>
                      </TD>
                    </TR>
                    {isExpanded && (
                      <tr>
                        <td colSpan={5} className="px-4 py-2">
                          <pre className="max-h-48 overflow-auto rounded-md border border-gray-100 bg-gray-50 p-3 font-mono text-xs">
                            {JSON.stringify(expandedDelivery?.request_payload ?? {}, null, 2)}
                          </pre>
                          {d.error_message && (
                            <p className="mt-1 text-xs text-red-600">{d.error_message}</p>
                          )}
                        </td>
                      </tr>
                    )}
                  </Fragment>
                );
              })}
            </TBody>
          </Table>

          <div className="mt-4 flex flex-wrap items-center justify-between gap-3 text-sm text-gray-500">
            <span>
              {total === 0
                ? "No results"
                : `${(page - 1) * limit + 1}–${Math.min(page * limit, total)} of ${total}`}
            </span>
            <div className="flex items-center gap-3">
              <FilterSelect
                value={limit}
                onChange={(e) => setLimit(Number(e.target.value))}
                className="w-24"
              >
                {PAGE_SIZES.map((n) => (
                  <option key={n} value={n}>
                    {n} / page
                  </option>
                ))}
              </FilterSelect>
              <Button variant="secondary" size="sm" disabled={page <= 1} onClick={() => setPage((p) => p - 1)}>
                Previous
              </Button>
              <span>
                Page {page} of {totalPages}
              </span>
              <Button
                variant="secondary"
                size="sm"
                disabled={page >= totalPages}
                onClick={() => setPage((p) => p + 1)}
              >
                Next
              </Button>
            </div>
          </div>
        </>
      )}
    </div>
  );
}
