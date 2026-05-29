import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { get, ns, patch, post, del } from "@/lib/api";
import type {
  Lead,
  Note,
  Pipeline,
  Stage,
  StageHistoryEntry,
  CustomField,
  DisqReason,
  UserRow,
} from "@/types";

export function usePipelines() {
  return useQuery({ queryKey: ["pipelines"], queryFn: () => get<Pipeline[]>(`${ns()}/pipelines`) });
}

export function useStages(pipelineId: number | undefined) {
  return useQuery({
    queryKey: ["stages", pipelineId],
    queryFn: () => get<Stage[]>(`${ns()}/pipelines/${pipelineId}/stages`),
    enabled: !!pipelineId,
  });
}

export interface LeadFilters {
  pipeline_id?: number;
  stage_id?: number;
  status?: string;
  campaign?: string;
  assigned?: number;
}

export function useLeads(filters: LeadFilters = {}) {
  const qs = new URLSearchParams();
  Object.entries(filters).forEach(([k, v]) => {
    if (v !== undefined && v !== "" && v !== 0) qs.set(k, String(v));
  });
  const q = qs.toString();
  return useQuery({
    queryKey: ["leads", filters],
    queryFn: () => get<Lead[]>(`${ns()}/leads${q ? `?${q}` : ""}`),
  });
}

export function useLead(id: number | null) {
  return useQuery({
    queryKey: ["lead", id],
    queryFn: () => get<Lead>(`${ns()}/leads/${id}`),
    enabled: !!id,
  });
}

export interface StageChangePayload {
  stage_id: number;
  action_at?: string | null;
  disqualification_reason_id?: number | null;
}

export function useChangeStage() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: ({ leadId, payload }: { leadId: number; payload: StageChangePayload }) =>
      patch<Lead>(`${ns()}/leads/${leadId}/stage`, payload),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ["leads"] });
      qc.invalidateQueries({ queryKey: ["lead"] });
    },
  });
}

export function useUpdateLead() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: ({ leadId, body }: { leadId: number; body: Record<string, unknown> }) =>
      patch<Lead>(`${ns()}/leads/${leadId}`, body),
    onSuccess: (_d, v) => {
      qc.invalidateQueries({ queryKey: ["leads"] });
      qc.invalidateQueries({ queryKey: ["lead", v.leadId] });
    },
  });
}

export function useNotes(leadId: number | null) {
  return useQuery({
    queryKey: ["notes", leadId],
    queryFn: () => get<Note[]>(`${ns()}/leads/${leadId}/notes`),
    enabled: !!leadId,
  });
}

export function useAddNote() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: ({ leadId, body }: { leadId: number; body: string }) =>
      post<Note>(`${ns()}/leads/${leadId}/notes`, { body }),
    onSuccess: (_d, v) => qc.invalidateQueries({ queryKey: ["notes", v.leadId] }),
  });
}

export function useStageHistory(leadId: number | null) {
  return useQuery({
    queryKey: ["stage-history", leadId],
    queryFn: () => get<StageHistoryEntry[]>(`${ns()}/leads/${leadId}/stage-history`),
    enabled: !!leadId,
  });
}

export function useCreateLead() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (body: Record<string, unknown>) => post<Lead>(`${ns()}/leads`, body),
    onSuccess: () => qc.invalidateQueries({ queryKey: ["leads"] }),
  });
}

export function useRedistribute() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: ({ leadId, contractId }: { leadId: number; contractId: number }) =>
      post<Lead>(`${ns()}/leads/${leadId}/redistribute`, { contract_id: contractId }),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ["leads"] });
      qc.invalidateQueries({ queryKey: ["lead"] });
    },
  });
}

export function useFollowers(leadId: number | null) {
  return useQuery({
    queryKey: ["followers", leadId],
    queryFn: () => get<number[]>(`${ns()}/leads/${leadId}/followers`).catch(() => [] as number[]),
    enabled: !!leadId,
  });
}

export function useToggleFollow() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: ({ leadId, userId, follow }: { leadId: number; userId: number; follow: boolean }) =>
      follow
        ? post(`${ns()}/leads/${leadId}/followers`, { user_id: userId })
        : del(`${ns()}/leads/${leadId}/followers/${userId}`),
    onSuccess: (_d, v) => qc.invalidateQueries({ queryKey: ["followers", v.leadId] }),
  });
}

// shared reference data
export function useCustomFields() {
  return useQuery({ queryKey: ["custom-fields"], queryFn: () => get<CustomField[]>(`${ns()}/custom-fields`) });
}

export function useDisqReasons() {
  return useQuery({
    queryKey: ["disq-reasons"],
    queryFn: () => get<DisqReason[]>(`${ns()}/disqualification-reasons`),
  });
}

export function useUsers() {
  return useQuery({ queryKey: ["users"], queryFn: () => get<UserRow[]>(`${ns()}/users`) });
}
