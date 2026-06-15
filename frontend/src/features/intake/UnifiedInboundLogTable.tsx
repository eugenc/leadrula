import { Fragment, useState } from "react";
import { useRejectQueue } from "@/features/admin/hooks";
import { useIntegrationDelivery, useRetryIntegrationDelivery } from "@/features/intake/hooks";
import { useReplayWebhookDelivery, useWebhookDelivery } from "@/features/webhooks/hooks";
import { Table, THead, TH, TBody, TR, TD } from "@/components/ui/table";
import { Button } from "@/components/ui/button";
import { Spinner, EmptyState, Badge } from "@/components/ui/misc";
import { format } from "date-fns";
import { toast } from "@/store/toastStore";
import { errorMessage } from "@/lib/api";
import { useUIStore } from "@/store/uiStore";
import type { DeliveryRequestLog, IntegrationDeliveryDetail, QueueItem } from "@/types";
import { QueueItemDrawer, RouteDialog } from "@/pages/publisher/intakeShared";
import {
  LogPagination,
  LogLeadLink,
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
  mappingSource?: "publisher" | "buyer";
  canReplayWebhooks?: boolean;
  integrationLogMode?: boolean;
  onPageChange: (page: number) => void;
  onLimitChange: (limit: number) => void;
  onWebhookReplayed?: () => void;
}

function formatJsonBlock(value: unknown): string {
  if (value == null) return "";
  if (typeof value === "string") {
    const trimmed = value.trim();
    if (!trimmed) return "";
    try {
      return JSON.stringify(JSON.parse(trimmed), null, 2);
    } catch {
      return value;
    }
  }
  return JSON.stringify(value, null, 2);
}

function parseDeliveryRequestLog(body: unknown): DeliveryRequestLog | null {
  if (!body || typeof body !== "object") return null;
  const o = body as Record<string, unknown>;
  if (!o.mapped || typeof o.mapped !== "object" || !o.http || typeof o.http !== "object") return null;
  return body as DeliveryRequestLog;
}

function MappedPayloadPanel({ mapped }: { mapped: Record<string, string> }) {
  const entries = Object.entries(mapped).sort(([a], [b]) => a.localeCompare(b));
  if (entries.length === 0) {
    return <p className="text-xs text-gray-400">No mapped fields.</p>;
  }
  return (
    <div className="space-y-1 rounded-md border border-gray-100 bg-gray-50 p-3">
      {entries.map(([key, value]) => (
        <div key={key} className="text-xs text-gray-700">
          <span className="font-mono font-medium text-gray-800">{key}</span>
          <span className="ml-2 font-mono text-gray-600">{formatJsonBlock(value)}</span>
        </div>
      ))}
    </div>
  );
}

function HTTPRequestPanel({ http }: { http: DeliveryRequestLog["http"] }) {
  return (
    <div className="space-y-2 rounded-md border border-gray-100 bg-gray-50 p-3">
      <p className="font-mono text-xs text-gray-800">
        {http.method} {http.url}
      </p>
      {http.headers && Object.keys(http.headers).length > 0 && (
        <div className="space-y-1">
          <p className="text-xs font-medium text-gray-500">Headers</p>
          {Object.entries(http.headers).map(([key, value]) => (
            <div key={key} className="font-mono text-xs text-gray-600">
              {key}: {value}
            </div>
          ))}
        </div>
      )}
      {http.body != null && (
        <pre className="max-h-48 overflow-auto font-mono text-xs text-gray-700">{formatJsonBlock(http.body)}</pre>
      )}
    </div>
  );
}

