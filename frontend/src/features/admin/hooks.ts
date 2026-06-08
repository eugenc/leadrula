import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { get, ns, post, postForm, patch, del } from "@/lib/api";
import { useAuthStore } from "@/store/authStore";
import type {
  ApiKey,
  BuyerSummary,
  BuyerDetail,
  PublisherSummary,
  PublisherDetail,
  BuyerCollabSummary,
  BuyerPublisher,
  CollaborationStatus,
  Partnership,
  CollaborationAuditEntry,
  AuditLogListResponse,
  AuditLogActor,
  CalendarEvent,
  Source,
  Route,
  RouteFieldMapEntry,
  RouteFieldMapOptions,
  Contract,
  ContractCompensation,
  ContractLeadCriteria,
  Dispute,
  FieldMapEntry,
  SourceSamplePayload,
  QueueItem,
  QueueListResponse,
  ReturnRule,
  RuleAction,
  RuleCondition,
  StageRule,
  Transaction,
  UserRow,
  CustomField,
  DisqReason,
} from "@/types";

function useInvalidate(keys: string[]) {
  const qc = useQueryClient();
  return () => keys.forEach((k) => qc.invalidateQueries({ queryKey: [k] }));
}

// ── Pipelines & stages ────────────────────────────────────────────
export function useCreatePipeline() {
  const inv = useInvalidate(["pipelines"]);
  return useMutation({ mutationFn: (name: string) => post(`${ns()}/pipelines`, { name }), onSuccess: inv });
}
export function useDeletePipeline() {
  const inv = useInvalidate(["pipelines"]);
  return useMutation({ mutationFn: (id: number) => del(`${ns()}/pipelines/${id}`), onSuccess: inv });
}
export function useUpdatePipeline() {
  const inv = useInvalidate(["pipelines"]);
  return useMutation({
    mutationFn: ({ id, name }: { id: number; name: string }) =>
      patch(`${ns()}/pipelines/${id}`, { name }),
    onSuccess: inv,
  });
}
export function useCreateStage() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: ({ pipelineId, body }: { pipelineId: number; body: Record<string, unknown> }) =>
      post(`${ns()}/pipelines/${pipelineId}/stages`, body),
    onSuccess: () => qc.invalidateQueries({ queryKey: ["stages"] }),
  });
}
export function useUpdateStage() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: ({ id, body }: { id: number; body: Record<string, unknown> }) =>
      patch(`${ns()}/stages/${id}`, body),
    onSuccess: () => qc.invalidateQueries({ queryKey: ["stages"] }),
  });
}
export function useDeleteStage() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (id: number) => del(`${ns()}/stages/${id}`),
    onSuccess: () => qc.invalidateQueries({ queryKey: ["stages"] }),
  });
}

export function useStageRules(stageId: number | null) {
  return useQuery({
    queryKey: ["stage-rules", stageId],
    queryFn: () => get<StageRule[]>(`${ns()}/stages/${stageId}/rules`),
    enabled: !!stageId,
  });
}
export function useCreateStageRule() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: ({
      stageId,
      body,
    }: {
      stageId: number;
      body: { condition_logic: string; conditions: RuleCondition[]; actions: RuleAction[] };
    }) => post(`${ns()}/stages/${stageId}/rules`, body),
    onSuccess: () => qc.invalidateQueries({ queryKey: ["stage-rules"] }),
  });
}
export function useUpdateStageRule() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: ({
      id,
      body,
    }: {
      id: number;
      body: Partial<{ condition_logic: string; conditions: RuleCondition[]; actions: RuleAction[] }>;
    }) => patch(`${ns()}/stage-rules/${id}`, body),
    onSuccess: () => qc.invalidateQueries({ queryKey: ["stage-rules"] }),
  });
}
export function useDeleteStageRule() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (id: number) => del(`${ns()}/stage-rules/${id}`),
    onSuccess: () => qc.invalidateQueries({ queryKey: ["stage-rules"] }),
  });
}

// ── Custom fields ─────────────────────────────────────────────────
export function useCreateField() {
  const inv = useInvalidate(["custom-fields"]);
  return useMutation({
    mutationFn: (body: Record<string, unknown>) => post<CustomField>(`${ns()}/custom-fields`, body),
    onSuccess: inv,
  });
}
export function useUpdateField() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: ({ id, body }: { id: number; body: Record<string, unknown> }) =>
      patch(`${ns()}/custom-fields/${id}`, body),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ["custom-fields"] });
      qc.invalidateQueries({ queryKey: ["stage-rules"] });
    },
  });
}
export function useDeleteField() {
  const inv = useInvalidate(["custom-fields"]);
  return useMutation({ mutationFn: (id: number) => del(`${ns()}/custom-fields/${id}`), onSuccess: inv });
}

export const CUSTOM_FIELD_IMPORT_BATCH_SIZE = 1000;

export interface ImportCustomFieldsResult {
  created: number;
  updated: number;
  skipped: number;
  errors: { row: number; message: string }[];
}

