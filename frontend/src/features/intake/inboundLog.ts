import type { InboundLogItem, QueueItem, WebhookDelivery } from "@/types";

export interface IntegrationDeliveryRow {
  id: number;
  lead_id?: number | null;
  lead_public_id?: string | null;
  first_name?: string;
  last_name?: string;
  status: string;
  error_message?: string | null;
  created_at: string;
  connection_name: string;
  provider_slug: string;
  attempts: number;
}

export type InboundLogRow =
  | { kind: "source"; direction: "inbound"; item: QueueItem }
  | { kind: "webhook"; direction: "inbound" | "outbound"; item: WebhookDelivery }
  | { kind: "integration"; direction: "outbound"; item: IntegrationDeliveryRow };

export function queueItemToRow(item: QueueItem): InboundLogRow {
  return { kind: "source", direction: "inbound", item };
}

export function webhookDeliveryToRow(item: WebhookDelivery): InboundLogRow {
  return { kind: "webhook", direction: "inbound", item };
}

export function queueItemsToRows(items: QueueItem[]): InboundLogRow[] {
  return items.map(queueItemToRow);
}

export function webhookDeliveriesToRows(items: WebhookDelivery[]): InboundLogRow[] {
  return items.map(webhookDeliveryToRow);
}

function inboundItemToQueueItem(item: InboundLogItem): QueueItem {
  return {
    id: item.id,
    lead_id: item.lead_id ?? 0,
    first_name: item.first_name ?? "",
    last_name: item.last_name ?? "",
    phone: item.phone ?? null,
    source: item.source ?? (item.origin_slug || null),
    raw_payload: item.raw_payload ?? {},
    status: item.status,
    unmapped_keys: item.unmapped_keys,
    created_at: item.created_at,
  };
}

function inboundItemToWebhookDelivery(item: InboundLogItem): WebhookDelivery {
  return {
    id: item.id,
    webhook_id: item.webhook_id ?? 0,
    webhook_name: item.origin || undefined,
    webhook_slug: item.origin_slug || undefined,
    connection_name: item.connection_name || undefined,
    provider_slug: item.provider_slug || undefined,
    lead_id: item.lead_id,
    lead_public_id: item.lead_label || null,
    first_name: item.first_name,
    last_name: item.last_name,
    status: item.status as WebhookDelivery["status"],
    error_message: item.error_message,
    created_at: item.created_at,
  };
}

function inboundItemToIntegrationDelivery(item: InboundLogItem): IntegrationDeliveryRow {
  return {
    id: item.id,
    lead_id: item.lead_id,
    lead_public_id: item.lead_label || null,
    first_name: item.first_name,
    last_name: item.last_name,
    status: item.status,
    error_message: item.error_message,
    created_at: item.created_at,
    connection_name: item.connection_name ?? item.origin,
    provider_slug: item.provider_slug ?? item.origin_slug,
    attempts: item.attempts ?? 0,
  };
}

export function inboundItemToRow(item: InboundLogItem): InboundLogRow {
  const direction = item.direction ?? "inbound";
  if (item.kind === "integration") {
    return { kind: "integration", direction: "outbound", item: inboundItemToIntegrationDelivery(item) };
  }
  if (item.kind === "webhook") {
    return {
      kind: "webhook",
      direction: direction === "outbound" ? "outbound" : "inbound",
      item: inboundItemToWebhookDelivery(item),
    };
  }
  return { kind: "source", direction: "inbound", item: inboundItemToQueueItem(item) };
}

export function inboundItemsToRows(items: InboundLogItem[]): InboundLogRow[] {
  return items.map(inboundItemToRow);
}

/** Merge two paginated lists for buyer "All" view (approximate global sort). */
export function mergeInboundRows(a: InboundLogRow[], b: InboundLogRow[], limit: number): InboundLogRow[] {
  const merged = [...a, ...b].sort(
    (x, y) => new Date(rowCreatedAt(y)).getTime() - new Date(rowCreatedAt(x)).getTime()
  );
  return merged.slice(0, limit);
}

export function rowCreatedAt(row: InboundLogRow): string {
  return row.item.created_at;
}

export function rowDirectionLabel(row: InboundLogRow): string {
  return row.direction === "outbound" ? "Outbound" : "Inbound";
}

export function rowKey(row: InboundLogRow): string {
  if (row.kind === "source") return `source:${row.item.id}`;
  if (row.kind === "integration") return `integration:${row.item.id}`;
  return `webhook:${row.direction}:${row.item.webhook_id}:${row.item.id}`;
}
