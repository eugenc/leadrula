import { useQuery, useMutation, useQueryClient, keepPreviousData } from "@tanstack/react-query";
import { get, ns, patch, post, del } from "@/lib/api";
import { chunk } from "@/lib/chunk";
import { toast } from "@/store/toastStore";
import { computeBoardStageId } from "./boardStage";
import { useAuthStore } from "@/store/authStore";
import type {
  Lead,
  LeadListResponse,
  Note,
  Pipeline,
  Stage,
  LeadHistoryEntry,
  CustomField,
  CustomFieldFolder,
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
  q?: string;
  page?: number;
  limit?: number;
  sort?: string;
  sort_dir?: "asc" | "desc";
  all?: boolean;
  include_economics?: boolean;
  include_stage_history?: boolean;
  namespace?: "publisher" | "buyer";
}

function normalizeLeadsResponse(raw: LeadListResponse | Lead[] | undefined): LeadListResponse {
  if (!raw) return { items: [], total: 0, page: 1, limit: 0 };
  if (Array.isArray(raw)) return { items: raw, total: raw.length, page: 1, limit: raw.length };
  return { ...raw, items: raw.items ?? [] };
}

function leadsQueryString(filters: LeadFilters): string {
  const qs = new URLSearchParams();
  Object.entries(filters).forEach(([k, v]) => {
    if (k === "namespace") return;
    if (k === "all") {
      if (v) qs.set("all", "1");
      return;
    }
    if (k === "action_overdue") {
      if (v) qs.set("action_overdue", "1");
      return;
    }
    if (k === "include_economics" || k === "include_stage_history") {
      if (v === false) qs.set(k, "0");
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

function leadsBasePath(namespace?: "publisher" | "buyer"): string {
  if (namespace === "publisher") return "/publisher";
  if (namespace === "buyer") return "/buyer";
  return ns();
}

export function useLeads(
  filters: LeadFilters = {},
  options?: { enabled?: boolean; keepPreviousData?: boolean }
) {
  const q = leadsQueryString(filters);
  const base = leadsBasePath(filters.namespace);
  return useQuery({
    queryKey: ["leads", filters],
    queryFn: async () =>
      normalizeLeadsResponse(
        await get<LeadListResponse | Lead[]>(`${base}/leads${q ? `?${q}` : ""}`)
      ),
    enabled: options?.enabled ?? true,
    placeholderData: options?.keepPreviousData ? keepPreviousData : undefined,
  });
}

export async function fetchLeads(filters: LeadFilters): Promise<LeadListResponse> {
  const q = leadsQueryString(filters);
  const base = leadsBasePath(filters.namespace);
  return normalizeLeadsResponse(
    await get<LeadListResponse | Lead[]>(`${base}/leads${q ? `?${q}` : ""}`)
  );
}

export const BOARD_STAGE_LIMIT = 50;

export async function fetchAllLeadIds(filters: LeadFilters): Promise<number[]> {
  const q = leadsQueryString({ ...filters, all: true });
  const raw = await get<LeadListResponse | Lead[]>(`${ns()}/leads${q ? `?${q}` : ""}`);
  return normalizeLeadsResponse(raw).items.map((l) => l.id);
}

export function useLead(id: number | null) {
  const accountType = useAuthStore((s) => s.user?.account_type);
  return useQuery({
    queryKey: ["lead", accountType, id],
    queryFn: () => get<Lead>(`${ns()}/leads/${id}`),
    enabled: !!id,
  });
}

export type StageChangePayload =
  | { clear: true }
  | {
      stage_id: number;
      action_at?: string | null;
      disqualification_reason_id?: number | null;
    };

export function useChangeStage() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: ({ leadId, payload }: { leadId: number; payload: StageChangePayload }) =>
      patch<Lead>(`${ns()}/leads/${leadId}/stage`, payload),
    onSuccess: (updated, { leadId }) => {
      qc.setQueryData(["lead", leadId], updated);
      qc.invalidateQueries({ queryKey: ["leads"] });
      qc.invalidateQueries({ queryKey: ["lead-history", leadId] });
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
      qc.invalidateQueries({ queryKey: ["lead-history", v.leadId] });
      qc.invalidateQueries({ queryKey: ["calendar"] });
    },
  });
}

export function useUpdateLead() {
  const qc = useQueryClient();
  const accountType = useAuthStore((s) => s.user?.account_type);
  return useMutation({
    mutationFn: ({ leadId, body }: { leadId: number; body: Record<string, unknown> }) =>
      patch<Lead>(`${ns()}/leads/${leadId}`, body),
    onSuccess: (updated, v) => {
      qc.setQueryData(["lead", v.leadId], updated);
      qc.setQueriesData<LeadListResponse>({ queryKey: ["leads"] }, (old) => {
        if (!old?.items) return old;
        return {
          ...old,
          items: old.items.map((l) => {
            if (l.id !== v.leadId) return l;
            const merged = { ...l, ...updated };
            return { ...merged, board_stage_id: computeBoardStageId(merged, accountType) };
          }),
        };
      });
      qc.invalidateQueries({ queryKey: ["leads"] });
      qc.invalidateQueries({ queryKey: ["lead-history", v.leadId] });
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
    onSuccess: (_d, v) => {
      qc.invalidateQueries({ queryKey: ["notes", v.leadId] });
      qc.invalidateQueries({ queryKey: ["lead-history", v.leadId] });
    },
  });
}

export function useLeadHistory(leadId: number | null) {
  return useQuery({
    queryKey: ["lead-history", leadId],
    queryFn: () => get<LeadHistoryEntry[]>(`${ns()}/leads/${leadId}/stage-history`),
    enabled: !!leadId,
  });
}

/** @deprecated use useLeadHistory */
export const useStageHistory = useLeadHistory;

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
  import_filename?: string;
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
      qc.invalidateQueries({ queryKey: ["lead-history"] });
    },
  });
}