export interface ImportCustomFieldsPayload {
  mapping: { csv_column: string; target: string }[];
  rows: Record<string, string>[];
}

export function useImportCustomFields() {
  const inv = useInvalidate(["custom-fields"]);
  return useMutation({
    mutationFn: (body: ImportCustomFieldsPayload) =>
      post<ImportCustomFieldsResult>(`${ns()}/custom-fields/import`, body),
    onSuccess: inv,
  });
}

// ── Stage disqualification reasons ────────────────────────────────
export function useStageDisqReasons(stageId: number | null) {
  return useQuery({
    queryKey: ["stage-disq-reasons", stageId],
    queryFn: () => get<DisqReason[]>(`${ns()}/stages/${stageId}/disqualification-reasons`),
    enabled: !!stageId,
  });
}

export function usePipelineDisqReasons(pipelineId: number | null) {
  return useQuery({
    queryKey: ["pipeline-disq-reasons", pipelineId],
    queryFn: () => get<DisqReason[]>(`${ns()}/pipelines/${pipelineId}/disqualification-reasons`),
    enabled: !!pipelineId,
  });
}

export function useCreateStageReason() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: ({ stageId, label }: { stageId: number; label: string }) =>
      post(`${ns()}/stages/${stageId}/disqualification-reasons`, { label }),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ["stage-disq-reasons"] });
      qc.invalidateQueries({ queryKey: ["pipeline-disq-reasons"] });
    },
  });
}

export function useUpdateStageReason() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: ({ id, body }: { id: number; body: Record<string, unknown> }) =>
      patch(`${ns()}/disqualification-reasons/${id}`, body),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ["stage-disq-reasons"] });
      qc.invalidateQueries({ queryKey: ["pipeline-disq-reasons"] });
    },
  });
}

export function useDeleteStageReason() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (id: number) => del(`${ns()}/disqualification-reasons/${id}`),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ["stage-disq-reasons"] });
      qc.invalidateQueries({ queryKey: ["pipeline-disq-reasons"] });
    },
  });
}

// ── Users ─────────────────────────────────────────────────────────
export function useInviteUser() {
  const inv = useInvalidate(["users"]);
  return useMutation({ mutationFn: (body: Record<string, unknown>) => post(`${ns()}/users/invite`, body), onSuccess: inv });
}
export function useUpdateUser() {
  const qc = useQueryClient();
  const inv = useInvalidate(["users"]);
  return useMutation({
    mutationFn: ({ id, body }: { id: number; body: Record<string, unknown> }) =>
      patch<UserRow>(`${ns()}/users/${id}`, body),
    onSuccess: (data) => {
      inv();
      qc.invalidateQueries({ queryKey: ["me"] });
      const user = useAuthStore.getState().user;
      if (user && data.public_id === user.id) {
        useAuthStore.getState().syncUserProfile({
          full_name: data.full_name,
          email: data.email,
          role: data.role,
          avatar_url: data.avatar_url,
        });
      }
    },
  });
}
export function useUpdateInvite() {
  const inv = useInvalidate(["users"]);
  return useMutation({
    mutationFn: ({ id, body }: { id: number; body: Record<string, unknown> }) =>
      patch(`${ns()}/users/invites/${id}`, body),
    onSuccess: inv,
  });
}
export function useDeleteUser() {
  const inv = useInvalidate(["users"]);
  return useMutation({ mutationFn: (id: number) => del(`${ns()}/users/${id}`), onSuccess: inv });
}
export function useDeleteInvite() {
  const inv = useInvalidate(["users"]);
  return useMutation({ mutationFn: (id: number) => del(`${ns()}/users/invites/${id}`), onSuccess: inv });
}
export function useResendInvite() {
  const inv = useInvalidate(["users"]);
  return useMutation({
    mutationFn: (id: number) => post(`${ns()}/users/invites/${id}/resend`),
    onSuccess: inv,
  });
}
export function useRequestPasswordReset() {
  return useMutation({
    mutationFn: (email: string) => post("/auth/password-reset/request", { email }),
  });
}
export function useUploadMyAvatar() {
  const qc = useQueryClient();
  const setUserAvatar = useAuthStore((s) => s.setUserAvatar);
  return useMutation({
    mutationFn: (file: File) => {
      const form = new FormData();
      form.append("avatar", file);
      return postForm<{ avatar_url: string }>("/auth/me/avatar", form);
    },
    onSuccess: (res) => {
      setUserAvatar(res.avatar_url);
      qc.invalidateQueries({ queryKey: ["me"] });
    },
  });
}
export function useUploadUserAvatar() {
  const inv = useInvalidate(["users", "leads"]);
  return useMutation({
    mutationFn: ({ id, file }: { id: number; file: File }) => {
      const form = new FormData();
      form.append("avatar", file);
      return postForm<{ avatar_url: string }>(`${ns()}/users/${id}/avatar`, form);
    },
    onSuccess: inv,
  });
}