function IntegrationDeliveryExpand({ detail }: { detail: IntegrationDeliveryDetail | undefined }) {
  if (!detail) {
    return <Spinner className="h-4 w-4" />;
  }

  return (
    <div className="space-y-3">
      {detail.external_id && (
        <p className="text-xs text-gray-600">
          CRM ID: <span className="font-mono text-gray-800">{detail.external_id}</span>
        </p>
      )}
      {detail.last_error && <p className="text-xs text-red-600">{detail.last_error}</p>}
      {detail.attempts.length === 0 ? (
        <p className="text-xs text-gray-400">Not delivered yet.</p>
      ) : (
        detail.attempts.map((attempt) => {
          const reqLog = parseDeliveryRequestLog(attempt.request_body);
          return (
            <div key={attempt.attempt_number} className="space-y-2">
              <p className="text-xs font-medium text-gray-500">
                Attempt {attempt.attempt_number} — {attempt.status}
                {attempt.http_status != null && attempt.http_status > 0 ? ` — HTTP ${attempt.http_status}` : ""}
                {attempt.duration_ms != null ? ` — ${attempt.duration_ms}ms` : ""}
              </p>
              {attempt.error && <p className="text-xs text-red-600">{attempt.error}</p>}
              {reqLog ? (
                <>
                  <div>
                    <p className="mb-1 text-xs font-medium text-gray-500">Outbound payload (mapped)</p>
                    <MappedPayloadPanel mapped={reqLog.mapped} />
                  </div>
                  <div>
                    <p className="mb-1 text-xs font-medium text-gray-500">HTTP request (actual)</p>
                    <HTTPRequestPanel http={reqLog.http} />
                  </div>
                </>
              ) : (
                <p className="text-xs text-gray-400">Request not recorded — resend to capture mapped + HTTP payload.</p>
              )}
              {attempt.response_body ? (
                <>
                  <p className="mb-1 text-xs text-gray-400">Response from server</p>
                  <pre className="max-h-48 overflow-auto rounded-md border border-gray-100 bg-gray-50 p-3 font-mono text-xs">
                    {formatJsonBlock(attempt.response_body)}
                  </pre>
                </>
              ) : (
                !attempt.error && <p className="text-xs text-gray-400">No response body recorded.</p>
              )}
            </div>
          );
        })
      )}
    </div>
  );
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
  mappingSource = "publisher",
  canReplayWebhooks = false,
  integrationLogMode = false,
  onPageChange,
  onLimitChange,
  onWebhookReplayed,
}: UnifiedInboundLogTableProps) {
  const reject = useRejectQueue();
  const replay = useReplayWebhookDelivery();
  const retryIntegration = useRetryIntegrationDelivery(mappingSource);
  const openDetail = useUIStore((s) => s.openDetail);
  const [drawerItem, setDrawerItem] = useState<QueueItem | null>(null);
  const [routing, setRouting] = useState<QueueItem | null>(null);
  const [expandedKey, setExpandedKey] = useState<string | null>(null);

  const expandedParts = expandedKey?.split(":") ?? [];
  const expandedKind = expandedParts[0] ?? "";
  const expandedWebhookId = expandedKind === "webhook" && expandedParts[1] ? Number(expandedParts[1]) : null;
  const expandedDeliveryId = expandedKind === "webhook" && expandedParts[2] ? Number(expandedParts[2]) : null;
  const expandedIntegrationId = expandedKind === "integration" && expandedParts[1] ? Number(expandedParts[1]) : null;

  const { data: expandedDelivery } = useWebhookDelivery(expandedWebhookId, expandedDeliveryId);
  const { data: expandedIntegration } = useIntegrationDelivery(expandedIntegrationId, mappingSource);

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
                  <TD>
                    <LogLeadLink
                      leadId={q.lead_id || null}
                      firstName={q.first_name}
                      lastName={q.last_name}
                      onClick={openDetail}
                    />
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
              const expandKey = `integration:${d.id}`;
              const isExpanded = expandedKey === expandKey;
              return (
                <Fragment key={key}>
                  <TR>
                    <TD className="text-xs">{format(new Date(d.created_at), "MMM d, h:mma")}</TD>
                    <TD className="text-xs text-gray-600">Integration</TD>
                    <TD className="text-xs text-gray-600">{direction}</TD>
                    <TD>
                      <span className="font-medium text-gray-800">{d.connection_name || "—"}</span>
                      {d.provider_slug && (
                        <span className="ml-1 font-mono text-xs text-gray-400">{d.provider_slug}</span>
                      )}
                    </TD>
                    <TD>
                      <LogLeadLink
                        leadId={d.lead_id}
                        firstName={d.first_name}
                        lastName={d.last_name}
                        fallback={d.lead_public_id}
                        onClick={openDetail}
                      />
                    </TD>
                    <TD>{integrationDeliveryStatusText(d.status)}</TD>
                    <TD>
                      <div className="flex shrink-0 justify-end gap-1">
                        <Button
                          size="sm"
                          variant="secondary"
                          className="shrink-0 whitespace-nowrap"
                          onClick={() => setExpandedKey(isExpanded ? null : expandKey)}
                        >
                          {isExpanded ? "Hide" : "View"}
                        </Button>
                        {canReplayWebhooks && (
                          <Button
                            size="sm"
                            className="shrink-0 whitespace-nowrap"
                            disabled={retryIntegration.isPending}
                            onClick={() =>
                              retryIntegration.mutate(d.id, {
                                onSuccess: () => {
                                  toast.success("Resent");
                                  onWebhookReplayed?.();
                                },
                                onError: (e) => toast.error(errorMessage(e)),
                              })
                            }
                          >
                            Resend
                          </Button>
                        )}
                      </div>
                    </TD>
                  </TR>
                  {isExpanded && (
                    <tr>
                      <td colSpan={7} className="px-4 py-2">
                        <IntegrationDeliveryExpand detail={expandedIntegration} />
                        {d.error_message && !expandedIntegration?.last_error && (
                          <p className="mt-1 text-xs text-red-600">{d.error_message}</p>
                        )}
                      </td>
                    </tr>
                  )}
                </Fragment>
              );
            }

            const d = row.item;
            const isInbound = row.direction === "inbound";
            const isOutbound = row.direction === "outbound";
            const expandKey = isOutbound ? `integration:${d.id}` : `webhook:${d.webhook_id}:${d.id}`;
            const isExpanded = expandedKey === expandKey;
            const statusNode = isInbound
              ? webhookDeliveryStatusText(d.status)
              : integrationDeliveryStatusText(d.status);
            const originName = d.connection_name || d.webhook_name || "—";
            const originSlug = d.provider_slug || d.webhook_slug;
            const typeLabel = integrationLogMode && isInbound && d.connection_name ? "Integration" : "Webhook";

            return (
              <Fragment key={key}>
                <TR>
                  <TD className="text-xs">{format(new Date(d.created_at), "MMM d, h:mma")}</TD>
                  <TD className="text-xs text-gray-600">{typeLabel}</TD>
                  <TD className="text-xs text-gray-600">{direction}</TD>
                  <TD>
                    <span className="font-medium text-gray-800">{originName}</span>
                    {originSlug && (
                      <span className="ml-1 font-mono text-xs text-gray-400">{originSlug}</span>
                    )}
                  </TD>
                  <TD>
                    <LogLeadLink
                      leadId={d.lead_id}
                      firstName={d.first_name}
                      lastName={d.last_name}
                      fallback={d.lead_public_id}
                      onClick={openDetail}
                    />
                  </TD>
                  <TD>{statusNode}</TD>
                  <TD>
                    <div className="flex shrink-0 justify-end gap-1">
                      {(isInbound || isOutbound) && (
                        <Button
                          size="sm"
                          variant="secondary"
                          className="shrink-0 whitespace-nowrap"
                          onClick={() => setExpandedKey(isExpanded ? null : expandKey)}
                        >
                          {isExpanded ? "Hide" : "View"}
                        </Button>
                      )}
                      {isInbound && canReplayWebhooks && canReplayDelivery(d) && (
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
                      {isOutbound && canReplayWebhooks && (
                        <Button
                          size="sm"
                          className="shrink-0 whitespace-nowrap"
                          disabled={retryIntegration.isPending}
                          onClick={() =>
                            retryIntegration.mutate(d.id, {
                              onSuccess: () => {
                                toast.success("Resent");
                                onWebhookReplayed?.();
                              },
                              onError: (e) => toast.error(errorMessage(e)),
                            })
                          }
                        >
                          Resend
                        </Button>
                      )}
                    </div>
                  </TD>
                </TR>
                {isInbound && isExpanded && (
                  <tr>
                    <td colSpan={7} className="px-4 py-2">
                      <p className="mb-1 text-xs font-medium text-gray-500">
                        {integrationLogMode && d.connection_name ? "Received from CRM" : "Payload"}
                      </p>
                      <pre className="max-h-48 overflow-auto rounded-md border border-gray-100 bg-gray-50 p-3 font-mono text-xs">
                        {JSON.stringify(expandedDelivery?.request_payload ?? {}, null, 2)}
                      </pre>
                      {d.error_message && <p className="mt-1 text-xs text-red-600">{d.error_message}</p>}
                    </td>
                  </tr>
                )}
                {isOutbound && isExpanded && (
                  <tr>
                    <td colSpan={7} className="px-4 py-2">
                      <IntegrationDeliveryExpand detail={expandedIntegration} />
                      {d.error_message && !expandedIntegration?.last_error && (
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

      <LogPagination
        page={page}
        limit={limit}
        total={total}
        onPageChange={onPageChange}
        onLimitChange={onLimitChange}
      />

      {!readOnly && routing && mappingSource === "publisher" && (
        <RouteDialog item={routing} onClose={() => setRouting(null)} />
      )}
      {drawerItem && (
        <QueueItemDrawer
          item={drawerItem}
          readOnly={readOnly}
          mappingSource={mappingSource}
          onClose={() => setDrawerItem(null)}
          onUpdated={readOnly ? undefined : setDrawerItem}
          onRoute={
            readOnly || mappingSource === "buyer"
              ? undefined
              : () => {
                  setRouting(drawerItem);
                  setDrawerItem(null);
                }
          }
          onReject={
            readOnly || mappingSource === "buyer"
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
