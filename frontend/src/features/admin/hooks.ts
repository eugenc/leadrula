import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { get, ns, post, postForm, patch, del } from "@/lib/api";
import { useAuthStore } from "@/store/authStore";
import type {
  ApiKey,
  BuyerSummary,
  BuyerDetail,
  BuyerCollabSummary,
  CollaborationStatus,
  CalendarEvent,
  Source,
  Route,
  RouteFieldMapEntry,
  RouteFieldMapOptions,
  Contract,
  Dispute,
  FieldMapEntry,
  SourceSamplePayload,
  QueueItem,
  ReturnRule,
  RuleAction,
  RuleCondition,
  StageRule,
  Transaction,
  UserRow,
  CustomField,
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

// ── Disqualification reasons ──────────────────────────────────────
export function useCreateReason() {
  const inv = useInvalidate(["disq-reasons"]);
  return useMutation({ mutationFn: (label: string) => post(`${ns()}/disqualification-reasons`, { label }), onSuccess: inv });
}
export function useUpdateReason() {
  const inv = useInvalidate(["disq-reasons"]);
  return useMutation({
    mutationFn: ({ id, body }: { id: number; body: Record<string, unknown> }) =>
      patch(`${ns()}/disqualification-reasons/${id}`, body),
    onSuccess: inv,
  });
}
export function useDeleteReason() {
  const inv = useInvalidate(["disq-reasons"]);
  return useMutation({ mutationFn: (id: number) => del(`${ns()}/disqualification-reasons/${id}`), onSuccess: inv });
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
  return useQuery({
    queryKey: ["contracts"],
    queryFn: () => get<Contract[]>(`/publisher/contracts`),
    enabled,
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
export function useReturnRules(contractId: number | null, buyer = false) {
  const path = buyer ? `/buyer/contract/return-rules` : `/publisher/contracts/${contractId}/return-rules`;
  return useQuery({
    queryKey: ["return-rules", contractId, buyer],
    queryFn: () => get<ReturnRule[]>(path),
    enabled: buyer || !!contractId,
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
      contractId: number | null;
      buyerStageId: number;
      returnStageId: number;
    }) =>
      post(buyer ? `/buyer/contract/return-rules` : `/publisher/contracts/${contractId}/return-rules`, {
        buyer_stage_id: buyerStageId,
        return_stage_id: returnStageId,
      }),
    onSuccess: () => qc.invalidateQueries({ queryKey: ["return-rules"] }),
  });
}
export function useUpdateReturnRule(buyer = false) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: ({
      ruleId,
      buyerStageId,
      returnStageId,
    }: {
      ruleId: number;
      buyerStageId: number;
      returnStageId: number;
    }) =>
      patch(
        buyer ? `/buyer/contract/return-rules/${ruleId}` : `/publisher/return-rules/${ruleId}`,
        { buyer_stage_id: buyerStageId, return_stage_id: returnStageId }
      ),
    onSuccess: () => qc.invalidateQueries({ queryKey: ["return-rules"] }),
  });
}
export function useDeleteReturnRule(buyer = false) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (ruleId: number) =>
      del(buyer ? `/buyer/contract/return-rules/${ruleId}` : `/publisher/return-rules/${ruleId}`),
    onSuccess: () => qc.invalidateQueries({ queryKey: ["return-rules"] }),
  });
}

export function useMyContract() {
  return useQuery({ queryKey: ["my-contract"], queryFn: () => get<Contract>(`/buyer/contract`) });
}
export function useContractPublisherStages(buyer = false, sourcePipelineId?: number) {
  return useQuery({
    queryKey: ["contract-publisher-stages", buyer, sourcePipelineId],
    queryFn: () =>
      buyer
        ? get<import("@/types").Stage[]>(`/buyer/contract/publisher-stages`)
        : get<import("@/types").Stage[]>(`${ns()}/pipelines/${sourcePipelineId}/stages`),
    enabled: buyer || !!sourcePipelineId,
  });
}

// ── Sources & Routes (publisher) ────────────────────────────────────
export function useSources() {
  return useQuery({ queryKey: ["sources"], queryFn: () => get<Source[]>(`/publisher/sources`) });
}
export function useCreateSource() {
  const inv = useInvalidate(["sources"]);
  return useMutation({ mutationFn: (body: Record<string, unknown>) => post(`/publisher/sources`, body), onSuccess: inv });
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
export function useIntakeQueue(status = "pending_review") {
  return useQuery({
    queryKey: ["intake-queue", status],
    queryFn: () => get<QueueItem[]>(`/publisher/intake-queue?status=${status}`),
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

// ── Buyers (publisher oversight) ──────────────────────────────────
export function useBuyers() {
  return useQuery({ queryKey: ["buyers"], queryFn: () => get<BuyerSummary[]>(`/publisher/buyers`) });
}
export function useCreateBuyer() {
  const inv = useInvalidate(["buyers"]);
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
export function useTopup() {
  const inv = useInvalidate(["balance", "transactions"]);
  return useMutation({ mutationFn: (amount: number) => post(`/buyer/billing/balance/topup`, { amount }), onSuccess: inv });
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
