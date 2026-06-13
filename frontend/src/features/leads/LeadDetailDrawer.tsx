import { useEffect, useState } from "react";
import { Sheet, DrawerHeader, DrawerBody } from "@/components/ui/dialog";
import { Button } from "@/components/ui/button";
import { Input, InputWithOverflowTooltip, Label, Textarea, Select } from "@/components/ui/input";
import { Avatar, Badge, Spinner } from "@/components/ui/misc";
import { SectionLabel } from "@/components/layout/SectionLabel";
import { ActionDot } from "./ActionDot";
import { format, isPast } from "date-fns";
import { CircleHelp } from "lucide-react";
import { cn, formatMoney } from "@/lib/utils";
import { useUIStore } from "@/store/uiStore";
import { useAuthStore } from "@/store/authStore";
import { toast } from "@/store/toastStore";
import { errorMessage, apiError } from "@/lib/api";
import {
  useLead,
  useNotes,
  useAddNote,
  useStageHistory,
  useUpdateLead,
  useSetActionAt,
  useUsers,
  useCustomFields,
  useDeleteLead,
  useChangeStage,
  usePipelines,
  useStages,
} from "./hooks";
import { DeleteLeadConfirmDialog } from "./DeleteLeadConfirmDialog";
import { StagePromptModal, type PromptResult } from "./StagePromptModal";
import { stageNeedsPrompt } from "@/features/pipelines/stageTypes";
import type { Lead, Stage } from "@/types";
import { formatStatus, leadSourceLabel } from "./leadsListColumns";
import { LeadTagsEditor } from "./LeadTagsEditor";
import { effectiveFieldFormat } from "@/features/admin/customFieldConstants";
import {
  fromNativeDatetimeLocal,
  inputModeForFormat,
  normalizeCustomDateValue,
  toNativeDateValue,
  toNativeDatetimeLocalValue,
} from "./customFieldDate";

const BUILTINS: { key: keyof Lead; label: string }[] = [
  { key: "first_name", label: "First Name" },
  { key: "last_name", label: "Last Name" },
  { key: "phone", label: "Phone" },
  { key: "email", label: "Email" },
  { key: "address", label: "Address" },
  { key: "city", label: "City" },
  { key: "state", label: "State" },
  { key: "zip", label: "Zip" },
];

const DRAWER_TABS = [
  { id: "details", label: "Details" },
  { id: "notes", label: "Notes" },
  { id: "history", label: "History" },
  { id: "profit", label: "Profit" },
] as const;

type DrawerTab = (typeof DRAWER_TABS)[number]["id"];

function isoToDatetimeLocal(iso: string): string {
  const d = new Date(iso);
  const p = (n: number) => String(n).padStart(2, "0");
  return `${d.getFullYear()}-${p(d.getMonth() + 1)}-${p(d.getDate())}T${p(d.getHours())}:${p(d.getMinutes())}`;
}

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
  const { data: lead, isLoading } = useLead(leadId);

  return (
    <Sheet open={!!leadId} onClose={close}>
      {isLoading || !lead ? (
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

  const [fields, setFields] = useState<Record<string, string>>({});
  const [actionAtLocal, setActionAtLocal] = useState("");
  useEffect(() => {
    const f: Record<string, string> = {};
    for (const b of BUILTINS) f[b.key as string] = (lead[b.key] as string) ?? "";
    setFields(f);
    setActionAtLocal(lead.action_at ? isoToDatetimeLocal(lead.action_at) : "");
  }, [lead]);

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
            Action{overdue && " — overdue"}
          </span>
          <Input
            type="datetime-local"
            value={actionAtLocal}
            onChange={(e) => setActionAtLocal(e.target.value)}
            onBlur={saveActionAt}
            disabled={setAction.isPending}
            className="h-7 min-w-0 flex-1 border-0 bg-transparent px-1 py-0 text-xs shadow-none focus:ring-0"
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
                <div>
                  <Label>Buyer</Label>
                  <div className="mt-1 text-sm text-gray-700">{lead.buyer_name ?? "—"}</div>
                </div>
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
              </div>
            </div>
            {(customFields ?? []).filter((f) => f.is_active).length > 0 && (
              <div>
                <SectionLabel className="mb-2">Custom Fields</SectionLabel>
                <div className="flex flex-col gap-2.5">
                  {(customFields ?? [])
                    .filter((f) => f.is_active)
                    .map((f) => (
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
                </div>
              </div>
            )}
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
  const [prompt, setPrompt] = useState<{ stage: Stage } | null>(null);

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
        onSuccess: () => toast.success("Saved"),
        onError: (err) => {
          const e = apiError(err);
          if (e.code === "business_rule" && stageNeedsPrompt(stage.stage_type)) {
            setPrompt({ stage });
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
      setPrompt({ stage });
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
        open={!!prompt}
        stage={prompt?.stage ?? null}
        onCancel={() => {
          setPrompt(null);
          revertFromLead();
        }}
        onConfirm={(r) => {
          if (prompt) commitStage(prompt.stage, r);
          setPrompt(null);
        }}
      />
    </>
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

function HistoryTab({ leadId }: { leadId: number }) {
  const { data: history } = useStageHistory(leadId);
  return (
    <div>
      <SectionLabel className="mb-2">Stage History</SectionLabel>
      {(history ?? []).length === 0 && (
        <p className="text-sm text-gray-400">No stage changes yet.</p>
      )}
      {(history ?? []).map((h) => (
        <div key={h.id} className="flex items-start gap-2.5 py-1.5 text-sm text-gray-500">
          <span className="mt-1.5 h-2 w-2 shrink-0 rounded-full bg-jade-300" />
          <div>
            <div>
              {h.from_stage_name ?? "Created"} → <span className="font-medium">{h.to_stage_name}</span>
            </div>
            <div className="text-xs text-gray-400">
              {h.moved_by_name ?? "System"} · {format(new Date(h.created_at), "MMM d, h:mma")}
              {h.action_at_captured &&
                ` · action ${format(new Date(h.action_at_captured), "MMM d, h:mm a")}`}
              {h.disqualification_reason && ` · ${h.disqualification_reason}`}
            </div>
          </div>
        </div>
      ))}
    </div>
  );
}

function RedistributeBox({ lead }: { lead: Lead }) {
  const user = useAuthStore((s) => s.user);
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
    </div>
  );
}