// ── Contracts (publisher) ─────────────────────────────────────────
export function useContracts(enabled = true) {
  const accountId = useAuthStore((s) => s.user?.account_id);
  return useQuery({
    queryKey: ["contracts", accountId],
    queryFn: () => get<Contract[]>(`/publisher/contracts`),
    enabled: enabled && !!accountId,
  });
}
export function useCreateContract() {
  const inv = useInvalidate(["contracts"]);
  return useMutation({ mutationFn: (body: Record<string, unknown>) => post(`/publisher/contracts`, body), onSuccess: inv });
}
export function useUpdateContract() {
  const inv = useInvalidate(["contracts"]);
  return useMutation({
    mutationFn: ({ id, body }: { id: number; body: Record<string, unknown> }) =>
      patch(`/publisher/contracts/${id}`, body),
    onSuccess: inv,
  });
}
export function useDeleteContract() {
  const inv = useInvalidate(["contracts"]);
  return useMutation({ mutationFn: (id: number) => del(`/publisher/contracts/${id}`), onSuccess: inv });
}

export function useContractCompensations(contractId: number | null, buyer = false) {
  const path = buyer
    ? `/buyer/contracts/${contractId}/compensations`
    : `/publisher/contracts/${contractId}/compensations`;
  return useQuery({
    queryKey: ["contract-compensations", contractId, buyer],
    queryFn: () => get<ContractCompensation[]>(path),
    enabled: !!contractId,
  });
}

export function useAddContractCompensation() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: ({ contractId, body }: { contractId: number; body: Record<string, unknown> }) =>
      post(`/publisher/contracts/${contractId}/compensations`, body),
    onSuccess: (_, v) => {
      qc.invalidateQueries({ queryKey: ["contract-compensations", v.contractId] });
      qc.invalidateQueries({ queryKey: ["contracts"] });
    },
  });
}

export function useUpdateContractCompensation() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: ({
      contractId,
      compId,
      body,
    }: {
      contractId: number;
      compId: number;
      body: Record<string, unknown>;
    }) => patch(`/publisher/contracts/${contractId}/compensations/${compId}`, body),
    onSuccess: (_, v) => {
      qc.invalidateQueries({ queryKey: ["contract-compensations", v.contractId] });
      qc.invalidateQueries({ queryKey: ["contracts"] });
    },
  });
}

export function useDeleteContractCompensation() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: ({ contractId, compId }: { contractId: number; compId: number }) =>
      del(`/publisher/contracts/${contractId}/compensations/${compId}`),
    onSuccess: (_, v) => {
      qc.invalidateQueries({ queryKey: ["contract-compensations", v.contractId] });
      qc.invalidateQueries({ queryKey: ["contracts"] });
    },
  });
}

export function useContractLeadCriteria(contractId: number | null) {
  return useQuery({
    queryKey: ["contract-lead-criteria", contractId],
    queryFn: () => get<ContractLeadCriteria>(`/publisher/contracts/${contractId}/lead-criteria`),
    enabled: !!contractId,
  });
}

export function useSaveContractLeadCriteria() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: ({ contractId, body }: { contractId: number; body: ContractLeadCriteria }) =>
      patch(`/publisher/contracts/${contractId}/lead-criteria`, body),
    onSuccess: (_, v) => qc.invalidateQueries({ queryKey: ["contract-lead-criteria", v.contractId] }),
  });
}

export function useLinkPublisherPartnership() {
  return useMutation({
    mutationFn: (publisher_handler_id: string) =>
      post(`/publisher/partnerships/publishers/link`, { publisher_handler_id }),
  });
}
export function useReturnRules(contractId: number | null, buyer = false) {
  const path = buyer
    ? `/buyer/contracts/${contractId}/return-rules`
    : `/publisher/contracts/${contractId}/return-rules`;
  return useQuery({
    queryKey: ["return-rules", contractId, buyer],
    queryFn: () => get<ReturnRule[]>(path),
    enabled: !!contractId,
  });
}
export function useAddReturnRule(buyer = false) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: ({
      contractId,
      buyerStageId,
      returnStageId,
    }: {
      contractId: number;
      buyerStageId: number;
      returnStageId: number;
    }) =>
      post(
        buyer
          ? `/buyer/contracts/${contractId}/return-rules`
          : `/publisher/contracts/${contractId}/return-rules`,
        { buyer_stage_id: buyerStageId, return_stage_id: returnStageId }
      ),
    onSuccess: () => qc.invalidateQueries({ queryKey: ["return-rules"] }),
  });
}
export function useUpdateReturnRule(buyer = false) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: ({
      contractId,
      ruleId,
      buyerStageId,
      returnStageId,
    }: {
      contractId: number;
      ruleId: number;
      buyerStageId: number;
      returnStageId: number;
    }) =>
      patch(
        buyer
          ? `/buyer/contracts/${contractId}/return-rules/${ruleId}`
          : `/publisher/return-rules/${ruleId}`,
        { buyer_stage_id: buyerStageId, return_stage_id: returnStageId }
      ),
    onSuccess: () => qc.invalidateQueries({ queryKey: ["return-rules"] }),
  });
}
export function useDeleteReturnRule(buyer = false) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: ({ contractId, ruleId }: { contractId: number; ruleId: number }) =>
      del(
        buyer
          ? `/buyer/contracts/${contractId}/return-rules/${ruleId}`
          : `/publisher/return-rules/${ruleId}`
      ),
    onSuccess: () => qc.invalidateQueries({ queryKey: ["return-rules"] }),
  });
}

