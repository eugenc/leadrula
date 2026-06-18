import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { get, post } from "@/lib/api";
import type { InboundLogListResponse, IntegrationDeliveryDetail } from "@/types";

export interface InboundLogFilters {
  type?: "all" | "source" | "webhook" | "integration" | "route";
  status?: string;
  page?: number;
  limit?: number;
  q?: string;
  source?: string;
  webhookId?: number;
  leadId?: number;
}

function inboundLogQueryString(filters: InboundLogFilters): string {
  const qs = new URLSearchParams();
  if (filters.type) qs.set("type", filters.type);
  Object.entries(filters).forEach(([k, v]) => {
    if (k === "type") return;
    if (k === "webhookId" && v) {
      qs.set("webhook_id", String(v));
      return;
    }
    if (k === "leadId" && v) {
      qs.set("lead_id", String(v));
      return;
    }
    if (k === "q" && v) {
      qs.set("q", String(v));
      return;
    }
    if (v !== undefined && v !== "" && v !== 0) qs.set(k, String(v));
  });
  return qs.toString();
}

export function useInboundLog(
  filters: InboundLogFilters = { type: "all", page: 1, limit: 25 },
  enabled = true,
  source: "publisher" | "buyer" = "publisher"
) {
  const q = inboundLogQueryString(filters);
  const path = source === "buyer" ? "/buyer/inbound-log" : "/publisher/inbound-log";
  return useQuery({
    queryKey: ["inbound-log", source, filters],
    queryFn: () => get<InboundLogListResponse>(`${path}?${q}`),
    enabled,
    refetchInterval: enabled ? 5000 : false,
  });
}

export function useIntegrationDelivery(id: number | null, source: "publisher" | "buyer" = "publisher") {
  const base = source === "buyer" ? "/buyer" : "/publisher";
  return useQuery({
    queryKey: ["integration-delivery", source, id],
    queryFn: () => get<IntegrationDeliveryDetail>(`${base}/integration-deliveries/${id}`),
    enabled: !!id,
  });
}

export function useRetryIntegrationDelivery(source: "publisher" | "buyer" = "publisher") {
  const qc = useQueryClient();
  const base = source === "buyer" ? "/buyer" : "/publisher";
  return useMutation({
    mutationFn: (deliveryId: number) =>
      post<{ ok: boolean }>(`${base}/integration-deliveries/${deliveryId}/retry`, {}),
    onSuccess: (_, deliveryId) => {
      qc.invalidateQueries({ queryKey: ["inbound-log", source] });
      qc.invalidateQueries({ queryKey: ["integration-delivery", source, deliveryId] });
    },
  });
}
