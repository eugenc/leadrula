import { useEffect, useRef, useState, useCallback, type Dispatch, type SetStateAction } from "react";
import { useNavigate } from "react-router-dom";
import { useQueryClient } from "@tanstack/react-query";
import {
  Sheet,
  DrawerBody,
  FormDrawer,
  drawerTitleClass,
  drawerSubtitleClass,
  formFieldClass,
} from "@/components/ui/dialog";
import { IconButton } from "@/components/layout/IconButton";
import { Button } from "@/components/ui/button";
import { Input, InputWithOverflowTooltip, Label, Textarea, Select } from "@/components/ui/input";
import { Badge, Spinner } from "@/components/ui/misc";
import { LinkifiedText } from "@/components/ui/linkified-text";
import { SectionLabel } from "@/components/layout/SectionLabel";
import { LeadMessageButton } from "@/features/messaging/MessageButton";
import { ActionDot } from "./ActionDot";
import { ReturnIndicator } from "./ReturnIndicator";
import { format, isPast } from "date-fns";
import { CircleHelp, Copy, MapPin, ChevronDown, ChevronRight, X, Zap } from "lucide-react";
import { stageColorBorder, stageColorFill } from "@/features/pipelines/stageColors";
import { cn, formatMoney } from "@/lib/utils";
import { useUIStore } from "@/store/uiStore";
import { useAuthStore } from "@/store/authStore";
import {
  ActionBilling,
  ActionContractsPartners,
  canAction,
  canEditLead,
  canSeeAllLeads,
} from "@/lib/permissions";
import { toast } from "@/store/toastStore";
import { errorMessage, apiError } from "@/lib/api";
import {
  useLead,
  useAddNote,
  useLeadHistory,
  useUpdateLead,
  useSetActionAt,
  useUsers,
  useCustomFields,
  useCustomFieldFolders,
  useDeleteLead,
  useChangeStage,
  usePipelines,
  useStages,
} from "./hooks";
import { DeleteLeadConfirmDialog } from "./DeleteLeadConfirmDialog";
import { LeadCallTab } from "@/features/calls/LeadCallTab";
import { useLeadCall } from "@/features/calls/hooks";
import { useOpenReturnDispute } from "@/features/admin/hooks";
import { DEADLINE_DAY_OPTIONS } from "@/features/billing/disputeOptions";
import { StagePromptModal, type PromptResult } from "./StagePromptModal";
import {
  initialActionAtForStageMove,
  showActionAtForStage,
  stageNeedsPrompt,
  stagePromptMissingError,
} from "@/features/pipelines/stageTypes";
import type { CustomField, CustomFieldFolder, Lead, LeadHistoryEntry, Stage } from "@/types";
import { formatStatus, leadSourceLabel, formatBuyerStatus } from "./leadsListColumns";
import { LeadTagsEditor } from "./LeadTagsEditor";
import { buildWebhookActivityLogUrl } from "@/features/intake/logShared";
import {
  activityFilterGroup,
  activityGroupLabel,
  activityKindLabel,
  presentActivityGroups,
  useActivityGroupFilters,
} from "./activityFilterStorage";
import { useQuery } from "@tanstack/react-query";
import { get } from "@/lib/api";
import type { BuyerSummary } from "@/types";
import { effectiveFieldFormat } from "@/features/admin/customFieldConstants";
import { groupCustomFieldsByFolder } from "@/features/admin/customFieldLayout";
import { isContactFolder, resolveContactBuiltinOrder, type ContactFieldKey } from "./contactSection";
import { useFolderCollapse } from "./customFieldFolderCollapse";
import { useLeadAssignmentCollapse } from "./leadAssignmentCollapse";
import { DatetimeFieldInput } from "./DatetimeFieldInput";
import {
  AddressAutocomplete,
  ValidatedAddressLink,
  formatLeadAddress,
} from "./AddressAutocomplete";
import { AddressMapDialog } from "./AddressMapDialog";
import { useGoogleMapsStatus } from "@/features/integrations/hooks";
import {
  fromNativeDatetimeLocal,
  inputModeForFormat,
  isoToDatetimeLocal,
  normalizeCustomDateValue,
  toNativeDateValue,
  toNativeDatetimeLocalValue,
} from "./customFieldDate";
import { contactFieldsFromLead, dirtyContactPatch } from "./leadContactFields";

function copyText(text: string, label: string) {
  navigator.clipboard.writeText(text).then(
    () => toast.success(label),
    () => toast.error("Could not copy to clipboard")
  );
}

const DRAWER_TABS = [
  { id: "details", label: "Details" },
  { id: "activity", label: "Activity" },
  { id: "profit", label: "Profit" },
] as const;

type DrawerTab = (typeof DRAWER_TABS)[number]["id"] | "call";

function moneyOrDash(v: number | null | undefined): string {
  return v != null ? formatMoney(v) : "—";
}

function LabelWithHint({ label, hint }: { label: string; hint: string }) {
  return (
    <Label className="flex items-center gap-1.5">
      {label}
      <span className="group/hint relative inline-flex shrink-0">
        <button
          type="button"
          className="inline-flex text-gray-400 hover:text-gray-600"
          aria-label={hint}
        >
          <CircleHelp className="h-3.5 w-3.5" />
        </button>
        <span
          role="tooltip"
          className="pointer-events-none absolute left-0 top-full z-10 mt-1.5 w-56 rounded-md bg-[#101828] px-2 py-1.5 text-xs font-normal leading-snug text-[#F9FAFB] opacity-0 shadow-sm transition-opacity duration-150 group-hover/hint:opacity-100"
        >
          {hint}
        </span>
      </span>
    </Label>
  );
}

function LeadEconomics({ lead, accountType }: { lead: Lead; accountType?: string }) {
  const isBuyer = accountType === "buyer";
  return (
    <div className="flex flex-col gap-2.5">
      {isBuyer ? (
        <div>
          <Label>Purchase Price</Label>
          <div className="mt-1 text-sm text-gray-700">{moneyOrDash(lead.purchase_price)}</div>
        </div>
      ) : (
        <>
          <div>
            <Label>Cost</Label>
            <div className="mt-1 text-sm text-gray-700">{moneyOrDash(lead.cost)}</div>
          </div>
          <div>
            <Label>Revenue</Label>
            <div className="mt-1 text-sm text-gray-700">{moneyOrDash(lead.revenue)}</div>
          </div>
          <div>
            <LabelWithHint
              label="Gross Profit"
              hint="Sale price minus lead cost when this lead was sold to a buyer."
            />
            <div className="mt-1 text-sm text-gray-700">{moneyOrDash(lead.gross_profit)}</div>
          </div>
          <div>
            <LabelWithHint
              label="Net Profit"
              hint="Revenue generated from rev share or profit share, minus lead cost."
            />
            <div className="mt-1 text-sm text-gray-700">{moneyOrDash(lead.net_profit)}</div>
          </div>
        </>
      )}
    </div>
  );
}

