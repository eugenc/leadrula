import { useQuery } from "@tanstack/react-query";
import { get } from "@/lib/api";
import type { InboundLogListResponse } from "@/types";

export interface InboundLogFilters {
  type?: "all" | "source" | "webhook" | "integration";
  status?: string;
  page?: number;
  limit?: number;
  q?: string;
  source?: string;
  webhookId?: number;
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
  enabled = true
) {
  const q = inboundLogQueryString(filters);
  return useQuery({
    queryKey: ["inbound-log", filters],
    queryFn: () => get<InboundLogListResponse>(`/publisher/inbound-log?${q}`),
    enabled,
    refetchInterval: enabled ? 5000 : false,
  });
}
