import { Fragment, useEffect, useState } from "react";
import { useRejectQueue } from "@/features/admin/hooks";
import { diagnoseGhlInboundStageSyncPayload } from "@/features/integrations/ghlStageSyncDiagnosis";
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
  routePipelineDestinationLabel,
  routeTriggerLabel,
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
  loadError?: string | null;
  hasFilters: boolean;
  readOnly?: boolean;
  mappingSource?: "publisher" | "buyer";
  canReplayWebhooks?: boolean;
  integrationLogMode?: boolean;
  initialExpandedKey?: string | null;
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

function JsonPayloadPanel({ payload, label = "Payload" }: { payload: unknown; label?: string }) {
  const text = formatJsonBlock(payload);
  if (!text) {
    return <p className="text-xs text-gray-400">No payload recorded.</p>;
  }
  return (
    <>
      <p className="mb-1 text-xs font-medium text-gray-500">{label}</p>
      <pre className="max-h-48 overflow-auto rounded-md border border-gray-100 bg-gray-50 p-3 font-mono text-xs text-gray-700">
        {text}
      </pre>
    </>
  );
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
      <p className="break-all font-mono text-xs text-gray-800">
        {http.method} {http.url}
      </p>
      {http.headers && Object.keys(http.headers).length > 0 && (
        <div className="space-y-1">
          <p className="text-xs font-medium text-gray-500">Headers</p>
          {Object.entries(http.headers).map(([key, value]) => (
            <div key={key} className="break-all font-mono text-xs text-gray-600">
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
        <div className="space-y-2">
          <p className="text-xs text-gray-400">Not delivered yet.</p>
          <JsonPayloadPanel payload={detail.payload} label="Queued payload" />
        </div>
      ) : (
        detail.attempts.map((attempt) => {
          const reqLog = parseDeliveryRequestLog(attempt.request_body);
          return (
            <div key={attempt.attempt_number} className="space-y-2">
              <p className="text-xs font-medium text-gray-500">
                Attempt {attempt.attempt_number}
                {attempt.created_at ? ` — ${format(new Date(attempt.created_at), "MMM d, h:mma")}` : ""}
                — {attempt.status}
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

function rowExpandKey(row: InboundLogRow): string {
  if (row.kind === "source") return `source:${row.item.id}`;
  if (row.kind === "integration") return `integration:${row.item.id}`;
  if (row.kind === "route") return `route:${row.item.id}`;
  const isOutbound = row.direction === "outbound";
  return isOutbound
    ? `integration:${row.item.id}`
    : `webhook:${row.item.webhook_id}:${row.item.id}`;
}

export function UnifiedInboundLogTable({
  rows,
  total,
  page,
  limit,
  isLoading,
  emptyTitle,
  loadError,
  hasFilters,
  readOnly = false,
  mappingSource = "publisher",
  canReplayWebhooks = false,
  integrationLogMode = false,
  initialExpandedKey,
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

  useEffect(() => {
    if (!initialExpandedKey || isLoading || rows.length === 0) return;
    if (rows.some((row) => rowExpandKey(row) === initialExpandedKey)) {
      setExpandedKey(initialExpandedKey);
    }
  }, [initialExpandedKey, isLoading, rows]);

  if (isLoading) return <Spinner className="h-6 w-6" />;

  if (loadError) {
    return <EmptyState title="Could not load logs." subtitle={loadError} />;
  }

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
              const expandKey = `source:${q.id}`;
              const isExpanded = expandedKey === expandKey;
              return (
                <Fragment key={key}>
                  <TR>
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
                        fallback={q.phone}
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
                      <div className="flex shrink-0 justify-end gap-2">
                        <Button
                          size="sm"
                          variant="secondary"
                          className="shrink-0 whitespace-nowrap"
                          onClick={() => setExpandedKey(isExpanded ? null : expandKey)}
                        >
                          {isExpanded ? "Hide" : "View"}
                        </Button>
                        {!readOnly && unmapped > 0 && (
                          <Button size="sm" variant="secondary" className="shrink-0 whitespace-nowrap" onClick={() => setDrawerItem(q)}>
                            Map
                          </Button>
                        )}
                      </div>
                    </TD>
                  </TR>
                  {isExpanded && (
                    <tr>
                      <td colSpan={7} className="min-w-0 px-4 py-2">
                        <JsonPayloadPanel payload={q.raw_payload} />
                      </td>
                    </tr>
                  )}
                </Fragment>
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
                      <td colSpan={7} className="min-w-0 px-4 py-2">
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

            if (row.kind === "route") {
              const r = row.item;
              const expandKey = `route:${r.id}`;
              const isExpanded = expandedKey === expandKey;
              return (
                <Fragment key={key}>
                  <TR>
                    <TD className="text-xs">{format(new Date(r.created_at), "MMM d, h:mma")}</TD>
                    <TD className="text-xs text-gray-600">Route</TD>
                    <TD className="text-xs text-gray-600">{direction}</TD>
                    <TD>
                      <span className="font-medium text-gray-800">{r.route_name}</span>
                      {r.target_account_name && (
                        <span className="ml-1 text-xs text-gray-500">→ {r.target_account_name}</span>
                      )}
                    </TD>
                    <TD>
                      <LogLeadLink
                        leadId={r.lead_id}
                        firstName={r.first_name}
                        lastName={r.last_name}
                        onClick={openDetail}
                      />
                    </TD>
                    <TD>
                      <span className="text-sm text-gray-700 capitalize">{r.status}</span>
                    </TD>
                    <TD>
                      <Button
                        size="sm"
                        variant="secondary"
                        className="shrink-0 whitespace-nowrap"
                        onClick={() => setExpandedKey(isExpanded ? null : expandKey)}
                      >
                        {isExpanded ? "Hide" : "View"}
                      </Button>
                    </TD>
                  </TR>
                  {isExpanded && (
                    <tr>
                      <td colSpan={7} className="min-w-0 px-4 py-2">
                        <div className="space-y-3">
                          <div className="space-y-2 rounded-md border border-gray-100 bg-gray-50 p-3 text-xs text-gray-700">
                            <div>
                              <span className="font-medium text-gray-500">Trigger: </span>
                              {routeTriggerLabel(r.trigger_type)}
                              {r.trigger_label ? ` (${r.trigger_label})` : ""}
                            </div>
                            <div>
                              <span className="font-medium text-gray-500">Destination: </span>
                              {routePipelineDestinationLabel(
                                r.destination,
                                r.delivery,
                                r.target_pipeline_name,
                                r.target_stage_name
                              )}
                            </div>
                            {r.target_account_name && (
                              <div>
                                <span className="font-medium text-gray-500">Buyer: </span>
                                {r.target_account_name}
                              </div>
                            )}
                            {r.branch_position != null && r.branch_position > 0 && (
                              <div>
                                <span className="font-medium text-gray-500">Branch: </span>
                                {r.branch_position}
                              </div>
                            )}
                            {r.route_name && (
                              <div>
                                <span className="font-medium text-gray-500">Route Name: </span>
                                {r.route_name}
                              </div>
                            )}
                            {r.route_id != null && (
                              <div>
                                <span className="font-medium text-gray-500">Route ID: </span>
                                {r.route_id}
                              </div>
                            )}
                            {r.error_message && (
                              <p className="text-red-600">{r.error_message}</p>
                            )}
                          </div>
                          <JsonPayloadPanel payload={r.raw_payload} />
                        </div>
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
                    <td colSpan={7} className="min-w-0 px-4 py-2">
                      <JsonPayloadPanel
                        payload={expandedDelivery?.request_payload}
                        label={integrationLogMode && d.connection_name ? "Received from CRM" : "Payload"}
                      />
                      {integrationLogMode && d.provider_slug === "ghl" && (() => {
                        const diag = diagnoseGhlInboundStageSyncPayload(expandedDelivery?.request_payload);
                        return (
                          <p className={`mt-2 text-xs ${diag.status === "warning" ? "text-amber-700" : "text-gray-600"}`}>
                            <span className="font-medium">Stage sync: </span>
                            {diag.message}
                          </p>
                        );
                      })()}
                      {d.error_message && <p className="mt-1 text-xs text-red-600">{d.error_message}</p>}
                    </td>
                  </tr>
                )}
                {isOutbound && isExpanded && (
                  <tr>
                    <td colSpan={7} className="min-w-0 px-4 py-2">
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