export function LeadDetailDrawer() {
  const leadId = useUIStore((s) => s.detailLeadId);
  const requestedDetailLeadId = useUIStore((s) => s.requestedDetailLeadId);
  const close = useUIStore((s) => s.closeDetail);
  const completeDetailSwitch = useUIStore((s) => s.completeDetailSwitch);
  const abortDetailSwitch = useUIStore((s) => s.abortDetailSwitch);
  const qc = useQueryClient();
  const { data: lead, isLoading, isError, error } = useLead(leadId);
  const closeHandlerRef = useRef<(() => void) | null>(null);
  const flushHandlerRef = useRef<(() => Promise<boolean>) | null>(null);

  useEffect(() => {
    if (isError) {
      qc.invalidateQueries({ queryKey: ["leads"] });
    }
  }, [isError, qc]);

  useEffect(() => {
    if (requestedDetailLeadId == null) return;
    void (async () => {
      const flush = flushHandlerRef.current;
      const ok = flush ? await flush() : true;
      if (ok) completeDetailSwitch();
      else abortDetailSwitch();
    })();
  }, [requestedDetailLeadId, completeDetailSwitch, abortDetailSwitch]);

  function handleSheetClose() {
    if (closeHandlerRef.current) {
      closeHandlerRef.current();
    } else {
      close();
    }
  }

  return (
    <Sheet open={!!leadId} onClose={handleSheetClose} width={560}>
      {isError ? (
        <div className="px-6 py-20 text-center text-sm text-gray-400">
          {leadDrawerErrorMessage(error)}
        </div>
      ) : isLoading || !lead ? (
        <div className="flex justify-center py-20">
          <Spinner className="h-6 w-6" />
        </div>
      ) : (
        <DrawerContent
          lead={lead}
          onClose={close}
          registerCloseHandler={(fn) => {
            closeHandlerRef.current = fn;
          }}
          registerFlushHandler={(fn) => {
            flushHandlerRef.current = fn;
          }}
        />
      )}
    </Sheet>
  );
}

function leadDrawerErrorMessage(err: unknown): string {
  const e = apiError(err);
  if (e.code === "forbidden") {
    return "You don't have permission to view this lead.";
  }
  if (e.code === "not_found") {
    return "This lead is no longer available.";
  }
  return errorMessage(err);
}

