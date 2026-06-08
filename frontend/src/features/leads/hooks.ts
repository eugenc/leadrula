import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { get, ns, patch, post, del } from "@/lib/api";
import { chunk } from "@/lib/chunk";
import { toast } from "@/store/toastStore";
import type {
  Lead,
  LeadListResponse,
  Note,
  Pipeline,
  Stage,
  StageHistoryEntry,
  CustomField,
  DisqReason,
  UserRow,
  Me,
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
  source?: string;
  assigned?: number;
  tag?: string;
  action_on?: string;
  action_overdue?: boolean;
  view_id?: string;
  filters?: string;
  page?: number;
  limit?: number;
  sort?: string;
  sort_dir?: "asc" | "desc";
  all?: boolean;
}

function normalizeLeadsResponse(raw: LeadListResponse | Lead[] | undefined): LeadListResponse {
  if (!raw) return { items: [], total: 0, page: 1, limit: 0 };
  if (Array.isArray(raw)) return { items: raw, total: raw.length, page: 1, limit: raw.length };
  return { ...raw, items: raw.items ?? [] };
}

function leadsQueryString(filters: LeadFilters): string {
  const qs = new URLSearchParams();
  Object.entries(filters).forEach(([k, v]) => {
    if (k === "all") {
      if (v) qs.set("all", "1");
      return;
    }
    if (k === "action_overdue") {
      if (v) qs.set("action_overdue", "1");
      return;
    }
    if (k === "view_id" || k === "filters") {
      if (v) qs.set(k, String(v));
      return;
    }
    if (v !== undefined && v !== "" && v !== 0) qs.set(k, String(v));
  });
  return qs.toString();
}

export function useLeads(filters: LeadFilters = {}) {
  const q = leadsQueryString(filters);
  return useQuery({
    queryKey: ["leads", filters],
    queryFn: async () =>
      normalizeLeadsResponse(
        await get<LeadListResponse | Lead[]>(`${ns()}/leads${q ? `?${q}` : ""}`)
      ),
  });
}

export async function fetchAllLeadIds(filters: LeadFilters): Promise<number[]> {
  const q = leadsQueryString({ ...filters, all: true });
  const raw = await get<LeadListResponse | Lead[]>(`${ns()}/leads${q ? `?${q}` : ""}`);
  return normalizeLeadsResponse(raw).items.map((l) => l.id);
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
    onSuccess: (updated, { leadId }) => {
      qc.setQueriesData<LeadListResponse>({ queryKey: ["leads"] }, (old) => {
        if (!old?.items) return old;
        return {
          ...old,
          items: old.items.map((l) => (l.id === leadId ? { ...l, ...updated } : l)),
        };
      });
      qc.setQueryData(["lead", leadId], updated);
    },
  });
}

export function useSetActionAt() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: ({ leadId, action_at }: { leadId: number; action_at: string | null }) =>
      patch<{ ok: boolean }>(`${ns()}/leads/${leadId}/action`, { action_at }),
    onSuccess: (_d, v) => {
      qc.invalidateQueries({ queryKey: ["leads"] });
      qc.invalidateQueries({ queryKey: ["lead", v.leadId] });
      qc.invalidateQueries({ queryKey: ["calendar"] });
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
      if ("tags" in v.body) {
        qc.invalidateQueries({ queryKey: ["lead-tags"] });
      }
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
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ["leads"] });
    },
  });
}

/** Must match backend maxImportRows — each API call sends at most this many rows. */
export const IMPORT_BATCH_SIZE = 1000;

export interface ImportLeadsPayload {
  destination: "pipeline" | "intake";
  pipeline_id?: number;
  stage_id?: number;
  default_tags?: string[];
  mapping: { csv_column: string; target: string }[];
  rows: Record<string, string>[];
}

export interface ImportLeadsResult {
  created: number;
  skipped: number;
  errors: { row: number; message: string }[];
}

export function useImportLeads() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (body: ImportLeadsPayload) =>
      post<ImportLeadsResult>(`${ns()}/leads/import`, body),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ["leads"] });
      qc.invalidateQueries({ queryKey: ["lead-tags"] });
    },
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

export function useStageDisqReasons(stageId: number | null) {
  return useQuery({
    queryKey: ["stage-disq-reasons", stageId],
    queryFn: () => get<DisqReason[]>(`${ns()}/stages/${stageId}/disqualification-reasons`),
    enabled: !!stageId,
  });
}

export function useUsers() {
  return useQuery({ queryKey: ["users"], queryFn: () => get<UserRow[]>(`${ns()}/users`) });
}

export function useTagSuggestions() {
  return useQuery({
    queryKey: ["lead-tags"],
    queryFn: () => get<string[]>(`${ns()}/leads/tags`),
  });
}

export type BulkLeadAction = "delete" | "assign_user" | "add_follower" | "assign_buyer";

export const DELETE_BATCH_SIZE = 100;

export function useBulkLeads() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (body: {
      action: BulkLeadAction;
      ids: number[];
      user_id?: number;
      contract_id?: number;
    }) => post<{ affected: number }>(`${ns()}/leads/bulk`, body),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ["leads"] });
      qc.invalidateQueries({ queryKey: ["lead"] });
    },
  });
}

export function useDeleteLead() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (id: number) => del<{ ok: boolean }>(`${ns()}/leads/${id}`),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ["leads"] });
      qc.invalidateQueries({ queryKey: ["lead"] });
    },
  });
}

export async function deleteLeadsWithProgress(
  ids: number[],
  postBulk: (body: { action: "delete"; ids: number[] }) => Promise<{ affected: number }>
): Promise<number> {
  if (!ids.length) return 0;

  const batches = chunk(ids, DELETE_BATCH_SIZE);
  const totalLabel = ids.length.toLocaleString();
  let progressToastId = toast.progress(`Deleting 0 of ${totalLabel} leads…`);
  let affected = 0;
  let processed = 0;

  try {
    for (const batch of batches) {
      const res = await postBulk({ action: "delete", ids: batch });
      affected += res.affected;
      processed += batch.length;
      toast.update(progressToastId, `Deleting ${processed.toLocaleString()} of ${totalLabel} leads…`);
    }
    toast.dismiss(progressToastId);
    return affected;
  } catch (err) {
    toast.dismiss(progressToastId);
    throw err;
  }
}

export function useMe() {
  return useQuery({ queryKey: ["me"], queryFn: () => get<Me>("/auth/me") });
}

export function usePatchPrefs() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (patchBody: Record<string, unknown>) =>
      patch<Record<string, unknown>>("/auth/me/prefs", patchBody),
    onSuccess: () => qc.invalidateQueries({ queryKey: ["me"] }),
  });
}

export function useUpdateMyAccount() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (timezone: string) =>
      patch<{ account: Me["account"] }>("/auth/me/account", { timezone }),
    onSuccess: () => qc.invalidateQueries({ queryKey: ["me"] }),
  });
}
