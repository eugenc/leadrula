import { useEffect, useState } from "react";
import { Sheet, DrawerHeader, DrawerBody, FormDrawer } from "@/components/ui/dialog";
import { Button } from "@/components/ui/button";
import { Input, InputWithOverflowTooltip, Label, Textarea, Select } from "@/components/ui/input";
import { Avatar, Badge, Spinner } from "@/components/ui/misc";
import { SectionLabel } from "@/components/layout/SectionLabel";
import { ActionDot } from "./ActionDot";
import { format, isPast } from "date-fns";
import { CircleHelp, MapPin, ChevronDown, ChevronRight } from "lucide-react";
import { cn, formatMoney } from "@/lib/utils";
import { useUIStore } from "@/store/uiStore";
import { useAuthStore } from "@/store/authStore";
import { toast } from "@/store/toastStore";
import { errorMessage, apiError } from "@/lib/api";
import {
  useLead,
  useNotes,
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
import { useOpenReturnDispute } from "@/features/admin/hooks";
import { DEADLINE_DAY_OPTIONS } from "@/features/billing/disputeOptions";
import { StagePromptModal, type PromptResult } from "./StagePromptModal";
import {
  initialActionAtForStageMove,
  stageNeedsPrompt,
  stagePromptMissingError,
} from "@/features/pipelines/stageTypes";
import type { CustomField, Lead, LeadHistoryEntry, Stage } from "@/types";
import { formatStatus, leadSourceLabel } from "./leadsListColumns";
import { LeadTagsEditor } from "./LeadTagsEditor";
import { useQuery } from "@tanstack/react-query";
import { get } from "@/lib/api";
import type { BuyerSummary } from "@/types";
import { effectiveFieldFormat } from "@/features/admin/customFieldConstants";
import { groupCustomFieldsByFolder } from "@/features/admin/customFieldLayout";
import { useFolderCollapse } from "./customFieldFolderCollapse";
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

const BUILTINS: { key: keyof Lead; label: string }[] = [
  { key: "first_name", label: "First Name" },
  { key: "last_name", label: "Last Name" },
  { key: "phone", label: "Phone" },
  { key: "email", label: "Email" },
];

const DRAWER_TABS = [
  { id: "details", label: "Details" },
  { id: "notes", label: "Notes" },
  { id: "history", label: "History" },
  { id: "profit", label: "Profit" },
] as const;

type DrawerTab = (typeof DRAWER_TABS)[number]["id"];

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
  const close = useUIStore((s) => s.closeDetail);
  const { data: lead, isLoading, isError } = useLead(leadId);

  return (
    <Sheet open={!!leadId} onClose={close}>
      {isError ? (
        <div className="px-6 py-20 text-center text-sm text-gray-400">
          This lead is no longer available.
        </div>
      ) : isLoading || !lead ? (
        <div className="flex justify-center py-20">
          <Spinner className="h-6 w-6" />
        </div>
      ) : (
        <DrawerContent lead={lead} onClose={close} />
      )}
    </Sheet>
  );
}

