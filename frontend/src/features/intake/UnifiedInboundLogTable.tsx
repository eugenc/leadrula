import { Fragment, useState } from "react";
import { useRejectQueue } from "@/features/admin/hooks";
import { useReplayWebhookDelivery, useWebhookDelivery } from "@/features/webhooks/hooks";
import { Table, THead, TH, TBody, TR, TD } from "@/components/ui/table";
import { Button } from "@/components/ui/button";
import { Spinner, EmptyState, Badge } from "@/components/ui/misc";
import { format } from "date-fns";
import { toast } from "@/store/toastStore";
import { errorMessage } from "@/lib/api";
import type { QueueItem } from "@/types";
import { QueueItemDrawer, RouteDialog } from "@/pages/publisher/intakeShared";
import {
  LogPagination,
  canReplayDelivery,
  integrationDeliveryStatusText,
  statusText,
  webhookDeliveryStatusText,
} from "./logShared";
import { type InboundLogRow, rowDirectionLabel, rowKey } from "./inboundLog";

interface UnifiedInboundLogTableProps {
  rows: InboundLogRow[];
  total: number;
  page: number;
  limit: number;
  isLoading: boolean;
  emptyTitle: string;
  hasFilters: boolean;
  readOnly?: boolean;
  canReplayWebhooks?: boolean;
  onPageChange: (page: number) => void;
  onLimitChange: (limit: number) => void;
  onWebhookReplayed?: () => void;
}