export function useBuyerContracts() {
  const accountId = useAuthStore((s) => s.user?.account_id);
  return useQuery({
    queryKey: ["buyer-contracts", accountId],
    queryFn: () => get<Contract[]>(`/buyer/contracts`),
    enabled: !!accountId,
  });
}

export function useMyPublisher() {
  return useQuery({
    queryKey: ["my-publisher"],
    queryFn: () => get<BuyerPublisher>("/buyer/publisher"),
  });
}

export function useBuyerRoutes() {
  return useQuery({
    queryKey: ["buyer-routes"],
    queryFn: () => get<Route[]>("/buyer/routes"),
  });
}

export interface BuyerLogsFilters {
  page?: number;
  limit?: number;
  from?: string;
  to?: string;
  actor_user_id?: number;
}

export interface PublisherLogsFilters extends BuyerLogsFilters {
  buyer_id?: number;
}

function auditLogsQueryString(filters: BuyerLogsFilters): string {
  const qs = new URLSearchParams();
  Object.entries(filters).forEach(([k, v]) => {
    if (v !== undefined && v !== "" && v !== 0) qs.set(k, String(v));
  });
  return qs.toString();
}

export function useBuyerLogs(filters: BuyerLogsFilters = { page: 1, limit: 25 }) {
  const q = auditLogsQueryString(filters);
  return useQuery({
    queryKey: ["buyer-logs", filters],
    queryFn: () => get<AuditLogListResponse>(`/buyer/logs?${q}`),
  });
}

export function useBuyerLogActors() {
  return useQuery({
    queryKey: ["buyer-logs", "actors"],
    queryFn: () => get<AuditLogActor[]>("/buyer/logs/actors"),
  });
}

export function usePublisherLogs(filters: PublisherLogsFilters = { page: 1, limit: 25 }) {
  const q = auditLogsQueryString(filters);
  return useQuery({
    queryKey: ["publisher-logs", filters],
    queryFn: () => get<AuditLogListResponse>(`/publisher/collaboration/logs?${q}`),
  });
}

export function usePublisherLogActors() {
  return useQuery({
    queryKey: ["publisher-logs", "actors"],
    queryFn: () => get<AuditLogActor[]>("/publisher/collaboration/logs/actors"),
  });
}
export function useContractPublisherStages(contractId?: number, buyer = false) {
  return useQuery({
    queryKey: ["contract-publisher-stages", contractId, buyer],
    queryFn: () =>
      buyer
        ? get<import("@/types").Stage[]>(`/buyer/contracts/${contractId}/publisher-stages`)
        : get<import("@/types").Stage[]>(`${ns()}/pipelines/${contractId}/stages`),
    enabled: buyer ? !!contractId : !!contractId,
  });
}

// ── Sources & Routes (publisher) ────────────────────────────────────
export function useSources() {
  return useQuery({ queryKey: ["sources"], queryFn: () => get<Source[]>(`/publisher/sources`) });
}
export function useCreateSource() {
  const inv = useInvalidate(["sources"]);
  return useMutation({
    mutationFn: (body: Record<string, unknown>) => post<Source>(`/publisher/sources`, body),
    onSuccess: inv,
  });
}
export function useUpdateSource() {
  const inv = useInvalidate(["sources"]);
  return useMutation({
    mutationFn: ({ id, body }: { id: number; body: Record<string, unknown> }) => patch(`/publisher/sources/${id}`, body),
    onSuccess: inv,
  });
}
export function useDeleteSource() {
  const inv = useInvalidate(["sources"]);
  return useMutation({ mutationFn: (id: number) => del(`/publisher/sources/${id}`), onSuccess: inv });
}
export function useSourceFieldMap(sourceId: number | null) {
  return useQuery({
    queryKey: ["source-field-map", sourceId],
    queryFn: () => get<FieldMapEntry[]>(`/publisher/sources/${sourceId}/field-map`),
    enabled: !!sourceId,
  });
}
export function useSourceSamplePayload(sourceId: number | null, poll = false) {
  return useQuery({
    queryKey: ["source-sample-payload", sourceId],
    queryFn: () => get<SourceSamplePayload>(`/publisher/sources/${sourceId}/sample-payload`),
    enabled: !!sourceId,
    refetchInterval: poll ? 5000 : false,
  });
}
export function useAddSourceFieldMap() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: ({ sourceId, body }: { sourceId: number; body: Record<string, unknown> }) =>
      post(`/publisher/sources/${sourceId}/field-map`, body),
    onSuccess: () => qc.invalidateQueries({ queryKey: ["source-field-map"] }),
  });
}
export function useDeleteSourceFieldMap() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (id: number) => del(`/publisher/source-field-map/${id}`),
    onSuccess: () => qc.invalidateQueries({ queryKey: ["source-field-map"] }),
  });
}