function DrawerContent({
  lead,
  onClose,
  registerCloseHandler,
  registerFlushHandler,
}: {
  lead: Lead;
  onClose: () => void;
  registerCloseHandler: (fn: (() => void) | null) => void;
  registerFlushHandler: (fn: (() => Promise<boolean>) | null) => void;
}) {
  const user = useAuthStore((s) => s.user);
  const canEdit = canEditLead(user, lead);
  const [tab, setTab] = useState<DrawerTab>("details");
  const edgeTouchStart = useRef<number | null>(null);
  const closingRef = useRef(false);
  const fieldsTouchedRef = useRef(false);
  const [confirmDelete, setConfirmDelete] = useState(false);
  const update = useUpdateLead();
  const { data: leadCall } = useLeadCall(lead.id, user?.account_type);
  const drawerTabs = leadCall
    ? [...DRAWER_TABS, { id: "call" as const, label: "Call" }]
    : DRAWER_TABS;
  const removeLead = useDeleteLead();
  const { data: customFields } = useCustomFields();
  const { data: customFieldFolders } = useCustomFieldFolders();
  const { collapsed, toggle: toggleFolder } = useFolderCollapse(user?.account_id);
  const { data: mapsStatus } = useGoogleMapsStatus();

  useEffect(() => {
    setTab("details");
  }, [lead.id]);

  function onDrawerTouchStart(e: React.TouchEvent) {
    if (e.touches[0].clientX <= 24) {
      edgeTouchStart.current = e.touches[0].clientX;
    }
  }

  function onDrawerTouchMove(e: React.TouchEvent) {
    if (edgeTouchStart.current === null) return;
    if (e.touches[0].clientX - edgeTouchStart.current > 80) {
      edgeTouchStart.current = null;
      void handleClose();
    }
  }

  function onDrawerTouchEnd() {
    edgeTouchStart.current = null;
  }
  const mapsConnected = mapsStatus?.connected === true;

  const [fields, setFieldsState] = useState<Record<string, string>>(() => contactFieldsFromLead(lead));
  const [mapOpen, setMapOpen] = useState(false);
  const fieldsRef = useRef(fields);
  const leadRef = useRef(lead);
  fieldsRef.current = fields;
  leadRef.current = lead;

  function setFields(next: SetStateAction<Record<string, string>>) {
    fieldsTouchedRef.current = true;
    setFieldsState(next);
  }

  useEffect(() => {
    fieldsTouchedRef.current = false;
    setFieldsState(contactFieldsFromLead(lead));
  }, [lead.id]);

  useEffect(() => {
    if (fieldsTouchedRef.current) return;
    setFieldsState(contactFieldsFromLead(lead));
  }, [lead]);

  const flushContactFields = useCallback(async (): Promise<boolean> => {
    if (!canEditLead(user, leadRef.current)) return true;
    const body = dirtyContactPatch(fieldsRef.current, leadRef.current);
    if (!body) return true;

    const toastId = toast.progress("Saving…");
    try {
      await update.mutateAsync({ leadId: leadRef.current.id, body });
      toast.update(toastId, "Saved");
      setTimeout(() => toast.dismiss(toastId), 1500);
      fieldsTouchedRef.current = false;
      return true;
    } catch (e) {
      toast.dismiss(toastId);
      toast.error(errorMessage(e));
      return false;
    }
  }, [update, user]);

  const handleClose = useCallback(async () => {
    if (closingRef.current) return;
    closingRef.current = true;
    try {
      const ok = await flushContactFields();
      if (ok) onClose();
    } finally {
      closingRef.current = false;
    }
  }, [flushContactFields, onClose]);

  useEffect(() => {
    registerCloseHandler(() => {
      void handleClose();
    });
    registerFlushHandler(flushContactFields);
    return () => {
      registerCloseHandler(null);
      registerFlushHandler(null);
    };
  }, [handleClose, flushContactFields, registerCloseHandler, registerFlushHandler]);

  const formattedAddress = formatLeadAddress(fields);

  function saveValidatedAddress(validated: {
    address: string;
    city: string;
    state: string;
    zip: string;
    country: string;
    address_place_id: string;
  }) {
    setFields((prev) => ({ ...prev, ...validated }));
    update.mutate(
      { leadId: lead.id, body: { fields: validated } },
      { onSuccess: () => toast.success("Saved"), onError: (e) => toast.error(errorMessage(e)) }
    );
  }

  function saveAddressField(key: keyof typeof fields, opts?: { silent?: boolean }) {
    setFields((prev) => ({ ...prev, address_place_id: "" }));
    update.mutate(
      {
        leadId: lead.id,
        body: {
          fields: {
            [key]: fieldsRef.current[key],
            address_place_id: null,
          },
        },
      },
      {
        onSuccess: () => {
          if (!opts?.silent) toast.success("Saved");
        },
        onError: (e) => toast.error(errorMessage(e)),
      }
    );
  }

  function saveField(key: string, opts?: { silent?: boolean }) {
    update.mutate(
      { leadId: lead.id, body: { fields: { [key]: fieldsRef.current[key] } } },
      {
        onSuccess: () => {
          if (!opts?.silent) toast.success("Saved");
        },
        onError: (e) => toast.error(errorMessage(e)),
      }
    );
  }

  const saveFieldSilent = (key: string) => saveField(key, { silent: true });
  const saveAddressFieldSilent = (key: keyof typeof fields) => saveAddressField(key, { silent: true });

  async function handleDeleteLead() {
    try {
      await removeLead.mutateAsync(lead.id);
      toast.success("Lead deleted");
      setConfirmDelete(false);
      onClose();
    } catch (err) {
      toast.error(errorMessage(err));
    }
  }

  return (
    <div
      className="flex h-full flex-col"
      onTouchStart={onDrawerTouchStart}
      onTouchMove={onDrawerTouchMove}
      onTouchEnd={onDrawerTouchEnd}
    >
      <LeadHeader lead={lead} onClose={() => void handleClose()} />

      <div className="flex overflow-x-auto border-b border-gray-100 px-5 py-[10px]">
        {drawerTabs.map(({ id, label }) => (
          <button
            key={id}
            onClick={() => setTab(id)}
            className={cn(
              "-mb-px shrink-0 border-b-2 px-2.5 py-1.5 text-base font-semibold transition-colors",
              tab === id ? "border-jade-500 text-jade-700" : "border-transparent text-gray-400 hover:text-gray-600"
            )}
          >
            {label}
          </button>
        ))}
      </div>

      <DrawerBody>
        {tab === "details" && (
          <div className="flex flex-col gap-4">
            <CustomFieldsSection
              lead={lead}
              customFields={customFields}
              folders={customFieldFolders}
              collapsed={collapsed}
              onToggleFolder={toggleFolder}
              fields={fields}
              setFields={setFields}
              mapsConnected={mapsConnected}
              formattedAddress={formattedAddress}
              mapOpen={mapOpen}
              setMapOpen={setMapOpen}
              saveField={saveFieldSilent}
              saveAddressField={saveAddressFieldSilent}
              saveValidatedAddress={saveValidatedAddress}
              addressUpdatePending={update.isPending}
            />
            <RedistributeBox lead={lead} />
          </div>
        )}
        {tab === "activity" && <ActivityTab leadId={lead.id} />}
        {tab === "profit" && (
          <LeadEconomics lead={lead} accountType={user?.account_type} />
        )}
        {tab === "call" && <LeadCallTab leadId={lead.id} accountType={user?.account_type} />}
      </DrawerBody>

      {canEdit && (
        <div className="border-t border-gray-100 px-5 py-4">
          <Button variant="danger" size="sm" onClick={() => setConfirmDelete(true)}>
            Delete lead
          </Button>
        </div>
      )}

      <DeleteLeadConfirmDialog
        open={confirmDelete}
        onClose={() => setConfirmDelete(false)}
        count={1}
        loading={removeLead.isPending}
        onConfirm={handleDeleteLead}
      />
    </div>
  );
}