function DrawerContent({ lead, onClose }: { lead: Lead; onClose: () => void }) {
  const user = useAuthStore((s) => s.user);
  const isAdmin = user?.role === "admin";
  const [tab, setTab] = useState<DrawerTab>("details");
  const [confirmDelete, setConfirmDelete] = useState(false);
  const update = useUpdateLead();
  const setAction = useSetActionAt();
  const removeLead = useDeleteLead();
  const { data: users } = useUsers();
  const { data: customFields } = useCustomFields();
  const { data: customFieldFolders } = useCustomFieldFolders();
  const { collapsed, toggle: toggleFolder } = useFolderCollapse(user?.account_id);
  const { data: mapsStatus } = useGoogleMapsStatus();
  const mapsConnected = mapsStatus?.connected === true;

  const canEditPreassignedBuyer =
    isAdmin &&
    user?.account_type === "publisher" &&
    lead.status === "review" &&
    !lead.contract_id &&
    lead.owner_account_id === lead.publisher_id;
  const { data: buyers } = useQuery({
    queryKey: ["buyers"],
    queryFn: () => get<BuyerSummary[]>("/publisher/buyers"),
    enabled: canEditPreassignedBuyer,
  });

  const [fields, setFields] = useState<Record<string, string>>({});
  const [actionAtLocal, setActionAtLocal] = useState("");
  const [mapOpen, setMapOpen] = useState(false);
  useEffect(() => {
    const f: Record<string, string> = {};
    for (const b of BUILTINS) f[b.key as string] = (lead[b.key] as string) ?? "";
    f.address = lead.address ?? "";
    f.city = lead.city ?? "";
    f.state = lead.state ?? "";
    f.zip = lead.zip ?? "";
    f.country = lead.country ?? "";
    f.address_place_id = lead.address_place_id ?? "";
    setFields(f);
    setActionAtLocal(lead.action_at ? isoToDatetimeLocal(lead.action_at) : "");
  }, [lead]);

  const formattedAddress = formatLeadAddress(lead);

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

  function saveAddressField(key: keyof typeof fields) {
    update.mutate(
      {
        leadId: lead.id,
        body: {
          fields: {
            [key]: fields[key],
            address_place_id: null,
          },
        },
      },
      { onSuccess: () => toast.success("Saved"), onError: (e) => toast.error(errorMessage(e)) }
    );
  }

  function saveField(key: string) {
    update.mutate(
      { leadId: lead.id, body: { fields: { [key]: fields[key] } } },
      { onSuccess: () => toast.success("Saved"), onError: (e) => toast.error(errorMessage(e)) }
    );
  }

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
    <div className="flex h-full flex-col">
      <DrawerHeader
        title={`${lead.first_name} ${lead.last_name}`}
        subtitle={`${leadSourceLabel(lead)} · ${formatStatus(lead.status)}`}
        detail={lead.public_id}
        onClose={onClose}
      />

      <div className="border-b border-gray-100 px-5 py-2">
        <div
          className={cn(
            "flex items-center gap-2 rounded-md border px-2.5 py-1.5",
            overdue ? "border-danger-border bg-danger-bg" : "border-gray-100 bg-gray-100"
          )}
        >
          {lead.action_at && <ActionDot actionAt={lead.action_at} variant="dot" />}
          <span
            className={cn(
              "shrink-0 text-xs",
              overdue ? "font-semibold text-danger-fg" : "text-gray-700"
            )}
          >
            Action Date & Time{overdue && " — overdue"}
          </span>
          <DatetimeFieldInput
            value={actionAtLocal}
            onChange={setActionAtLocal}
            onBlur={saveActionAt}
            disabled={setAction.isPending}
            className={overdue ? "font-semibold text-danger-fg" : "text-gray-700"}
          />
        </div>
      </div>

      <div className="flex border-b border-gray-100 px-5">
        {DRAWER_TABS.map(({ id, label }) => (
          <button
            key={id}
            onClick={() => setTab(id)}
            className={cn(
              "-mb-px border-b-2 px-2.5 py-1.5 text-sm font-semibold transition-colors",
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
            <div>
              <SectionLabel className="mb-2">Lead</SectionLabel>
              <div className="flex flex-col gap-2.5">
                {user?.role === "admin" ? (
                  <div>
                    <Label>Assigned To</Label>
                    <Select
                      value={lead.assigned_user_id ?? ""}
                      onChange={(e) =>
                        update.mutate({
                          leadId: lead.id,
                          body: e.target.value
                            ? { assigned_user_id: Number(e.target.value) }
                            : { clear_assignee: true },
                        })
                      }
                    >
                      <option value="">Unassigned</option>
                      {(users ?? []).filter((u) => u.status === "active").map((u) => (
                        <option key={u.id} value={u.id}>
                          {u.full_name}
                        </option>
                      ))}
                    </Select>
                  </div>
                ) : (
                  <div>
                    <Label>Assigned To</Label>
                    <div className="mt-1 flex items-center gap-2 text-sm text-gray-700">
                      {lead.assignee_name ? (
                        <>
                          <Avatar
                            name={lead.assignee_name}
                            src={lead.assignee_avatar_url}
                            variant="card"
                          />
                          {lead.assignee_name}
                        </>
                      ) : (
                        "Unassigned"
                      )}
                    </div>
                  </div>
                )}
                {canEditPreassignedBuyer ? (
                  <div>
                    <Label>Buyer</Label>
                    <Select
                      value={lead.preassigned_buyer_id ?? ""}
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
                  </div>
                ) : (
                  <div>
                    <Label>Buyer</Label>
                    <div className="mt-1 text-sm text-gray-700">
                      {lead.buyer_name ?? lead.preassigned_buyer_name ?? "—"}
                    </div>
                  </div>
                )}
                <LeadPipelineFields lead={lead} />
              </div>
            </div>
            <LeadTagsEditor leadId={lead.id} tags={lead.tags ?? []} />
            <div>
              <SectionLabel className="mb-2">Contact</SectionLabel>
              <div className="flex flex-col gap-2.5">
                {BUILTINS.map((b) => (
                  <div key={b.key as string}>
                    <Label>{b.label}</Label>
                    <InputWithOverflowTooltip
                      value={fields[b.key as string] ?? ""}
                      onChange={(e) => setFields((f) => ({ ...f, [b.key as string]: e.target.value }))}
                      onBlur={() => saveField(b.key as string)}
                    />
                  </div>
                ))}
                {mapsConnected && lead.address_place_id && formattedAddress && (
                  <div>
                    <Label>Verified address</Label>
                    <div className="mt-1 flex items-center gap-1.5">
                      <MapPin className="h-3.5 w-3.5 shrink-0 text-indigo-600" />
                      <ValidatedAddressLink
                        formatted={formattedAddress}
                        onClick={() => setMapOpen(true)}
                      />
                    </div>
                  </div>
                )}
                {mapsConnected && !lead.address_place_id && formattedAddress && (
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
                  disabled={update.isPending}
                  onPlainChange={(next) => setFields((prev) => ({ ...prev, ...next }))}
                  onFieldBlur={(key) => saveAddressField(key)}
                  onSelect={saveValidatedAddress}
                />
              </div>
            </div>
            <AddressMapDialog
              open={mapOpen}
              onClose={() => setMapOpen(false)}
              placeId={lead.address_place_id ?? ""}
              formattedAddress={formattedAddress}
            />
            <CustomFieldsSection
              lead={lead}
              customFields={customFields}
              folders={customFieldFolders}
              collapsed={collapsed}
              onToggleFolder={toggleFolder}
            />
            <RedistributeBox lead={lead} />
          </div>
        )}
        {tab === "notes" && <NotesTab leadId={lead.id} />}
        {tab === "history" && <HistoryTab leadId={lead.id} />}
        {tab === "profit" && (
          <LeadEconomics lead={lead} accountType={user?.account_type} />
        )}
      </DrawerBody>

      {isAdmin && (
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

function LeadPipelineFields({ lead }: { lead: Lead }) {
  const user = useAuthStore((s) => s.user);
  const canEditPipeline =
    user?.role === "admin" ||
    (user?.role === "user" && lead.assigned_user_id === Number(user.id));

  const changeStage = useChangeStage();
  const { data: pipelines } = usePipelines();
  const [pipelineId, setPipelineId] = useState(lead.pipeline_id ?? 0);
  const [stageId, setStageId] = useState(lead.stage_id ?? 0);
  const { data: stages } = useStages(pipelineId || undefined);
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
    if (!stage) return;
    setStageId(nextStageId);
    if (stageNeedsPrompt(stage.stage_type)) {
      openStagePrompt(stage);
      return;
    }
    commitStage(stage);
  }

  if (!canEditPipeline) {
    return (
      <>
        <div>
          <Label>Pipeline</Label>
          <div className="mt-1 text-sm text-gray-700">{lead.pipeline_name ?? "—"}</div>
        </div>
        <div>
          <Label>Pipeline Stage</Label>
          <div className="mt-1 text-sm text-gray-700">{lead.stage_name ?? "—"}</div>
        </div>
      </>
    );
  }

  return (
    <>
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
      {pipelineId > 0 && (
        <div>
          <Label>Pipeline Stage</Label>
          <Select
            value={stageId}
            onChange={(e) => onStageChange(Number(e.target.value))}
            disabled={changeStage.isPending}
          >
            <option value={0}>Select stage…</option>
            {(stages ?? []).map((s) => (
              <option key={s.id} value={s.id}>
                {s.name}
              </option>
            ))}
          </Select>
        </div>
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
    </>
  );
}

function CustomFieldsSection({
  lead,
  customFields,
  folders,
  collapsed,
  onToggleFolder,
}: {
  lead: Lead;
  customFields: CustomField[] | undefined;
  folders: { id: number; name: string; position: number }[] | undefined;
  collapsed: Record<string, boolean>;
  onToggleFolder: (folderId: number) => void;
}) {
  const activeFields = (customFields ?? []).filter((f) => f.is_active);
  if (activeFields.length === 0) return null;
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

  return (
    <div>
      <SectionLabel className="mb-2">Custom Fields</SectionLabel>
      <div className="flex flex-col gap-3">
        {grouped.folders
          .filter((g) => g.fields.length > 0)
          .map((g) => {
            const isCollapsed = !!collapsed[g.folder.id];
            return (
              <div key={g.folder.id} className="rounded-md border border-gray-200">
                <button
                  type="button"
                  onClick={() => onToggleFolder(g.folder.id)}
                  className="flex w-full items-center gap-1.5 px-2.5 py-2 text-left text-sm font-medium text-gray-700 hover:bg-gray-50"
                >
                  {isCollapsed ? (
                    <ChevronRight className="h-4 w-4 text-gray-400" />
                  ) : (
                    <ChevronDown className="h-4 w-4 text-gray-400" />
                  )}
                  {g.folder.name}
                </button>
                {!isCollapsed && (
                  <div className="flex flex-col gap-2.5 border-t border-gray-100 px-2.5 py-2.5">
                    {g.fields.map(renderField)}
                  </div>
                )}
              </div>
            );
          })}
        {grouped.unassigned.length > 0 && (
          <div className="flex flex-col gap-2.5">{grouped.unassigned.map(renderField)}</div>
        )}
      </div>
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
      value={val}
      type={type === "number" ? "number" : "text"}
      onChange={(e) => setVal(e.target.value)}
      onBlur={() => save(type === "number" ? Number(val) : val)}
    />
  );
}

function NotesTab({ leadId }: { leadId: number }) {
  const { data: notes } = useNotes(leadId);
  const addNote = useAddNote();
  const [body, setBody] = useState("");
  return (
    <div className="flex flex-col gap-4">
      <div>
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
      <div>
        <SectionLabel className="mb-2">Notes</SectionLabel>
        {(notes ?? []).map((n) => (
          <div key={n.id} className="border-b border-gray-100 py-2 last:border-0">
            <div className="mb-0.5 flex items-center gap-1.5">
              <span className="text-xs font-semibold text-gray-600">{n.author_name || "System"}</span>
              <span className="text-xs text-gray-400">
                {format(new Date(n.created_at), "MMM d, h:mma")}
              </span>
            </div>
            <p className="whitespace-pre-wrap text-sm leading-relaxed text-gray-700">{n.body}</p>
          </div>
        ))}
      </div>
    </div>
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

function kindLabel(kind: LeadHistoryEntry["kind"]): string {
  switch (kind) {
    case "stage_change":
      return "Stage";
    case "account_transfer":
      return "Transfer";
    case "purchase":
      return "Purchase";
    case "refund":
      return "Refund";
    case "dispute_opened":
    case "dispute_resolved":
      return "Dispute";
    case "webhook":
      return "Webhook";
    case "outbound_webhook":
      return "Outbound";
    case "integration":
      return "CRM";
    case "lead_created":
      return "Created";
    case "pipeline_placed":
      return "Placement";
    case "status_change":
      return "Status";
    case "field_change":
      return "Field";
    case "assignee_change":
      return "Assignee";
    case "tag_change":
      return "Tags";
    case "calendar_event":
      return "Calendar";
    case "follower_added":
    case "follower_removed":
      return "Follower";
    case "lead_deleted":
      return "Deleted";
    case "pipeline_cleared":
      return "Pipeline";
    case "imported":
      return "Import";
    case "note_added":
      return "Note";
    case "route_run":
      return "Route";
    default:
      return "Activity";
  }
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

function historyHeadline(entry: LeadHistoryEntry): string {
  if (entry.summary) return entry.summary;
  if (entry.kind === "account_transfer") return transferHeadline(entry);
  if (entry.kind === "stage_change") return stageChangeHeadline(entry);
  if (entry.field_name && entry.from_value != null && entry.to_value != null) {
    return `${entry.field_name} · ${entry.from_value} → ${entry.to_value}`;
  }
  return kindLabel(entry.kind);
}

function HistoryTab({ leadId }: { leadId: number }) {
  const { data: history, isLoading, isError } = useLeadHistory(leadId);
  return (
    <div>
      <SectionLabel className="mb-2">Activity</SectionLabel>
      {isLoading && (
        <div className="flex justify-center py-6">
          <Spinner />
        </div>
      )}
      {isError && (
        <p className="text-sm text-red-500">Could not load activity.</p>
      )}
      {!isLoading && !isError && (history ?? []).length === 0 && (
        <p className="text-sm text-gray-400">No activity yet.</p>
      )}
      {(history ?? []).map((h) => (
        <div key={`${h.kind}-${h.id}`} className="flex items-start gap-2.5 py-1.5 text-sm text-gray-500">
          <span className="mt-1.5 h-2 w-2 shrink-0 rounded-full bg-jade-300" />
          <div className="min-w-0 flex-1">
            <div className="mb-0.5 flex flex-wrap items-center gap-2">
              <span className="rounded bg-gray-100 px-1.5 py-0.5 text-[10px] font-semibold uppercase tracking-wide text-gray-500">
                {kindLabel(h.kind)}
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
              <div className={cn("font-medium", h.kind === "note_added" && "line-clamp-3 whitespace-pre-wrap")}>
                {historyHeadline(h)}
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
        <Badge variant="returned">Returned</Badge>
        <span className="text-xs font-semibold text-warning-fg">Re-distribute this lead</span>
      </div>
      <p className="text-xs text-gray-400">
        Send this returned lead to another buyer from the Contracts page.
      </p>
      {user?.role === "admin" && (
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