export function useRoutes() {
  return useQuery({ queryKey: ["routes"], queryFn: () => get<Route[]>(`/publisher/routes`) });
}
export function useCreateRoute() {
  const inv = useInvalidate(["routes"]);
  return useMutation({ mutationFn: (body: Record<string, unknown>) => post(`/publisher/routes`, body), onSuccess: inv });
}
export function useUpdateRoute() {
  const inv = useInvalidate(["routes"]);
  return useMutation({
    mutationFn: ({ id, body }: { id: number; body: Record<string, unknown> }) => patch(`/publisher/routes/${id}`, body),
    onSuccess: inv,
  });
}
export function useDeleteRoute() {
  const inv = useInvalidate(["routes"]);
  return useMutation({ mutationFn: (id: number) => del(`/publisher/routes/${id}`), onSuccess: inv });
}
export function useRouteFieldMap(routeId: number | null) {
  return useQuery({
    queryKey: ["route-field-map", routeId],
    queryFn: () => get<RouteFieldMapEntry[]>(`/publisher/routes/${routeId}/field-map`),
    enabled: !!routeId,
  });
}
export function useRouteFieldMapOptions(routeId: number | null) {
  return useQuery({
    queryKey: ["route-field-map-options", routeId],
    queryFn: () => get<RouteFieldMapOptions>(`/publisher/routes/${routeId}/field-map/options`),
    enabled: routeId != null,
  });
}
export function useAddRouteFieldMap() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: ({ routeId, body }: { routeId: number; body: Record<string, unknown> }) =>
      post(`/publisher/routes/${routeId}/field-map`, body),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ["route-field-map"] });
      qc.invalidateQueries({ queryKey: ["route-field-map-options"] });
    },
  });
}
export function useDeleteRouteFieldMap() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (id: number) => del(`/publisher/route-field-map/${id}`),
    onSuccess: () => qc.invalidateQueries({ queryKey: ["route-field-map"] }),
  });
}
export function useCreateBuyerRouteField(routeId: number) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (body: Record<string, unknown>) =>
      post<CustomField>(`/publisher/routes/${routeId}/buyer-custom-fields`, body),
    onSuccess: () => qc.invalidateQueries({ queryKey: ["route-field-map-options", routeId] }),
  });
}

// ── Intake queue (publisher) ──────────────────────────────────────
export interface IntakeLogFilters {
  status?: string;
  page?: number;
  limit?: number;
  q?: string;
  source?: string;
}

function intakeQueueQueryString(filters: IntakeLogFilters): string {
  const qs = new URLSearchParams();
  Object.entries(filters).forEach(([k, v]) => {
    if (v !== undefined && v !== "" && v !== 0) qs.set(k, String(v));
  });
  return qs.toString();
}

function normalizeQueueResponse(raw: QueueListResponse | QueueItem[] | undefined): QueueListResponse {
  if (!raw) return { items: [], total: 0, page: 1, limit: 0 };
  if (Array.isArray(raw)) return { items: raw, total: raw.length, page: 1, limit: raw.length };
  return { ...raw, items: raw.items ?? [] };
}

export function useIntakeQueue(status = "pending_review") {
  return useQuery({
    queryKey: ["intake-queue", status],
    queryFn: async () =>
      normalizeQueueResponse(
        await get<QueueListResponse | QueueItem[]>(`/publisher/intake-queue?status=${status}`)
      ),
  });
}
export function useIntakeLog(filters: IntakeLogFilters = { status: "all", page: 1, limit: 25 }) {
  const q = intakeQueueQueryString(filters);
  return useQuery({
    queryKey: ["intake-queue", "log", filters],
    queryFn: async () =>
      normalizeQueueResponse(await get<QueueListResponse>(`/publisher/intake-queue?${q}`)),
  });
}
export function useBuyerRoutingLog(filters: IntakeLogFilters = { status: "all", page: 1, limit: 25 }) {
  const q = intakeQueueQueryString(filters);
  return useQuery({
    queryKey: ["buyer-routing-log", filters],
    queryFn: async () =>
      normalizeQueueResponse(await get<QueueListResponse>(`/buyer/routing-log?${q}`)),
  });
}
export function useRoutingLog(source: "publisher" | "buyer", filters: IntakeLogFilters) {
  const q = intakeQueueQueryString(filters);
  return useQuery({
    queryKey: [source === "buyer" ? "buyer-routing-log" : "intake-queue", "log", filters],
    queryFn: async () => {
      const path =
        source === "buyer" ? `/buyer/routing-log?${q}` : `/publisher/intake-queue?${q}`;
      return normalizeQueueResponse(await get<QueueListResponse>(path));
    },
  });
}
export function useMapQueueField() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: ({ id, body }: { id: number; body: Record<string, unknown> }) =>
      post<QueueItem>(`/publisher/intake-queue/${id}/map-field`, body),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ["intake-queue"] });
      qc.invalidateQueries({ queryKey: ["source-field-map"] });
      qc.invalidateQueries({ queryKey: ["leads"] });
    },
  });
}
export function useRouteQueue() {
  const inv = useInvalidate(["intake-queue", "leads"]);
  return useMutation({
    mutationFn: ({ id, body }: { id: number; body: Record<string, unknown> }) =>
      post(`/publisher/intake-queue/${id}/route`, body),
    onSuccess: inv,
  });
}
export function useRejectQueue() {
  const inv = useInvalidate(["intake-queue"]);
  return useMutation({ mutationFn: (id: number) => post(`/publisher/intake-queue/${id}/reject`), onSuccess: inv });
}
export function useRerunIntakeQueue() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (id: number) => post<QueueItem>(`/publisher/intake-queue/${id}/rerun`, {}),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ["intake-queue"] });
      qc.invalidateQueries({ queryKey: ["leads"] });
    },
  });
}