function LeadHeader({ lead, onClose }: { lead: Lead; onClose: () => void }) {
  const user = useAuthStore((s) => s.user);
  const canAssignOthers = canSeeAllLeads(user);
  const update = useUpdateLead();
  const setAction = useSetActionAt();
  const { data: users } = useUsers();

  const canEditPreassignedBuyer =
    canAction(user, ActionContractsPartners) &&
    user?.account_type === "publisher" &&
    lead.status === "review" &&
    !lead.contract_id &&
    lead.owner_account_id === lead.publisher_id;
  const { data: buyers } = useQuery({
    queryKey: ["buyers"],
    queryFn: () => get<BuyerSummary[]>("/publisher/buyers"),
    enabled: canEditPreassignedBuyer,
  });

  const buyerName = lead.buyer_name ?? lead.preassigned_buyer_name ?? null;
  const { collapsed: assignmentCollapsed, toggle: toggleAssignment } = useLeadAssignmentCollapse(
    user?.account_id
  );
  const { data: summaryStages } = useStages(lead.pipeline_id ?? undefined);
  const assigneeLabel = lead.assignee_name ?? "Unassigned";
  const buyerLabel = buyerName ?? "—";
  const stageLabel =
    summaryStages?.find((s) => s.id === lead.stage_id)?.name ?? lead.stage_name ?? "—";
  const currentStageType =
    summaryStages?.find((s) => s.id === lead.stage_id)?.stage_type ?? lead.stage_type;
  const assignmentSummary = `${assigneeLabel} · ${buyerLabel} · ${stageLabel}`;

  const [actionAtLocal, setActionAtLocal] = useState(
    lead.action_at ? isoToDatetimeLocal(lead.action_at) : ""
  );
  useEffect(() => {
    setActionAtLocal(lead.action_at ? isoToDatetimeLocal(lead.action_at) : "");
  }, [lead.action_at]);

  function saveActionAt() {
    const prev = lead.action_at ? isoToDatetimeLocal(lead.action_at) : "";
    if (actionAtLocal === prev) return;
    const payload = actionAtLocal ? new Date(actionAtLocal).toISOString() : null;
    setAction.mutate(
      { leadId: lead.id, action_at: payload },
      { onSuccess: () => toast.success("Saved"), onError: (e) => toast.error(errorMessage(e)) }
    );
  }

  const overdue = lead.action_at && isPast(new Date(lead.action_at));

  return (
    <div className={cn("border-b border-gray-100 px-5 py-3.5", formFieldClass)}>
      <div className="flex items-start justify-between">
        <div className="min-w-0">
          <div className={cn(drawerTitleClass, "text-lg")}>
            {lead.first_name} {lead.last_name}
          </div>
          <div className={drawerSubtitleClass}>
            {leadSourceLabel(lead)} ·{" "}
            {user?.account_type === "buyer" ? formatBuyerStatus(lead) : formatStatus(lead.status, user?.account_type)}
          </div>
        </div>
        <IconButton onClick={onClose} aria-label="Close">
          <X className="h-4 w-4" />
        </IconButton>
      </div>

      <div className="mt-1 flex items-center gap-1">
        <span className="font-mono text-xs text-gray-400">{lead.public_id}</span>
        <IconButton
          aria-label="Copy lead ID"
          className="h-5 w-5"
          onClick={() => copyText(lead.public_id, "Lead ID copied")}
        >
          <Copy className="h-3 w-3" />
        </IconButton>
        {(user?.account_type === "buyer" || lead.buyer_name) && (
          <LeadMessageButton leadId={lead.public_id} size="sm" className="ml-2 h-6 px-2 text-xs" />
        )}
      </div>

      {showActionAtForStage(currentStageType) && (
        <div
          className={cn(
            "mt-3 flex flex-nowrap items-center gap-1 rounded-md border px-2.5 py-1.5",
            overdue ? "border-danger-border bg-danger-bg" : "border-gray-100 bg-gray-100"
          )}
        >
          {lead.action_at && <ActionDot actionAt={lead.action_at} variant="dot" />}
          <Zap
            className={cn(
              "h-3.5 w-3.5 shrink-0",
              overdue ? "text-danger-fg" : "text-gray-700"
            )}
            aria-hidden
          />
          <span
            className={cn(
              "shrink-0 text-xs",
              overdue ? "font-semibold text-danger-fg" : "text-gray-700"
            )}
          >
            Action Date & Time{overdue && " — overdue"}:
          </span>
          <DatetimeFieldInput
            value={actionAtLocal}
            onChange={setActionAtLocal}
            onBlur={saveActionAt}
            disabled={setAction.isPending}
            placeholder={
              <>
                <span className="hidden underline lg:inline">Click to Set</span>
                <span className="underline lg:hidden">Tap to Set</span>
              </>
            }
            className={overdue ? "font-semibold text-danger-fg" : "text-gray-700"}
          />
        </div>
      )}

      {lead.pending_return_at && (
        <ReturnIndicator
          pendingReturnAt={lead.pending_return_at}
          pendingReturnTimezone={lead.pending_return_timezone}
          variant="detail"
        />
      )}

      <button
        type="button"
        onClick={toggleAssignment}
        aria-expanded={!assignmentCollapsed}
        className="mt-3 flex w-full min-h-11 items-center gap-1 py-1 text-left text-xs font-semibold uppercase tracking-wide text-gray-400 hover:text-gray-600"
      >
        {assignmentCollapsed ? (
          <ChevronRight className="h-3.5 w-3.5 shrink-0" />
        ) : (
          <ChevronDown className="h-3.5 w-3.5 shrink-0" />
        )}
        <span className="min-w-0 truncate">
          Assignment & pipeline
          {assignmentCollapsed && (
            <span className="font-normal normal-case tracking-normal text-gray-500">
              {" "}
              · {assignmentSummary}
            </span>
          )}
        </span>
      </button>

      {!assignmentCollapsed && (
        <div className="mt-1 flex flex-col gap-4 sm:flex-row sm:items-start">
          <div className="min-w-0 flex-1">
            <Label>Assigned</Label>
            {canAssignOthers ? (
              <Select
                value={lead.assigned_user_id != null ? String(lead.assigned_user_id) : ""}
                onChange={(e) =>
                  update.mutate(
                    {
                      leadId: lead.id,
                      body: e.target.value
                        ? { assigned_user_id: Number(e.target.value) }
                        : { clear_assignee: true },
                    },
                    {
                      onSuccess: () => toast.success("Saved"),
                      onError: (err) => toast.error(errorMessage(err)),
                    }
                  )
                }
                disabled={update.isPending}
              >
                <option value="">Unassigned</option>
                {(users ?? []).filter((u) => u.status === "active").map((u) => (
                  <option key={u.id} value={u.id}>
                    {u.full_name}
                  </option>
                ))}
              </Select>
            ) : (
              <div className="mt-1 text-sm text-gray-700">
                {lead.assignee_name ? (
                  <span className="truncate">{lead.assignee_name}</span>
                ) : (
                  "Unassigned"
                )}
              </div>
            )}
          </div>
          <div className="min-w-0 flex-1">
            <Label>Buyer</Label>
            {canEditPreassignedBuyer ? (
              <Select
                value={lead.preassigned_buyer_id != null ? String(lead.preassigned_buyer_id) : ""}
                onChange={(e) => {
                  const val = e.target.value;
                  update.mutate(
                    {
                      leadId: lead.id,
                      body: val
                        ? { preassigned_buyer_id: Number(val) }
                        : { clear_preassigned_buyer: true },
                    },
                    {
                      onSuccess: () => toast.success("Saved"),
                      onError: (err) => toast.error(errorMessage(err)),
                    }
                  );
                }}
                disabled={update.isPending}
              >
                <option value="">None</option>
                {(buyers ?? []).map((b) => (
                  <option key={b.id} value={b.id}>
                    {b.name}
                  </option>
                ))}
              </Select>
            ) : (
              <div className="mt-1 text-sm text-gray-700">
                {buyerName ? <span className="truncate">{buyerName}</span> : "—"}
              </div>
            )}
          </div>
        </div>
      )}

      <LeadPipelineHeader lead={lead} collapsed={assignmentCollapsed} />
    </div>
  );
}