export function UnifiedInboundLogTable({
  rows,
  total,
  page,
  limit,
  isLoading,
  emptyTitle,
  hasFilters,
  readOnly = false,
  canReplayWebhooks = false,
  onPageChange,
  onLimitChange,
  onWebhookReplayed,
}: UnifiedInboundLogTableProps) {
  const reject = useRejectQueue();
  const replay = useReplayWebhookDelivery();
  const [drawerItem, setDrawerItem] = useState<QueueItem | null>(null);
  const [routing, setRouting] = useState<QueueItem | null>(null);
  const [expandedKey, setExpandedKey] = useState<string | null>(null);

  const expandedParts = expandedKey?.split(":") ?? [];
  const expandedWebhookId = expandedParts[1] ? Number(expandedParts[1]) : null;
  const expandedDeliveryId = expandedParts[2] ? Number(expandedParts[2]) : null;
  const { data: expandedDelivery } = useWebhookDelivery(expandedWebhookId, expandedDeliveryId);

  if (isLoading) return <Spinner className="h-6 w-6" />;

  if (rows.length === 0) {
    return <EmptyState title={hasFilters ? "No results." : emptyTitle} />;
  }

  return (
    <>
      <Table>
        <THead>
          <tr>
            <TH>Time</TH>
            <TH>Type</TH>
            <TH>Direction</TH>
            <TH>Origin</TH>
            <TH>Lead</TH>
            <TH>Status</TH>
            <TH className="text-right whitespace-nowrap" />
          </tr>
        </THead>
        <TBody>
          {rows.map((row) => {
            const key = rowKey(row);
            const direction = rowDirectionLabel(row);

            if (row.kind === "source") {
              const q = row.item;
              const unmapped = q.unmapped_keys?.length ?? 0;
              return (
                <TR key={key} onClick={() => setDrawerItem(q)} className="cursor-pointer">
                  <TD className="text-xs">{format(new Date(q.created_at), "MMM d, h:mma")}</TD>
                  <TD className="text-xs text-gray-600">Source</TD>
                  <TD className="text-xs text-gray-600">{direction}</TD>
                  <TD>
                    <span className="font-mono text-xs text-gray-600">{q.source ?? "—"}</span>
                  </TD>
                  <TD className="font-medium text-gray-800">
                    {q.first_name} {q.last_name}
                    {unmapped > 0 && (
                      <Badge variant="pending" className="ml-2">
                        {unmapped} unmapped
                      </Badge>
                    )}
                  </TD>
                  <TD>{statusText(q.status)}</TD>
                  <TD>
                    {!readOnly && (
                      <div className="flex shrink-0 justify-end gap-2" onClick={(e) => e.stopPropagation()}>
                        {unmapped > 0 && (
                          <Button size="sm" variant="secondary" className="shrink-0 whitespace-nowrap" onClick={() => setDrawerItem(q)}>
                            Map
                          </Button>
                        )}
                      </div>
                    )}
                  </TD>
                </TR>
              );
            }

            if (row.kind === "integration") {
              const d = row.item;
              return (
                <TR key={key}>
                  <TD className="text-xs">{format(new Date(d.created_at), "MMM d, h:mma")}</TD>
                  <TD className="text-xs text-gray-600">Integration</TD>
                  <TD className="text-xs text-gray-600">{direction}</TD>
                  <TD>
                    <span className="font-medium text-gray-800">{d.connection_name || "—"}</span>
                    {d.provider_slug && (
                      <span className="ml-1 font-mono text-xs text-gray-400">{d.provider_slug}</span>
                    )}
                  </TD>
                  <TD className="font-mono text-xs">{d.lead_public_id ?? "—"}</TD>
                  <TD>{integrationDeliveryStatusText(d.status)}</TD>
                  <TD />
                </TR>
              );
            }

            const d = row.item;
            const isInbound = row.direction === "inbound";
            const expandKey = `webhook:${d.webhook_id}:${d.id}`;
            const isExpanded = expandedKey === expandKey;
            const statusNode = isInbound
              ? webhookDeliveryStatusText(d.status)
              : integrationDeliveryStatusText(d.status);

            return (
              <Fragment key={key}>
                <TR>
                  <TD className="text-xs">{format(new Date(d.created_at), "MMM d, h:mma")}</TD>
                  <TD className="text-xs text-gray-600">Webhook</TD>
                  <TD className="text-xs text-gray-600">{direction}</TD>
                  <TD>
                    <span className="font-medium text-gray-800">{d.webhook_name ?? "—"}</span>
                    {d.webhook_slug && (
                      <span className="ml-1 font-mono text-xs text-gray-400">{d.webhook_slug}</span>
                    )}
                  </TD>
                  <TD className="font-mono text-xs">{d.lead_public_id ?? "—"}</TD>
                  <TD>{statusNode}</TD>
                  <TD>
                    {isInbound && (
                      <div className="flex shrink-0 justify-end gap-1">
                        <Button
                          size="sm"
                          variant="secondary"
                          className="shrink-0 whitespace-nowrap"
                          onClick={() => setExpandedKey(isExpanded ? null : expandKey)}
                        >
                          {isExpanded ? "Hide" : "View"}
                        </Button>
                        {canReplayWebhooks && canReplayDelivery(d) && (
                          <Button
                            size="sm"
                            className="shrink-0 whitespace-nowrap"
                            disabled={replay.isPending}
                            onClick={() =>
                              replay.mutate(
                                { webhookId: d.webhook_id, deliveryId: d.id },
                                {
                                  onSuccess: () => {
                                    toast.success("Replayed");
                                    onWebhookReplayed?.();
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
                    )}
                  </TD>
                </TR>
                {isInbound && isExpanded && (
                  <tr>
                    <td colSpan={7} className="px-4 py-2">
                      <pre className="max-h-48 overflow-auto rounded-md border border-gray-100 bg-gray-50 p-3 font-mono text-xs">
                        {JSON.stringify(expandedDelivery?.request_payload ?? {}, null, 2)}
                      </pre>
                      {d.error_message && <p className="mt-1 text-xs text-red-600">{d.error_message}</p>}
                    </td>
                  </tr>
                )}
              </Fragment>
            );
          })}
        </TBody>
      </Table>

      <LogPagination
        page={page}
        limit={limit}
        total={total}
        onPageChange={onPageChange}
        onLimitChange={onLimitChange}
      />

      {!readOnly && routing && <RouteDialog item={routing} onClose={() => setRouting(null)} />}
      {drawerItem && (
        <QueueItemDrawer
          item={drawerItem}
          readOnly={readOnly}
          onClose={() => setDrawerItem(null)}
          onUpdated={readOnly ? undefined : setDrawerItem}
          onRoute={
            readOnly
              ? undefined
              : () => {
                  setRouting(drawerItem);
                  setDrawerItem(null);
                }
          }
          onReject={
            readOnly
              ? undefined
              : () => {
                  reject.mutate(drawerItem.id, {
                    onSuccess: () => {
                      toast.success("Lead rejected");
                      setDrawerItem(null);
                    },
                    onError: (e) => toast.error(errorMessage(e)),
                  });
                }
          }
        />
      )}
    </>
  );
}