// ── Buyers (publisher oversight) ──────────────────────────────────
export function useBuyers() {
  return useQuery({ queryKey: ["buyers"], queryFn: () => get<BuyerSummary[]>(`/publisher/buyers`) });
}
export function useCreateBuyer() {
  const inv = useInvalidate(["buyers", "partnerships"]);
  return useMutation({
    mutationFn: (body: {
      name: string;
      admin_first_name: string;
      admin_last_name: string;
      admin_email: string;
      website?: string;
      starting_balance?: number;
      timezone?: string;
      collaborate_enabled?: boolean;
    }) => post<BuyerSummary>("/publisher/buyers", body),
    onSuccess: inv,
  });
}
export function useBuyer(buyerId: number | null) {
  return useQuery({
    queryKey: ["buyer", buyerId],
    queryFn: () => get<BuyerDetail>(`/publisher/buyers/${buyerId}`),
    enabled: !!buyerId,
  });
}
export function useUpdateBuyer() {
  const inv = useInvalidate(["buyers", "buyer"]);
  return useMutation({
    mutationFn: ({ id, body }: { id: number; body: Record<string, string> }) =>
      patch<BuyerDetail>(`/publisher/buyers/${id}`, body),
    onSuccess: inv,
  });
}
export function useResendBuyerAdminInvite() {
  const inv = useInvalidate(["buyer"]);
  return useMutation({
    mutationFn: (buyerId: number) => post(`/publisher/buyers/${buyerId}/resend-admin-invite`),
    onSuccess: inv,
  });
}
export function useBuyerPipelines(buyerId: number | null) {
  return useQuery({
    queryKey: ["buyer-pipelines", buyerId],
    queryFn: () => get<import("@/types").Pipeline[]>(`/publisher/buyers/${buyerId}/pipelines`),
    enabled: !!buyerId,
  });
}
export function useBuyerStages(buyerId: number | null, pipelineId: number | null) {
  return useQuery({
    queryKey: ["buyer-stages", buyerId, pipelineId],
    queryFn: () => get<import("@/types").Stage[]>(`/publisher/buyers/${buyerId}/pipelines/${pipelineId}/stages`),
    enabled: !!buyerId && !!pipelineId,
  });
}
export function useBuyerLeads(buyerId: number | null) {
  return useQuery({
    queryKey: ["buyer-leads", buyerId],
    queryFn: () => get<import("@/types").Lead[]>(`/publisher/buyers/${buyerId}/leads`),
    enabled: !!buyerId,
  });
}
export function useBuyerBilling(buyerId: number | null) {
  return useQuery({
    queryKey: ["buyer-billing", buyerId],
    queryFn: () =>
      get<{ balance: number; transactions: Transaction[] }>(`/publisher/buyers/${buyerId}/billing`),
    enabled: !!buyerId,
  });
}

// ── Publishers (buyer oversight) ──────────────────────────────────
export function usePublishers() {
  return useQuery({ queryKey: ["publishers"], queryFn: () => get<PublisherSummary[]>("/buyer/publishers") });
}

export function usePartnerPublishers() {
  return useQuery({
    queryKey: ["partner-publishers"],
    queryFn: () => get<PublisherSummary[]>(`/publisher/partnerships/publishers`),
  });
}
export function usePublisher(publisherId: number | null) {
  return useQuery({
    queryKey: ["publisher", publisherId],
    queryFn: () => get<PublisherDetail>(`/buyer/publishers/${publisherId}`),
    enabled: !!publisherId,
  });
}

// ── Partnerships ────────────────────────────────────────────────────
export function usePartnerships() {
  return useQuery({
    queryKey: ["partnerships"],
    queryFn: () => get<Partnership[]>(`${ns()}/partnerships`),
  });
}
export function useRequestPartnership() {
  const inv = useInvalidate(["partnerships", "buyers", "publishers"]);
  return useMutation({
    mutationFn: (body: { buyer_handler_id?: string; publisher_handler_id?: string }) =>
      post<Partnership>(`${ns()}/partnerships/request`, body),
    onSuccess: inv,
  });
}
export function useAcceptPartnership() {
  const inv = useInvalidate(["partnerships", "buyers", "publishers"]);
  return useMutation({
    mutationFn: (id: number) => post<Partnership>(`${ns()}/partnerships/${id}/accept`),
    onSuccess: inv,
  });
}
export function useRejectPartnership() {
  const inv = useInvalidate(["partnerships", "publishers"]);
  return useMutation({
    mutationFn: (id: number) => post(`${ns()}/partnerships/${id}/reject`),
    onSuccess: inv,
  });
}