function LeadPipelineHeader({ lead, collapsed = false }: { lead: Lead; collapsed?: boolean }) {
  const user = useAuthStore((s) => s.user);
  const canEditPipeline = canEditLead(user, lead);

  const changeStage = useChangeStage();
  const { data: pipelines } = usePipelines();
  const [pipelineId, setPipelineId] = useState(lead.pipeline_id ?? 0);
  const [stageId, setStageId] = useState(lead.stage_id ?? 0);
  const { data: stages, isFetching: stagesFetching, isFetched: stagesFetched } = useStages(
    pipelineId || undefined
  );
  const stagesReady = !pipelineId || (stagesFetched && !stagesFetching);
  const [prompt, setPrompt] = useState<{ stage: Stage; initialActionAt: string } | null>(null);

  function openStagePrompt(stage: Stage) {
    const fromStageType = stages?.find((s) => s.id === lead.stage_id)?.stage_type;
    setPrompt({
      stage,
      initialActionAt: initialActionAtForStageMove(fromStageType, stage.stage_type, lead.action_at),
    });
  }

  useEffect(() => {
    setPipelineId(lead.pipeline_id ?? 0);
    setStageId(lead.stage_id ?? 0);
  }, [lead.id, lead.pipeline_id, lead.stage_id]);

  function revertFromLead() {
    setPipelineId(lead.pipeline_id ?? 0);
    setStageId(lead.stage_id ?? 0);
  }

  function commitStage(stage: Stage, extra?: PromptResult) {
    if (stage.pipeline_id !== pipelineId) return;
    changeStage.mutate(
      { leadId: lead.id, payload: { stage_id: stage.id, ...extra } },
      {
        onSuccess: () => {
          setPrompt(null);
          toast.success("Saved");
        },
        onError: (err) => {
          const e = apiError(err);
          if (stagePromptMissingError(e.code, e.message, stage.stage_type)) {
            openStagePrompt(stage);
          } else {
            toast.error(errorMessage(err));
            revertFromLead();
          }
        },
      }
    );
  }

  function onPipelineChange(nextPipelineId: number) {
    if (nextPipelineId === (lead.pipeline_id ?? 0)) {
      revertFromLead();
      return;
    }
    setPipelineId(nextPipelineId);
    if (nextPipelineId === 0) {
      if (!lead.stage_id && !lead.pipeline_id) return;
      changeStage.mutate(
        { leadId: lead.id, payload: { clear: true } },
        {
          onSuccess: () => {
            setStageId(0);
            toast.success("Saved");
          },
          onError: (err) => {
            toast.error(errorMessage(err));
            revertFromLead();
          },
        }
      );
      return;
    }
    setStageId(0);
  }

  function onStageChange(nextStageId: number) {
    if (nextStageId === 0) return;
    if (nextStageId === (lead.stage_id ?? 0) && pipelineId === (lead.pipeline_id ?? 0)) return;
    const stage = stages?.find((s) => s.id === nextStageId);
    if (!stage || stage.pipeline_id !== pipelineId) return;
    setStageId(nextStageId);
    if (stageNeedsPrompt(stage.stage_type)) {
      openStagePrompt(stage);
      return;
    }
    commitStage(stage);
  }

  const currentStageIndex = (stages ?? []).findIndex((s) => s.id === stageId);
  const showStageBar = pipelineId > 0 && stagesReady && (stages?.length ?? 0) > 0;

  return (
    <div className={collapsed ? undefined : "mt-3"}>
      {!collapsed && canEditPipeline && (
        <div>
          <Label>Pipeline</Label>
          <Select
            value={pipelineId}
            onChange={(e) => onPipelineChange(Number(e.target.value))}
            disabled={changeStage.isPending}
          >
            <option value={0}>None</option>
            {(pipelines ?? []).map((p) => (
              <option key={p.id} value={p.id}>
                {p.name}
              </option>
            ))}
          </Select>
        </div>
      )}
      {!collapsed && showStageBar && (
        <>
          <div className="mt-[25px] flex gap-1 overflow-x-auto pb-5">
            {(stages ?? []).map((s, i) => {
              const reached = currentStageIndex >= 0 && i <= currentStageIndex;
              const isCurrent = s.id === stageId;
              return (
                <span key={s.id} className="relative min-w-[3rem] shrink-0 flex-1">
                  <button
                    type="button"
                    disabled={!canEditPipeline || changeStage.isPending || !stagesReady}
                    onClick={() => onStageChange(s.id)}
                    className={cn(
                      "h-6 w-full rounded-sm transition-colors",
                      !reached && "border border-gray-200 bg-gray-200/60",
                      reached && !isCurrent && cn("border", stageColorFill(s.color), stageColorBorder(s.color)),
                      isCurrent && cn("border", stageColorFill(s.color), stageColorBorder(s.color)),
                      canEditPipeline ? "cursor-pointer hover:opacity-80" : "cursor-default"
                    )}
                  />
                  <span
                    className={cn(
                      "pointer-events-none absolute left-1/2 top-full mt-1 -translate-x-1/2 whitespace-nowrap text-xs",
                      isCurrent ? "text-gray-300" : "text-gray-500/70"
                    )}
                  >
                    {s.name}
                  </span>
                </span>
              );
            })}
          </div>
        </>
      )}
      <StagePromptModal
        key={prompt ? `${lead.id}-${prompt.stage.id}` : "closed"}
        open={!!prompt}
        stage={prompt?.stage ?? null}
        initialActionAt={prompt?.initialActionAt}
        onCancel={() => {
          setPrompt(null);
          revertFromLead();
        }}
        onConfirm={(r) => {
          if (prompt) commitStage(prompt.stage, r);
        }}
      />
    </div>
  );
}