export function useRedistribute() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: ({ leadId, contractId }: { leadId: number; contractId: number }) =>
      post<Lead>(`${ns()}/leads/${leadId}/redistribute`, { contract_id: contractId }),
    onSuccess: (_d, v) => {
      qc.invalidateQueries({ queryKey: ["leads"] });
      qc.invalidateQueries({ queryKey: ["lead"] });
      qc.invalidateQueries({ queryKey: ["lead-history", v.leadId] });
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
    onSuccess: (_d, v) => {
      qc.invalidateQueries({ queryKey: ["followers", v.leadId] });
      qc.invalidateQueries({ queryKey: ["lead-history", v.leadId] });
    },
  });
}

// shared reference data
export function useCustomFields() {
  return useQuery({ queryKey: ["custom-fields"], queryFn: () => get<CustomField[]>(`${ns()}/custom-fields`) });
}

export function useCustomFieldFolders() {
  return useQuery({
    queryKey: ["custom-field-folders"],
    queryFn: () => get<CustomFieldFolder[]>(`${ns()}/custom-field-folders`),
  });
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
    onSuccess: (_d, v) => {
      qc.invalidateQueries({ queryKey: ["leads"] });
      qc.invalidateQueries({ queryKey: ["lead"] });
      qc.invalidateQueries({ queryKey: ["lead-history"] });
    },
  });
}

export function useDeleteLead() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (id: number) => del<{ ok: boolean }>(`${ns()}/leads/${id}`),
    onSuccess: (_d, leadId) => {
      qc.invalidateQueries({ queryKey: ["leads"] });
      qc.invalidateQueries({ queryKey: ["lead"] });
      qc.invalidateQueries({ queryKey: ["lead-history", leadId] });
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
    mutationFn: (body: Partial<Me["account"]>) =>
      patch<{ account: Me["account"] }>("/auth/me/account", body),
    onSuccess: () => qc.invalidateQueries({ queryKey: ["me"] }),
  });
}