// ── Billing ───────────────────────────────────────────────────────
export function useTransactions(scope: "publisher" | "buyer", buyerId?: number, type?: string) {
  const qs = new URLSearchParams();
  if (buyerId) qs.set("buyer_id", String(buyerId));
  if (type) qs.set("type", type);
  const q = qs.toString();
  return useQuery({
    queryKey: ["transactions", scope, buyerId, type],
    queryFn: () => get<Transaction[]>(`/${scope}/billing/transactions${q ? `?${q}` : ""}`),
  });
}
export function useBalance() {
  const isBuyer = useAuthStore.getState().user?.account_type === "buyer";
  return useQuery({
    queryKey: ["balance"],
    queryFn: () => get<{ balance: number }>(`/buyer/billing/balance`),
    enabled: isBuyer,
  });
}
export type PaymentMethod = {
  id: string;
  brand: string;
  last4: string;
  exp_month: number;
  exp_year: number;
  is_default: boolean;
};

export function usePaymentMethods() {
  return useQuery({
    queryKey: ["payment-methods"],
    queryFn: () => get<PaymentMethod[]>("/buyer/billing/stripe/payment-methods"),
  });
}

export function useCreateSetupIntent() {
  return useMutation({
    mutationFn: () => post<{ client_secret: string }>("/buyer/billing/stripe/setup-intent", {}),
  });
}

export function useDetachPaymentMethod() {
  const inv = useInvalidate(["payment-methods"]);
  return useMutation({
    mutationFn: (id: string) => del<{ ok: boolean }>(`/buyer/billing/stripe/payment-methods/${id}`),
    onSuccess: inv,
  });
}

export function useCreateTopupIntent() {
  return useMutation({
    mutationFn: (amountCents: number) =>
      post<{ client_secret: string }>("/buyer/billing/balance/topup-intent", { amount_cents: amountCents }),
  });
}

export function useStripeConnect() {
  return useMutation({
    mutationFn: () =>
      post<{ onboarding_url: string }>("/publisher/billing/stripe/connect", {
        return_base_url: window.location.origin,
      }),
  });
}

export function useStripeConnectStatus() {
  return useQuery({
    queryKey: ["stripe-connect-status"],
    queryFn: () => get<{ status: string }>("/publisher/billing/stripe/status"),
  });
}
export function useDisputes(scope: "publisher" | "buyer", status?: string) {
  const q = status ? `?status=${status}` : "";
  return useQuery({
    queryKey: ["disputes", scope, status],
    queryFn: () => get<Dispute[]>(`/${scope}/billing/disputes${q}`),
  });
}
export function useOpenDispute() {
  const inv = useInvalidate(["disputes"]);
  return useMutation({
    mutationFn: (body: Record<string, unknown>) => post(`/buyer/billing/disputes`, body),
    onSuccess: inv,
  });
}
export function useResolveDispute() {
  const inv = useInvalidate(["disputes", "transactions"]);
  return useMutation({
    mutationFn: ({ id, accept }: { id: number; accept: boolean }) =>
      post(`/publisher/billing/disputes/${id}/${accept ? "accept" : "reject"}`),
    onSuccess: inv,
  });
}
export function useManualInvoice() {
  const inv = useInvalidate(["transactions"]);
  return useMutation({ mutationFn: (body: Record<string, unknown>) => post(`/publisher/billing/manual-invoice`, body), onSuccess: inv });
}

// ── API keys ──────────────────────────────────────────────────────
export function useApiKeys() {
  return useQuery({ queryKey: ["api-keys"], queryFn: () => get<ApiKey[]>(`${ns()}/api-keys`) });
}
export function useCreateApiKey() {
  const inv = useInvalidate(["api-keys"]);
  return useMutation({
    mutationFn: (name: string) => post<{ key: ApiKey; secret: string }>(`${ns()}/api-keys`, { name }),
    onSuccess: inv,
  });
}
export function useRevokeApiKey() {
  const inv = useInvalidate(["api-keys"]);
  return useMutation({ mutationFn: (id: number) => del(`${ns()}/api-keys/${id}`), onSuccess: inv });
}

// ── Calendar ──────────────────────────────────────────────────────
export function useCalendar(scope: "global" | "me") {
  return useQuery({
    queryKey: ["calendar", scope],
    queryFn: () => get<CalendarEvent[]>(`/buyer/calendar/${scope}`),
  });
}

// ── Users list (re-export convenience) ────────────────────────────
export function useUsersList() {
  return useQuery({ queryKey: ["users"], queryFn: () => get<UserRow[]>(`${ns()}/users`) });
}