function ContactSection({
  lead,
  customFields,
  builtinOrder,
  fields,
  setFields,
  mapsConnected,
  formattedAddress,
  mapOpen,
  setMapOpen,
  saveField,
  saveAddressField,
  saveValidatedAddress,
  addressUpdatePending,
}: {
  lead: Lead;
  customFields: CustomField[];
  builtinOrder: ContactFieldKey[];
  fields: Record<string, string>;
  setFields: Dispatch<SetStateAction<Record<string, string>>>;
  mapsConnected: boolean;
  formattedAddress: string;
  mapOpen: boolean;
  setMapOpen: (open: boolean) => void;
  saveField: (key: string) => void;
  saveAddressField: (key: keyof typeof fields) => void;
  saveValidatedAddress: (validated: {
    address: string;
    city: string;
    state: string;
    zip: string;
    country: string;
    address_place_id: string;
  }) => void;
  addressUpdatePending: boolean;
}) {
  const renderBuiltin = (key: ContactFieldKey) => {
    switch (key) {
      case "first_name":
        return (
          <div key={key}>
            <Label>First Name</Label>
            <InputWithOverflowTooltip
              linkify
              value={fields.first_name ?? ""}
              onChange={(e) => setFields((f) => ({ ...f, first_name: e.target.value }))}
              onBlur={() => saveField("first_name")}
            />
          </div>
        );
      case "last_name":
        return (
          <div key={key}>
            <Label>Last Name</Label>
            <InputWithOverflowTooltip
              linkify
              value={fields.last_name ?? ""}
              onChange={(e) => setFields((f) => ({ ...f, last_name: e.target.value }))}
              onBlur={() => saveField("last_name")}
            />
          </div>
        );
      case "phone":
        return (
          <div key={key}>
            <Label>Phone</Label>
            <InputWithOverflowTooltip
              linkify
              value={fields.phone ?? ""}
              onChange={(e) => setFields((f) => ({ ...f, phone: e.target.value }))}
              onBlur={() => saveField("phone")}
            />
          </div>
        );
      case "email":
        return (
          <div key={key}>
            <Label>Email</Label>
            <InputWithOverflowTooltip
              linkify
              value={fields.email ?? ""}
              onChange={(e) => setFields((f) => ({ ...f, email: e.target.value }))}
              onBlur={() => saveField("email")}
            />
          </div>
        );
      case "address":
        return (
          <div key={key} className="flex flex-col gap-2.5">
            {mapsConnected && fields.address_place_id && formattedAddress && (
              <div>
                <Label>Verified address</Label>
                <div className="mt-1 flex items-center gap-1.5">
                  <MapPin className="h-3.5 w-3.5 shrink-0 text-indigo-600" />
                  <ValidatedAddressLink formatted={formattedAddress} onClick={() => setMapOpen(true)} />
                </div>
              </div>
            )}
            {mapsConnected && !fields.address_place_id && formattedAddress && (
              <p className="text-xs text-amber-700">
                Select an address from suggestions to verify and enable map.
              </p>
            )}
            <AddressAutocomplete
              address={fields.address ?? ""}
              city={fields.city ?? ""}
              state={fields.state ?? ""}
              zip={fields.zip ?? ""}
              country={fields.country ?? ""}
              disabled={addressUpdatePending}
              onPlainChange={(next) => setFields((prev) => ({ ...prev, ...next }))}
              onFieldBlur={(key) => saveAddressField(key)}
              onSelect={saveValidatedAddress}
            />
          </div>
        );
      case "tags":
        return <LeadTagsEditor key={key} leadId={lead.id} tags={lead.tags ?? []} />;
    }
  };

  return (
    <div className="flex flex-col gap-2.5">
      {builtinOrder.map(renderBuiltin)}
      {customFields.map((f) => (
        <div key={f.id}>
          <Label>{f.name}</Label>
          <CustomFieldValue
            leadId={lead.id}
            fieldId={f.id}
            type={f.type}
            format={f.format}
            options={f.options}
            value={lead.custom_values?.[String(f.id)]}
          />
        </div>
      ))}
      <AddressMapDialog
        open={mapOpen}
        onClose={() => setMapOpen(false)}
        placeId={fields.address_place_id ?? ""}
        formattedAddress={formattedAddress}
      />
    </div>
  );
}

function CustomFieldsSection({
  lead,
  customFields,
  folders,
  collapsed,
  onToggleFolder,
  fields,
  setFields,
  mapsConnected,
  formattedAddress,
  mapOpen,
  setMapOpen,
  saveField,
  saveAddressField,
  saveValidatedAddress,
  addressUpdatePending,
}: {
  lead: Lead;
  customFields: CustomField[] | undefined;
  folders: CustomFieldFolder[] | undefined;
  collapsed: Record<string, boolean>;
  onToggleFolder: (folderId: number) => void;
  fields: Record<string, string>;
  setFields: Dispatch<SetStateAction<Record<string, string>>>;
  mapsConnected: boolean;
  formattedAddress: string;
  mapOpen: boolean;
  setMapOpen: (open: boolean) => void;
  saveField: (key: string) => void;
  saveAddressField: (key: keyof typeof fields) => void;
  saveValidatedAddress: (validated: {
    address: string;
    city: string;
    state: string;
    zip: string;
    country: string;
    address_place_id: string;
  }) => void;
  addressUpdatePending: boolean;
}) {
  const activeFields = (customFields ?? []).filter((f) => f.is_active);
  const grouped = groupCustomFieldsByFolder(folders ?? [], activeFields);

  const renderField = (f: CustomField) => (
    <div key={f.id}>
      <Label>{f.name}</Label>
      <CustomFieldValue
        leadId={lead.id}
        fieldId={f.id}
        type={f.type}
        format={f.format}
        options={f.options}
        value={lead.custom_values?.[String(f.id)]}
      />
    </div>
  );

  const visibleFolders = grouped.folders.filter(
    (g) => g.fields.length > 0 || isContactFolder(g.folder)
  );

  if (visibleFolders.length === 0 && grouped.unassigned.length === 0) return null;

  return (
    <div className="flex flex-col gap-4">
      {visibleFolders.map((g, index) => {
        const isCollapsed = !!collapsed[g.folder.id];
        const contact = isContactFolder(g.folder);
        const builtinOrder = resolveContactBuiltinOrder(g.folder.contact_builtin_order);
        return (
          <div key={g.folder.id} className={cn(index > 0 && "border-t border-gray-100 pt-4")}>
            <button
              type="button"
              onClick={() => onToggleFolder(g.folder.id)}
              className="mb-2 flex w-full items-center gap-1 text-left text-xs font-semibold uppercase tracking-wide text-gray-400 hover:text-gray-600"
            >
              {isCollapsed ? (
                <ChevronRight className="h-3.5 w-3.5" />
              ) : (
                <ChevronDown className="h-3.5 w-3.5" />
              )}
              {contact ? "Contact" : g.folder.name}
            </button>
            {!isCollapsed &&
              (contact ? (
                <ContactSection
                  lead={lead}
                  customFields={g.fields}
                  builtinOrder={builtinOrder}
                  fields={fields}
                  setFields={setFields}
                  mapsConnected={mapsConnected}
                  formattedAddress={formattedAddress}
                  mapOpen={mapOpen}
                  setMapOpen={setMapOpen}
                  saveField={saveField}
                  saveAddressField={saveAddressField}
                  saveValidatedAddress={saveValidatedAddress}
                  addressUpdatePending={addressUpdatePending}
                />
              ) : (
                <div className="flex flex-col gap-2.5">{g.fields.map(renderField)}</div>
              ))}
          </div>
        );
      })}
      {grouped.unassigned.length > 0 && (
        <div
          className={cn(
            "flex flex-col gap-2.5",
            visibleFolders.length > 0 && "border-t border-gray-100 pt-4"
          )}
        >
          {grouped.unassigned.map(renderField)}
        </div>
      )}
    </div>
  );
}

