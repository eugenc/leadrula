import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { get, patch, post } from "@/lib/api";
import type { Call, CallSettings, CallTarget } from "@/types";

// ── Publisher: per-contract call settings ──────────────────────────

export function useCallSettings(contractId: number | null) {
  return useQuery({
    queryKey: ["call-settings", contractId],
    queryFn: () => get<CallSettings>(`/publisher/contracts/${contractId}/call-settings`),
    enabled: !!contractId,
  });
}

export function useSaveCallSettings() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: ({ contractId, body }: { contractId: number; body: CallSettings }) =>
      patch<CallSettings>(`/publisher/contracts/${contractId}/call-settings`, body),
    onSuccess: (_d, { contractId }) =>
      qc.invalidateQueries({ queryKey: ["call-settings", contractId] }),
  });
}

// ── Per-participation call target (publisher knobs + buyer destination) ──

export function useCallTarget(participationId: number | null, role: "publisher" | "buyer") {
  return useQuery({
    queryKey: ["call-target", role, participationId],
    queryFn: () => get<CallTarget>(`/${role}/participations/${participationId}/call-target`),
    enabled: !!participationId,
  });
}

export function useSaveCallTarget(role: "publisher" | "buyer") {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: ({ participationId, body }: { participationId: number; body: Partial<CallTarget> }) =>
      patch<CallTarget>(`/${role}/participations/${participationId}/call-target`, body),
    onSuccess: (_d, { participationId }) =>
      qc.invalidateQueries({ queryKey: ["call-target", role, participationId] }),
  });
}

// ── Publisher: call log + detail ───────────────────────────────────

export interface CallLogFilters {
  status?: string;
  contract_id?: number;
  billable?: boolean;
  lead_id?: number;
  q?: string;
  limit?: number;
}

function callLogQuery(f: CallLogFilters): string {
  const p = new URLSearchParams();
  if (f.status) p.set("status", f.status);
  if (f.contract_id) p.set("contract_id", String(f.contract_id));
  if (f.lead_id) p.set("lead_id", String(f.lead_id));
  if (f.billable != null) p.set("billable", String(f.billable));
  if (f.q) p.set("q", f.q);
  if (f.limit) p.set("limit", String(f.limit));
  const q = p.toString();
  return q ? `?${q}` : "";
}

export function useCalls(filters: CallLogFilters = {}, options?: { enabled?: boolean }) {
  return useQuery({
    queryKey: ["calls", filters],
    queryFn: async () => {
      const res = await get<{ items: Call[] }>(`/publisher/calls${callLogQuery(filters)}`);
      return res.items ?? [];
    },
    enabled: options?.enabled ?? true,
  });
}

export function useCallDetail(callId: number | null) {
  return useQuery({
    queryKey: ["call", callId],
    queryFn: () => get<Call>(`/publisher/calls/${callId}`),
    enabled: !!callId,
  });
}

// ── Buyer: call log + detail (billable winning calls only) ─────────

export function useBuyerCalls(filters: CallLogFilters = {}, options?: { enabled?: boolean }) {
  return useQuery({
    queryKey: ["buyer-calls", filters],
    queryFn: async () => {
      const res = await get<{ items: Call[] }>(`/buyer/calls${callLogQuery(filters)}`);
      return res.items ?? [];
    },
    enabled: options?.enabled ?? true,
  });
}

export function useBuyerCallDetail(callId: number | null) {
  return useQuery({
    queryKey: ["buyer-call", callId],
    queryFn: () => get<Call>(`/buyer/calls/${callId}`),
    enabled: !!callId,
  });
}

// ── Buyer: call assigned to a lead + disposition ───────────────────

export function useBuyerCallForLead(leadId: number | null) {
  return useQuery({
    queryKey: ["buyer-call-for-lead", leadId],
    queryFn: () => get<Call>(`/buyer/calls/by-lead/${leadId}`),
    enabled: !!leadId,
    retry: false,
  });
}

// useLeadCall resolves the call linked to a lead for the current role: publisher
// sees any call; buyer sees only a billable call assigned to them.
export function useLeadCall(leadId: number | null, accountType?: string) {
  const isPublisher = accountType === "publisher";
  return useQuery({
    queryKey: ["lead-call", accountType, leadId],
    queryFn: async (): Promise<Call | null> => {
      if (isPublisher) {
        const res = await get<{ items: Call[] }>(`/publisher/calls?lead_id=${leadId}`);
        return res.items?.[0] ?? null;
      }
      try {
        return await get<Call>(`/buyer/calls/by-lead/${leadId}`);
      } catch {
        return null;
      }
    },
    enabled: !!leadId && !!accountType,
    retry: false,
  });
}

export function useSetCallDisposition() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: ({ callId, disposition, note }: { callId: number; disposition: string; note?: string }) =>
      post(`/buyer/calls/${callId}/disposition`, { disposition, note: note ?? "" }),
    onSuccess: (_d, { callId }) => {
      qc.invalidateQueries({ queryKey: ["buyer-call-for-lead"] });
      qc.invalidateQueries({ queryKey: ["buyer-call", callId] });
      qc.invalidateQueries({ queryKey: ["buyer-calls"] });
      qc.invalidateQueries({ queryKey: ["lead-call"] });
    },
  });
}