// ── Collaboration ─────────────────────────────────────────────────
export function useCollabSummaries() {
  return useQuery({
    queryKey: ["collab-summaries"],
    queryFn: () => get<BuyerCollabSummary[]>("/publisher/collaboration/summaries"),
  });
}

export function useBuyerCollaboration(buyerId: number | null) {
  return useQuery({
    queryKey: ["collaboration", "buyer", buyerId],
    queryFn: () => get<CollaborationStatus>(`/publisher/collaboration/buyers/${buyerId}`),
    enabled: !!buyerId,
  });
}

export function useBuyerCollabStatus() {
  return useQuery({
    queryKey: ["collaboration", "self"],
    queryFn: () => get<CollaborationStatus>("/buyer/collaboration"),
  });
}

export function usePublisherCollaboration(publisherId: number | null) {
  return useQuery({
    queryKey: ["collaboration", "publisher", publisherId],
    queryFn: () => get<CollaborationStatus>(`/buyer/collaboration/publishers/${publisherId}`),
    enabled: !!publisherId,
  });
}

export function useInvitePublisherCollaborationForPublisher() {
  const inv = useInvalidate(["collaboration", "publishers"]);
  return useMutation({
    mutationFn: ({ publisherId, email }: { publisherId: number; email: string }) =>
      post<CollaborationStatus>(`/buyer/collaboration/publishers/${publisherId}/invite`, { email }),
    onSuccess: inv,
  });
}

export function useAcceptCollaborationForPublisher() {
  const inv = useInvalidate(["collaboration", "publishers"]);
  return useMutation({
    mutationFn: (publisherId: number) =>
      post<CollaborationStatus>(`/buyer/collaboration/publishers/${publisherId}/accept`),
    onSuccess: inv,
  });
}

export function useRejectCollaborationForPublisher() {
  const inv = useInvalidate(["collaboration", "publishers"]);
  return useMutation({
    mutationFn: (publisherId: number) =>
      post<CollaborationStatus>(`/buyer/collaboration/publishers/${publisherId}/reject`),
    onSuccess: inv,
  });
}

export function useRevokeCollaborationForPublisher() {
  const inv = useInvalidate(["collaboration", "publishers"]);
  return useMutation({
    mutationFn: (publisherId: number) =>
      del<CollaborationStatus>(`/buyer/collaboration/publishers/${publisherId}`),
    onSuccess: inv,
  });
}

export function useRequestCollaboration() {
  const inv = useInvalidate(["collaboration", "collab-summaries"]);
  return useMutation({
    mutationFn: (buyerId: number) => post<CollaborationStatus>(`/publisher/collaboration/buyers/${buyerId}/request`),
    onSuccess: inv,
  });
}

export function useAcceptCollaborationPublisher() {
  const inv = useInvalidate(["collaboration", "collab-summaries"]);
  return useMutation({
    mutationFn: (buyerId: number) => post<CollaborationStatus>(`/publisher/collaboration/buyers/${buyerId}/accept`),
    onSuccess: inv,
  });
}

export function useAcceptCollaborationByPublicId() {
  const inv = useInvalidate(["collaboration", "collab-summaries"]);
  return useMutation({
    mutationFn: (buyerPublicId: string) =>
      post<CollaborationStatus>("/publisher/collaboration/accept", { buyer_id: buyerPublicId }),
    onSuccess: inv,
  });
}

export function useRejectCollaborationPublisher() {
  const inv = useInvalidate(["collaboration", "collab-summaries"]);
  return useMutation({
    mutationFn: (buyerId: number) => post<CollaborationStatus>(`/publisher/collaboration/buyers/${buyerId}/reject`),
    onSuccess: inv,
  });
}

export function useInvitePublisherCollaboration() {
  const inv = useInvalidate(["collaboration"]);
  return useMutation({
    mutationFn: (email: string) => post<CollaborationStatus>("/buyer/collaboration/invite", { email }),
    onSuccess: inv,
  });
}

export function useAcceptCollaborationBuyer() {
  const inv = useInvalidate(["collaboration"]);
  return useMutation({
    mutationFn: () => post<CollaborationStatus>("/buyer/collaboration/accept"),
    onSuccess: inv,
  });
}

export function useRejectCollaborationBuyer() {
  const inv = useInvalidate(["collaboration"]);
  return useMutation({
    mutationFn: () => post<CollaborationStatus>("/buyer/collaboration/reject"),
    onSuccess: inv,
  });
}

export function useRevokeCollaboration() {
  const inv = useInvalidate(["collaboration"]);
  return useMutation({
    mutationFn: () => del<CollaborationStatus>("/buyer/collaboration"),
    onSuccess: inv,
  });
}

export function useImpersonateBuyer() {
  return useMutation({
    mutationFn: (buyerPublicId: string) =>
      post<{ access: string; user: Record<string, unknown> }>("/auth/impersonate", { buyer_id: buyerPublicId }),
  });
}

export function useEndImpersonation() {
  return useMutation({
    mutationFn: () => post<{ ok: boolean }>("/auth/impersonate/end"),
  });
}