function CustomFieldValue({
  leadId,
  fieldId,
  type,
  format,
  options,
  value,
}: {
  leadId: number;
  fieldId: number;
  type: string;
  format?: string | null;
  options: string[];
  value: unknown;
}) {
  const update = useUpdateLead();
  const raw = value == null ? "" : typeof value === "string" ? value : JSON.stringify(value);
  const formatToken = effectiveFieldFormat(type, format);
  const inputMode = type === "date" || type === "datetime" ? inputModeForFormat(type, formatToken) : "text";
  const [val, setVal] = useState(() => {
    if (inputMode === "date") return toNativeDateValue(raw, type, format);
    if (inputMode === "datetime-local") return toNativeDatetimeLocalValue(raw, type, format);
    return raw;
  });

  function save(next: unknown) {
    update.mutate({ leadId, body: { custom_values: { [String(fieldId)]: next } } });
  }

  function saveDateValue(next: string) {
    if (inputMode === "datetime-local") {
      save(fromNativeDatetimeLocal(next, type, format));
      return;
    }
    if (type === "date" || type === "datetime") {
      save(normalizeCustomDateValue(next, type, format));
      return;
    }
    save(next);
  }

  if (type === "dropdown") {
    return (
      <Select
        value={val}
        onChange={(e) => {
          setVal(e.target.value);
          save(e.target.value);
        }}
      >
        <option value="">—</option>
        {(options ?? []).map((o) => (
          <option key={o} value={o}>
            {o}
          </option>
        ))}
      </Select>
    );
  }

  if (type === "date" || type === "datetime") {
    if (inputMode === "datetime-local") {
      return (
        <DatetimeFieldInput
          value={val}
          onChange={(next) => {
            setVal(next);
            saveDateValue(next);
          }}
        />
      );
    }
    return (
      <Input
        value={val}
        type={inputMode}
        placeholder={inputMode === "text" ? formatToken : undefined}
        onChange={(e) => {
          const next = e.target.value;
          setVal(next);
          if (inputMode !== "text") saveDateValue(next);
        }}
        onBlur={() => {
          if (inputMode === "text") saveDateValue(val);
        }}
      />
    );
  }

  return (
    <InputWithOverflowTooltip
      linkify={type === "text"}
      value={val}
      type={type === "number" ? "number" : "text"}
      onChange={(e) => setVal(e.target.value)}
      onBlur={() => save(type === "number" ? Number(val) : val)}
    />
  );
}

function accountLabel(name: string | null | undefined, type: string | null | undefined): string | null {
  if (name) {
    if (type === "buyer") return `Buyer: ${name}`;
    if (type === "publisher") return `Publisher: ${name}`;
    return name;
  }
  if (type === "buyer") return "Buyer";
  if (type === "publisher") return "Publisher";
  return null;
}

function stageChangeHeadline(entry: LeadHistoryEntry): string {
  const from = entry.from_stage_name ?? "Created";
  const to = entry.to_stage_name?.trim();
  if (!to) {
    return from === "Created" ? "Created" : `${from} → (unknown stage)`;
  }
  return `${from} → ${to}`;
}

function transferHeadline(entry: LeadHistoryEntry): string {
  const from = entry.from_account_name ?? "Unknown";
  const to = entry.to_account_name ?? "Unknown";
  switch (entry.transfer_kind) {
    case "returned":
      return `Returned · ${from} → ${to}`;
    case "redistributed":
      return `Redistributed · ${from} → ${to}`;
    default:
      return `Sold · ${from} → ${to}`;
  }
}

function historyHeadline(entry: LeadHistoryEntry): string {
  if (entry.summary) return entry.summary;
  if (entry.kind === "account_transfer") return transferHeadline(entry);
  if (entry.kind === "stage_change") return stageChangeHeadline(entry);
  if (entry.field_name && entry.from_value != null && entry.to_value != null) {
    return `${entry.field_name} · ${entry.from_value} → ${entry.to_value}`;
  }
  return activityKindLabel(entry.kind);
}

function actorTypeLabel(type: string | null | undefined): string {
  switch (type) {
    case "webhook":
      return "Webhook";
    case "integration":
      return "CRM";
    case "route":
      return "Route";
    case "user":
      return "User";
    default:
      return "System";
  }
}

function historyActorLine(entry: LeadHistoryEntry): string {
  const name = entry.actor_name || entry.moved_by_name || "System";
  const type = actorTypeLabel(entry.actor_type);
  return `${name} · ${type}`;
}

function isWebhookHistoryEntry(kind: LeadHistoryEntry["kind"]): boolean {
  return kind === "webhook" || kind === "outbound_webhook";
}

function ActivityTab({ leadId }: { leadId: number }) {
  const navigate = useNavigate();
  const closeDetail = useUIStore((s) => s.closeDetail);
  const accountType = useAuthStore((s) => s.user?.account_type);
  const userId = useAuthStore((s) => s.user?.id);
  const { data: history, isLoading, isError } = useLeadHistory(leadId);
  const { toggleGroup, isVisible } = useActivityGroupFilters(userId);
  const addNote = useAddNote();
  const [body, setBody] = useState("");

  const allHistory = history ?? [];
  const presentGroups = presentActivityGroups(allHistory);
  const visibleHistory = allHistory.filter((h) => isVisible(activityFilterGroup(h.kind)));

  return (
    <div>
      <div className="mb-4">
        <Textarea
          value={body}
          onChange={(e) => setBody(e.target.value)}
          placeholder="Add a note…"
        />
        <div className="mt-1.5 flex justify-end">
          <Button
            size="sm"
            disabled={!body.trim()}
            onClick={() => addNote.mutate({ leadId, body }, { onSuccess: () => setBody("") })}
          >
            Add Note
          </Button>
        </div>
      </div>
      <SectionLabel className="mb-2">Activity</SectionLabel>
      {!isLoading && !isError && presentGroups.length > 0 && (
        <div className="mb-3 flex flex-nowrap gap-3 overflow-x-auto pb-1">
          {presentGroups.map((group) => (
            <label key={group} className="flex shrink-0 cursor-pointer items-center gap-1.5 text-xs text-gray-500">
              <input
                type="checkbox"
                className="rounded"
                checked={isVisible(group)}
                onChange={() => toggleGroup(group)}
              />
              {activityGroupLabel(group)}
            </label>
          ))}
        </div>
      )}
      {isLoading && (
        <div className="flex justify-center py-6">
          <Spinner />
        </div>
      )}
      {isError && (
        <p className="text-sm text-red-500">Could not load activity.</p>
      )}
      {!isLoading && !isError && allHistory.length === 0 && (
        <p className="text-sm text-gray-400">No activity yet.</p>
      )}
      {!isLoading && !isError && allHistory.length > 0 && visibleHistory.length === 0 && (
        <p className="text-sm text-gray-400">No activity matches your filters.</p>
      )}
      {visibleHistory.map((h) => (
        <div key={`${h.kind}-${h.id}`} className="flex items-start gap-2.5 py-1.5 text-sm text-gray-500">
          <span className="mt-1.5 h-2 w-2 shrink-0 rounded-full bg-jade-300" />
          <div className="min-w-0 flex-1">
            <div className="mb-0.5 flex flex-wrap items-center gap-2">
              <span className="rounded bg-gray-100 px-1.5 py-0.5 text-[10px] font-semibold uppercase tracking-wide text-gray-500">
                {activityKindLabel(h.kind)}
              </span>
              {h.status && h.status !== "success" && (
                <span className="text-[10px] font-medium uppercase text-amber-600">{h.status}</span>
              )}
            </div>
            {h.kind === "stage_change" ? (
              <div>
                {(() => {
                  const headline = stageChangeHeadline(h);
                  const arrowIdx = headline.indexOf(" → ");
                  if (arrowIdx === -1) {
                    return <span className="font-medium">{headline}</span>;
                  }
                  return (
                    <>
                      {headline.slice(0, arrowIdx + 3)}
                      <span className="font-medium">{headline.slice(arrowIdx + 3)}</span>
                    </>
                  );
                })()}
              </div>
            ) : (
              <div className={cn("font-medium", h.kind === "note_added" && "whitespace-pre-wrap")}>
                {isWebhookHistoryEntry(h.kind) && accountType ? (
                  (() => {
                    const logUrl = buildWebhookActivityLogUrl(accountType, h, leadId);
                    const headline = historyHeadline(h);
                    if (!logUrl) {
                      return <LinkifiedText text={headline} />;
                    }
                    return (
                      <button
                        type="button"
                        className="text-left text-jade-600 hover:underline"
                        onClick={() => {
                          closeDetail();
                          navigate(logUrl);
                        }}
                      >
                        <LinkifiedText text={headline} />
                      </button>
                    );
                  })()
                ) : (
                  <LinkifiedText text={historyHeadline(h)} />
                )}
              </div>
            )}
            <div className="text-xs text-gray-400">
              {historyActorLine(h)} · {format(new Date(h.created_at), "MMM d, h:mma")}
              {h.action_at_captured &&
                ` · Action Date & Time ${format(new Date(h.action_at_captured), "MMM d, h:mm a")}`}
              {h.disqualification_reason && ` · ${h.disqualification_reason}`}
              {h.trigger_label && ` · ${h.trigger_label}`}
              {h.actor_detail && h.kind !== "account_transfer" && ` · ${h.actor_detail}`}
              {(() => {
                const label = accountLabel(h.account_name, h.account_type);
                return label ? ` · ${label}` : null;
              })()}
            </div>
          </div>
        </div>
      ))}
    </div>
  );
}

function RedistributeBox({ lead }: { lead: Lead }) {
  const user = useAuthStore((s) => s.user);
  const [disputing, setDisputing] = useState(false);
  if (user?.account_type !== "publisher" || lead.status !== "returned") return null;
  return (
    <div className="rounded-md border border-warning-border bg-warning-bg p-2.5">
      <div className="mb-0.5 flex items-center gap-2">
        <Badge variant="returned" plain>Returned</Badge>
        <span className="text-xs font-semibold text-warning-fg">Re-distribute this lead</span>
      </div>
      <p className="text-xs text-gray-400">
        Send this returned lead to another buyer from the Contracts page.
      </p>
      {canAction(user, ActionBilling) && (
        <Button size="sm" variant="outline" className="mt-2" onClick={() => setDisputing(true)}>
          Dispute return
        </Button>
      )}
      {disputing && <DisputeReturnDialog lead={lead} onClose={() => setDisputing(false)} />}
    </div>
  );
}

function DisputeReturnDialog({ lead, onClose }: { lead: Lead; onClose: () => void }) {
  const openDispute = useOpenReturnDispute();
  const [reason, setReason] = useState("");
  const [deadlineDays, setDeadlineDays] = useState(14);
  return (
    <FormDrawer
      open
      onClose={onClose}
      title="Dispute return"
      subtitle={`Charge the buyer and open a dispute for ${lead.first_name} ${lead.last_name}.`}
      footer={
        <>
          <Button variant="secondary" onClick={onClose}>
            Cancel
          </Button>
          <Button
            disabled={!reason.trim() || openDispute.isPending}
            onClick={() =>
              openDispute.mutate(
                { leadId: lead.id, reason, deadlineDays },
                {
                  onSuccess: () => {
                    toast.success("Dispute opened");
                    onClose();
                  },
                  onError: (e) => toast.error(errorMessage(e)),
                }
              )
            }
          >
            Open dispute
          </Button>
        </>
      }
    >
      <div className="flex flex-col gap-3">
        <div>
          <Label>Reason</Label>
          <Textarea value={reason} onChange={(e) => setReason(e.target.value)} rows={4} />
        </div>
        <div>
          <Label>Response deadline</Label>
          <Select value={deadlineDays} onChange={(e) => setDeadlineDays(Number(e.target.value))}>
            {DEADLINE_DAY_OPTIONS.map((d) => (
              <option key={d} value={d}>
                {d} days
              </option>
            ))}
          </Select>
        </div>
      </div>
    </FormDrawer>
  );
}
