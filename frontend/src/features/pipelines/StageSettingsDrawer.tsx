import { useEffect, useState } from "react";
import { Sheet, DrawerHeader, DrawerBody } from "@/components/ui/dialog";
import { Button } from "@/components/ui/button";
import { Input, Select } from "@/components/ui/input";
import { SectionLabel } from "@/components/layout/SectionLabel";
import { IconButton } from "@/components/layout/IconButton";
import { Spinner, Switch } from "@/components/ui/misc";
import { STAGE_COLORS } from "./stageColors";
import { STAGE_TYPES } from "./stageTypes";
import {
  ACTION_DOMAINS,
  CONDITION_DOMAINS,
  CONDITION_OPS,
  LEAD_STATUSES,
  actionFields,
  conditionFields,
  findField,
  type FieldDef,
  type FieldKind,
} from "./ruleFieldRegistry";
import {
  useCreateStageRule,
  useCreateStageReason,
  useDeleteStageReason,
  useDeleteStageRule,
  usePipelineDisqReasons,
  useStageRules,
  useStageDisqReasons,
  useUpdateStage,
  useUpdateStageReason,
  useUpdateStageRule,
} from "@/features/admin/hooks";
import { useCustomFields, useStages, useUsers } from "@/features/leads/hooks";
import { formatStatus } from "@/features/leads/leadsListColumns";
import { cn } from "@/lib/utils";
import { errorMessage } from "@/lib/api";
import { toast } from "@/store/toastStore";
import { Plus, Trash2 } from "lucide-react";
import type {
  CustomField,
  DisqReason,
  RuleAction,
  RuleCondition,
  RuleConditionOp,
  Stage,
  StageRule,
  StageType,
} from "@/types";

type Lookups = {
  customFields: CustomField[];
  stages: Stage[];
  users: { id: number; full_name: string }[];
  reasons: DisqReason[];
  currentStageId: number;
};

type RuleDraft = {
  condition_logic: "and" | "or";
  conditions: RuleCondition[];
  actions: RuleAction[];
};

// ── value coercion helpers ──────────────────────────────────────────
const asNum = (v: unknown, d = 0) => (typeof v === "number" ? v : Number(v) || d);
const asStr = (v: unknown, d = "") => (typeof v === "string" ? v : v == null ? d : String(v));
const daysOf = (v: unknown) =>
  v && typeof v === "object" && "days" in v ? asNum((v as { days: unknown }).days) : asNum(v);
const modeOf = (v: unknown) =>
  v && typeof v === "object" && "mode" in v ? asStr((v as { mode: unknown }).mode, "today") : "today";

function defaultConditionValue(kind: FieldKind, op: RuleConditionOp, lk: Lookups): unknown {
  if (op === "empty" || op === "not_empty") return undefined;
  switch (kind) {
    case "date":
      return { days: 0 };
    case "number":
      return 0;
    case "status":
      return LEAD_STATUSES[0];
    case "stage":
      return lk.stages[0]?.id ?? 0;
    case "user":
      return lk.users[0]?.id ?? 0;
    case "disq":
      return lk.reasons[0]?.id ?? 0;
    case "checkbox":
      return true;
    default:
      return "";
  }
}

function defaultActionValue(kind: FieldKind, lk: Lookups): unknown {
  switch (kind) {
    case "date":
      return { mode: "today" };
    case "status":
      return LEAD_STATUSES[0];
    case "stage":
      return lk.stages.find((s) => s.id !== lk.currentStageId)?.id ?? lk.stages[0]?.id ?? 0;
    case "user":
      return lk.users[0]?.id ?? null;
    case "disq":
      return lk.reasons[0]?.id ?? null;
    case "checkbox":
      return true;
    case "tags":
      return [];
    default:
      return "";
  }
}

function makeCondition(domain: "lead" | "pipeline", lk: Lookups, field?: string): RuleCondition {
  const fields = conditionFields(domain, lk.customFields);
  const def = (field && findField(fields, field)) || fields[0];
  const op = CONDITION_OPS[def.kind][0].value;
  return { domain, field: def.field, op, value: defaultConditionValue(def.kind, op, lk) };
}

function makeAction(domain: "lead" | "pipeline" | "user", lk: Lookups, field?: string): RuleAction {
  const fields = actionFields(domain, lk.customFields);
  const def = (field && findField(fields, field)) || fields[0];
  return { verb: "update", domain, field: def.field, value: defaultActionValue(def.kind, lk) };
}

function blankCondition(): RuleCondition {
  return { domain: "lead", field: "", op: "eq" };
}

function blankAction(): RuleAction {
  return { verb: "update", domain: "lead", field: "" };
}

const ACTION_VERBS = [{ value: "update", label: "Update" }] as const;
const showVerbPicker = ACTION_VERBS.length > 1;

function isCompleteCondition(c: RuleCondition): boolean {
  return c.field !== "";
}

function isCompleteAction(a: RuleAction): boolean {
  return a.field !== "";
}

function canSaveRule(value: RuleDraft): boolean {
  if (value.actions.length === 0 || !value.actions.every(isCompleteAction)) return false;
  if (!value.conditions.every(isCompleteCondition)) return false;
  return true;
}

function emptyRule(): RuleDraft {
  return { condition_logic: "and", conditions: [], actions: [] };
}

function disqReasonLabel(r: DisqReason): string {
  return r.stage_name ? `${r.stage_name} — ${r.label}` : r.label;
}

function findDisqReasonLabel(lk: Lookups, value: unknown): string {
  const r = lk.reasons.find((x) => x.id === asNum(value));
  return r ? disqReasonLabel(r) : "—";
}

type Props = {
  stage: Stage | null;
  pipelineId: number;
  open: boolean;
  onClose: () => void;
};

export function StageSettingsDrawer({ stage, pipelineId, open, onClose }: Props) {
  const updateStage = useUpdateStage();
  const { data: rules, isLoading: rulesLoading } = useStageRules(open ? stage?.id ?? null : null);
  const { data: stages } = useStages(pipelineId);
  const { data: customFields } = useCustomFields();
  const { data: users } = useUsers();
  const { data: stageReasons, isLoading: stageReasonsLoading } = useStageDisqReasons(
    open && stage?.stage_type === "disqualification" ? stage.id : null
  );
  const { data: pipelineReasons } = usePipelineDisqReasons(open ? pipelineId : null);
  const createReason = useCreateStageReason();
  const updateReason = useUpdateStageReason();
  const removeReason = useDeleteStageReason();
  const createRule = useCreateStageRule();
  const [draft, setDraft] = useState<RuleDraft | null>(null);
  const [newReasonLabel, setNewReasonLabel] = useState("");

  if (!stage) return null;

  const lk: Lookups = {
    customFields: customFields ?? [],
    stages: stages ?? [],
    users: (users ?? []).filter((u) => u.status === "active"),
    reasons: (pipelineReasons ?? []).filter((r) => r.is_active),
    currentStageId: stage.id,
  };

  return (
    <Sheet open={open} onClose={onClose} width={560}>
      <div className="flex h-full flex-col">
        <DrawerHeader title={stage.name} subtitle="Stage settings" onClose={onClose} />

        <DrawerBody>
          <div className="flex flex-col gap-5">
            <section>
              <SectionLabel className="mb-2">Colour</SectionLabel>
              <div className="flex flex-wrap gap-2">
                {STAGE_COLORS.map((c) => (
                  <button
                    key={c.slug}
                    type="button"
                    aria-label={c.slug}
                    onClick={() =>
                      updateStage.mutate(
                        { id: stage.id, body: { color: c.slug } },
                        { onError: (e) => toast.error(errorMessage(e)) }
                      )
                    }
                    className={cn(
                      "h-7 w-7 rounded-full border-2 border-surface-card shadow-sm",
                      c.dot,
                      stage.color === c.slug && `ring-2 ring-offset-2 ${c.ring}`
                    )}
                  />
                ))}
              </div>
            </section>

            <section>
              <SectionLabel className="mb-2">Stage type</SectionLabel>
              <Select
                value={stage.stage_type}
                onChange={(e) => {
                  const stage_type = e.target.value as StageType;
                  if (stage_type !== stage.stage_type) {
                    updateStage.mutate(
                      { id: stage.id, body: { stage_type } },
                      { onError: (err) => toast.error(errorMessage(err)) }
                    );
                  }
                }}
              >
                {STAGE_TYPES.map((t) => (
                  <option key={t.value} value={t.value}>
                    {t.label}
                  </option>
                ))}
              </Select>
              <p className="mt-1.5 text-xs text-gray-500">
                {STAGE_TYPES.find((t) => t.value === stage.stage_type)?.description}
              </p>
            </section>

            {stage.stage_type === "disqualification" && (
              <section>
                <SectionLabel className="mb-2">Disqualification reasons</SectionLabel>
                <p className="mb-3 text-xs text-gray-400">
                  Reasons shown when a lead is moved into this stage.
                </p>
                {stageReasonsLoading ? (
                  <Spinner className="h-5 w-5" />
                ) : (
                  <div className="space-y-2">
                    {(stageReasons ?? []).map((r) => (
                      <div
                        key={r.id}
                        className="flex items-center justify-between gap-3 border-b border-gray-100 pb-2 last:border-0"
                      >
                        <span className="text-sm font-medium text-gray-800">{r.label}</span>
                        <div className="flex items-center gap-3">
                          <Switch
                            checked={r.is_active}
                            onChange={(v) =>
                              updateReason.mutate(
                                { id: r.id, body: { is_active: v } },
                                { onError: (e) => toast.error(errorMessage(e)) }
                              )
                            }
                          />
                          <IconButton
                            variant="danger"
                            onClick={() =>
                              removeReason.mutate(r.id, {
                                onError: (e) => toast.error(errorMessage(e)),
                              })
                            }
                          >
                            <Trash2 className="h-4 w-4" />
                          </IconButton>
                        </div>
                      </div>
                    ))}
                  </div>
                )}
                <div className="mt-3 flex gap-2">
                  <Input
                    value={newReasonLabel}
                    onChange={(e) => setNewReasonLabel(e.target.value)}
                    placeholder="New reason"
                    className="h-8 text-sm"
                  />
                  <Button
                    size="sm"
                    disabled={!newReasonLabel.trim()}
                    onClick={() => {
                      const label = newReasonLabel.trim();
                      if (!label) return;
                      createReason.mutate(
                        { stageId: stage.id, label },
                        {
                          onSuccess: () => setNewReasonLabel(""),
                          onError: (e) => toast.error(errorMessage(e)),
                        }
                      );
                    }}
                  >
                    <Plus className="h-4 w-4" /> Add
                  </Button>
                </div>
              </section>
            )}

            <section>
              <div className="mb-2 flex items-center justify-between">
                <SectionLabel>Rules on entry</SectionLabel>
                <Button
                  size="sm"
                  variant="outline"
                  onClick={() => setDraft(emptyRule())}
                  disabled={!!draft}
                >
                  <Plus className="h-3.5 w-3.5" /> Add rule
                </Button>
              </div>
              <p className="mb-4 text-xs text-gray-400">
                When a lead is moved into this stage, the first matching rule runs its actions.
              </p>

              {rulesLoading ? (
                <Spinner className="h-5 w-5" />
              ) : (
                <div className="flex flex-col gap-3">
                  {(rules ?? []).map((rule) => (
                    <RuleCard key={rule.id} rule={rule} lk={lk} />
                  ))}
                  {draft && (
                    <RuleEditor
                      value={draft}
                      onChange={setDraft}
                      lk={lk}
                      onSave={() =>
                        createRule.mutate(
                          { stageId: stage.id, body: draft },
                          {
                            onSuccess: () => {
                              setDraft(null);
                              toast.success("Rule created");
                            },
                            onError: (e) => toast.error(errorMessage(e)),
                          }
                        )
                      }
                      onCancel={() => setDraft(null)}
                      saving={createRule.isPending}
                    />
                  )}
                  {!rules?.length && !draft && (
                    <p className="text-sm text-gray-400">No rules yet.</p>
                  )}
                </div>
              )}
            </section>
          </div>
        </DrawerBody>
      </div>
    </Sheet>
  );
}

function RuleCard({ rule, lk }: { rule: StageRule; lk: Lookups }) {
  const [editing, setEditing] = useState(false);
  const [value, setValue] = useState<RuleDraft>({
    condition_logic: rule.condition_logic,
    conditions: rule.conditions,
    actions: rule.actions,
  });
  const updateRule = useUpdateStageRule();
  const deleteRule = useDeleteStageRule();

  useEffect(() => {
    setValue({
      condition_logic: rule.condition_logic,
      conditions: rule.conditions,
      actions: rule.actions,
    });
  }, [rule]);

  if (editing) {
    return (
      <RuleEditor
        value={value}
        onChange={setValue}
        lk={lk}
        onSave={() =>
          updateRule.mutate(
            {
              id: rule.id,
              body: {
                condition_logic: value.condition_logic,
                conditions: value.conditions,
                actions: value.actions,
              },
            },
            {
              onSuccess: () => {
                setEditing(false);
                toast.success("Rule saved");
              },
              onError: (e) => toast.error(errorMessage(e)),
            }
          )
        }
        onCancel={() => {
          setValue({
            condition_logic: rule.condition_logic,
            conditions: rule.conditions,
            actions: rule.actions,
          });
          setEditing(false);
        }}
        saving={updateRule.isPending}
      />
    );
  }

  return (
    <div className="rounded-lg border border-gray-100 bg-gray-50 p-3">
      <div className="flex items-start justify-between gap-2">
        <div className="min-w-0 flex-1 text-sm text-gray-700">
          <div className="font-medium text-gray-800">{summarizeRule(rule, lk)}</div>
          <div className="mt-1 text-xs text-gray-400">
            {rule.condition_logic === "or" ? "ANY" : "ALL"} · {rule.actions.length} action
            {rule.actions.length === 1 ? "" : "s"}
          </div>
        </div>
        <div className="flex shrink-0 gap-1">
          <Button size="sm" variant="ghost" onClick={() => setEditing(true)}>
            Edit
          </Button>
          <IconButton
            variant="danger"
            onClick={() =>
              deleteRule.mutate(rule.id, { onError: (e) => toast.error(errorMessage(e)) })
            }
          >
            <Trash2 className="h-3.5 w-3.5" />
          </IconButton>
        </div>
      </div>
    </div>
  );
}

// ── summaries ───────────────────────────────────────────────────────
function summarizeRule(rule: StageRule, lk: Lookups): string {
  const conds = rule.conditions.map((c) => summarizeCondition(c, lk));
  const joiner = rule.condition_logic === "or" ? " or " : " and ";
  const acts = rule.actions.map((a) => summarizeAction(a, lk)).join(", then ");
  const ifPart = conds.length ? conds.join(joiner) : "always";
  return `If ${ifPart} → ${acts || "—"}`;
}

function summarizeCondition(c: RuleCondition, lk: Lookups): string {
  const fields = conditionFields(c.domain, lk.customFields);
  const def = findField(fields, c.field);
  const label = def?.label ?? c.field;
  const opLabel = CONDITION_OPS[def?.kind ?? "text"].find((o) => o.value === c.op)?.label ?? c.op;
  if (c.op === "empty" || c.op === "not_empty") return `${label} ${opLabel}`;
  if (def?.kind === "date") return `${label} ${opLabel} ${daysOf(c.value)} days from today`;
  return `${label} ${opLabel} ${valueLabel(def?.kind ?? "text", c.value, lk)}`;
}

function summarizeAction(a: RuleAction, lk: Lookups): string {
  const fields = actionFields(a.domain, lk.customFields);
  const def = findField(fields, a.field);
  const label = def?.label ?? a.field;
  if (a.domain === "pipeline" && a.field === "stage_id") {
    return `move to ${lk.stages.find((s) => s.id === asNum(a.value))?.name ?? "stage"}`;
  }
  if (def?.kind === "date") {
    const mode = modeOf(a.value);
    if (mode === "clear") return "clear action date";
    if (mode === "plus_days")
      return `set ${label} +${daysOf(a.value)} days`;
    return `set ${label} to today`;
  }
  if ((def?.kind === "user" || def?.kind === "disq") && (a.value == null || a.value === ""))
    return `clear ${label.toLowerCase()}`;
  return `set ${label} → ${valueLabel(def?.kind ?? "text", a.value, lk)}`;
}

function valueLabel(kind: FieldKind, value: unknown, lk: Lookups): string {
  switch (kind) {
    case "status":
      return formatStatus(asStr(value));
    case "stage":
      return lk.stages.find((s) => s.id === asNum(value))?.name ?? "—";
    case "user":
      return lk.users.find((u) => u.id === asNum(value))?.full_name ?? "—";
    case "disq":
      return findDisqReasonLabel(lk, value);
    case "checkbox":
      return value ? "checked" : "unchecked";
    case "tags":
      return Array.isArray(value) ? value.join(", ") : asStr(value);
    case "number":
      return String(asNum(value));
    default:
      return asStr(value) || "—";
  }
}

// ── editor ──────────────────────────────────────────────────────────
function RuleEditor({
  value,
  onChange,
  lk,
  onSave,
  onCancel,
  saving,
}: {
  value: RuleDraft;
  onChange: (v: RuleDraft) => void;
  lk: Lookups;
  onSave: () => void;
  onCancel: () => void;
  saving: boolean;
}) {
  const canSave = canSaveRule(value);

  return (
    <div className="rounded-lg border border-gray-100 bg-gray-50 p-3">
      <div className="mb-4 flex items-center gap-2 text-sm font-medium text-gray-700">
        <span className="shrink-0">Match</span>
        <Select
          value={value.condition_logic}
          onChange={(e) => onChange({ ...value, condition_logic: e.target.value as "and" | "or" })}
          className="w-24 shrink-0 font-medium"
        >
          <option value="and">ALL</option>
          <option value="or">ANY</option>
        </Select>
        <span className="text-sm text-gray-400">of the conditions</span>
      </div>

      <SectionLabel className="mb-2">Conditions</SectionLabel>
      {value.conditions.length === 0 && (
        <p className="mb-2 text-xs text-gray-400">No conditions — rule runs on every entry.</p>
      )}
      <div className="mb-4 flex flex-col gap-2">
        {value.conditions.map((c, i) => (
          <ConditionRow
            key={i}
            cond={c}
            lk={lk}
            onChange={(next) => {
              const arr = [...value.conditions];
              arr[i] = next;
              onChange({ ...value, conditions: arr });
            }}
            onRemove={() => onChange({ ...value, conditions: value.conditions.filter((_, j) => j !== i) })}
          />
        ))}
        <Button
          size="sm"
          variant="outline"
          onClick={() => onChange({ ...value, conditions: [...value.conditions, blankCondition()] })}
        >
          <Plus className="h-3.5 w-3.5" /> Add condition
        </Button>
      </div>

      <SectionLabel className="mb-2">Actions</SectionLabel>
      <div className="mb-4 flex flex-col gap-3">
        {value.actions.map((a, i) => (
          <ActionRow
            key={i}
            action={a}
            lk={lk}
            onChange={(next) => {
              const arr = [...value.actions];
              arr[i] = next;
              onChange({ ...value, actions: arr });
            }}
            onRemove={() => onChange({ ...value, actions: value.actions.filter((_, j) => j !== i) })}
          />
        ))}
        <Button
          size="sm"
          variant="outline"
          onClick={() => onChange({ ...value, actions: [...value.actions, blankAction()] })}
        >
          <Plus className="h-3.5 w-3.5" /> Add action
        </Button>
      </div>

      <div className="flex justify-end gap-2">
        <Button variant="outline" onClick={onCancel}>
          Cancel
        </Button>
        <Button onClick={onSave} disabled={saving || !canSave}>
          Save rule
        </Button>
      </div>
    </div>
  );
}

function ConditionRow({
  cond,
  lk,
  onChange,
  onRemove,
}: {
  cond: RuleCondition;
  lk: Lookups;
  onChange: (c: RuleCondition) => void;
  onRemove: () => void;
}) {
  const incomplete = !isCompleteCondition(cond);

  if (incomplete) {
    return (
      <div className="flex flex-wrap items-center gap-2 rounded-md border border-gray-100 bg-gray-50 p-2">
        <Select
          value=""
          onChange={(e) => {
            const domain = e.target.value as "lead" | "pipeline";
            if (domain) onChange(makeCondition(domain, lk));
          }}
          className="w-36"
        >
          <option value="">Select domain</option>
          {CONDITION_DOMAINS.map((d) => (
            <option key={d.value} value={d.value}>
              {d.label}
            </option>
          ))}
        </Select>
        <IconButton variant="danger" onClick={onRemove}>
          <Trash2 className="h-3.5 w-3.5" />
        </IconButton>
      </div>
    );
  }

  const fields = conditionFields(cond.domain as "lead" | "pipeline", lk.customFields);
  const def = findField(fields, cond.field) ?? fields[0];
  const ops = CONDITION_OPS[def.kind];
  const showValue = cond.op !== "empty" && cond.op !== "not_empty";

  return (
    <div className="flex flex-wrap items-center gap-2 rounded-md border border-gray-100 bg-gray-50 p-2">
      <Select
        value={cond.domain}
        onChange={(e) => onChange(makeCondition(e.target.value as "lead" | "pipeline", lk))}
        className="w-28"
      >
        {CONDITION_DOMAINS.map((d) => (
          <option key={d.value} value={d.value}>
            {d.label}
          </option>
        ))}
      </Select>

      <Select
        value={def.field}
        onChange={(e) => onChange(makeCondition(cond.domain as "lead" | "pipeline", lk, e.target.value))}
        className="min-w-[140px]"
      >
        {fields.map((f) => (
          <option key={f.field} value={f.field}>
            {f.label}
          </option>
        ))}
      </Select>

      <Select
        value={cond.op}
        onChange={(e) => {
          const op = e.target.value as RuleConditionOp;
          const next: RuleCondition = { ...cond, op };
          if (op === "empty" || op === "not_empty") next.value = undefined;
          else if (cond.value === undefined) next.value = defaultConditionValue(def.kind, op, lk);
          onChange(next);
        }}
        className="w-28"
      >
        {ops.map((o) => (
          <option key={o.value} value={o.value}>
            {o.label}
          </option>
        ))}
      </Select>

      {showValue && (
        <ConditionValue
          kind={def.kind}
          def={def}
          value={cond.value}
          lk={lk}
          onChange={(v) => onChange({ ...cond, value: v })}
        />
      )}

      <IconButton variant="danger" onClick={onRemove}>
        <Trash2 className="h-3.5 w-3.5" />
      </IconButton>
    </div>
  );
}

function ConditionValue({
  kind,
  def,
  value,
  lk,
  onChange,
}: {
  kind: FieldKind;
  def: FieldDef;
  value: unknown;
  lk: Lookups;
  onChange: (v: unknown) => void;
}) {
  switch (kind) {
    case "date":
      return (
        <>
          <Input
            type="number"
            min={0}
            value={String(daysOf(value))}
            onChange={(e) => onChange({ days: Number(e.target.value) })}
            className="w-16"
          />
          <span className="text-sm text-gray-600">days from today</span>
        </>
      );
    case "number":
      return (
        <Input
          type="number"
          value={String(asNum(value))}
          onChange={(e) => onChange(Number(e.target.value))}
          className="w-24"
        />
      );
    case "status":
      return (
        <Select value={asStr(value)} onChange={(e) => onChange(e.target.value)} className="min-w-[130px]">
          {LEAD_STATUSES.map((s) => (
            <option key={s} value={s}>
              {formatStatus(s)}
            </option>
          ))}
        </Select>
      );
    case "stage":
      return (
        <Select
          value={String(asNum(value))}
          onChange={(e) => onChange(Number(e.target.value))}
          className="min-w-[130px]"
        >
          {lk.stages.map((s) => (
            <option key={s.id} value={s.id}>
              {s.name}
            </option>
          ))}
        </Select>
      );
    case "user":
      return (
        <Select
          value={String(asNum(value))}
          onChange={(e) => onChange(Number(e.target.value))}
          className="min-w-[130px]"
        >
          {lk.users.map((u) => (
            <option key={u.id} value={u.id}>
              {u.full_name}
            </option>
          ))}
        </Select>
      );
    case "disq":
      return (
        <Select
          value={String(asNum(value))}
          onChange={(e) => onChange(Number(e.target.value))}
          className="min-w-[130px]"
        >
          {lk.reasons.map((r) => (
            <option key={r.id} value={r.id}>
              {disqReasonLabel(r)}
            </option>
          ))}
        </Select>
      );
    case "checkbox":
      return (
        <Select
          value={value ? "true" : "false"}
          onChange={(e) => onChange(e.target.value === "true")}
          className="w-28"
        >
          <option value="true">Checked</option>
          <option value="false">Unchecked</option>
        </Select>
      );
    default:
      // text / dropdown / tags
      if (def.options?.length) {
        return (
          <Select value={asStr(value)} onChange={(e) => onChange(e.target.value)} className="min-w-[130px]">
            {def.options.map((o) => (
              <option key={o} value={o}>
                {o}
              </option>
            ))}
          </Select>
        );
      }
      return (
        <Input
          value={asStr(value)}
          onChange={(e) => onChange(e.target.value)}
          className="min-w-[140px]"
          placeholder="value"
        />
      );
  }
}

function ActionRow({
  action,
  lk,
  onChange,
  onRemove,
}: {
  action: RuleAction;
  lk: Lookups;
  onChange: (a: RuleAction) => void;
  onRemove: () => void;
}) {
  const incomplete = !isCompleteAction(action);
  const fields = incomplete ? [] : actionFields(action.domain, lk.customFields);
  const def = incomplete ? undefined : findField(fields, action.field) ?? fields[0];

  return (
    <div className="relative rounded-md border border-gray-100 bg-surface-card p-2">
      <div className="flex items-center gap-2 pr-9">
        {showVerbPicker && (
          <Select value={action.verb} disabled className="w-24 shrink-0">
            {ACTION_VERBS.map((v) => (
              <option key={v.value} value={v.value}>
                {v.label}
              </option>
            ))}
          </Select>
        )}
        <Select
          value={incomplete ? "" : action.domain}
          onChange={(e) => {
            const domain = e.target.value as "lead" | "pipeline" | "user";
            if (domain) onChange(makeAction(domain, lk));
          }}
          className="min-w-0 flex-1"
        >
          {incomplete && <option value="">Select domain</option>}
          {ACTION_DOMAINS.map((d) => (
            <option key={d.value} value={d.value}>
              {d.label}
            </option>
          ))}
        </Select>
      </div>
      <IconButton variant="danger" className="absolute right-1 top-1" onClick={onRemove}>
        <Trash2 className="h-3.5 w-3.5" />
      </IconButton>

      {!incomplete && def && (
        <div className="mt-2 flex flex-col gap-2">
          <Select
            value={def.field}
            onChange={(e) => onChange(makeAction(action.domain, lk, e.target.value))}
            className="w-full"
          >
            {fields.map((f) => (
              <option key={f.field} value={f.field}>
                {f.label}
              </option>
            ))}
          </Select>
          <ActionValue
            kind={def.kind}
            def={def}
            value={action.value}
            lk={lk}
            fullWidth
            onChange={(v) => onChange({ ...action, value: v })}
          />
        </div>
      )}
    </div>
  );
}

function ActionValue({
  kind,
  def,
  value,
  lk,
  onChange,
  fullWidth = false,
}: {
  kind: FieldKind;
  def: FieldDef;
  value: unknown;
  lk: Lookups;
  onChange: (v: unknown) => void;
  fullWidth?: boolean;
}) {
  const fieldClass = fullWidth ? "w-full" : "min-w-[130px]";

  switch (kind) {
    case "date": {
      const mode = modeOf(value);
      return (
        <div className={fullWidth ? "flex flex-col gap-2" : "contents"}>
          <Select
            value={mode}
            onChange={(e) => {
              const m = e.target.value;
              onChange(m === "plus_days" ? { mode: m, days: daysOf(value) } : { mode: m });
            }}
            className={fullWidth ? "w-full" : "w-36"}
          >
            <option value="today">Today</option>
            <option value="plus_days">Days from today</option>
            <option value="clear">Clear</option>
          </Select>
          {mode === "plus_days" && (
            <Input
              type="number"
              min={0}
              value={String(daysOf(value))}
              onChange={(e) => onChange({ mode: "plus_days", days: Number(e.target.value) })}
              className={fullWidth ? "w-full" : "w-16"}
            />
          )}
        </div>
      );
    }
    case "status":
      return (
        <Select value={asStr(value)} onChange={(e) => onChange(e.target.value)} className={fieldClass}>
          {LEAD_STATUSES.map((s) => (
            <option key={s} value={s}>
              {formatStatus(s)}
            </option>
          ))}
        </Select>
      );
    case "stage":
      return (
        <Select
          value={String(asNum(value))}
          onChange={(e) => onChange(Number(e.target.value))}
          className={fieldClass}
        >
          {lk.stages.map((s) => (
            <option key={s.id} value={s.id}>
              {s.name}
            </option>
          ))}
        </Select>
      );
    case "user":
      return (
        <Select
          value={value == null ? "" : String(asNum(value))}
          onChange={(e) => onChange(e.target.value ? Number(e.target.value) : null)}
          className={fieldClass}
        >
          <option value="">Unassigned</option>
          {lk.users.map((u) => (
            <option key={u.id} value={u.id}>
              {u.full_name}
            </option>
          ))}
        </Select>
      );
    case "disq":
      return (
        <Select
          value={value == null ? "" : String(asNum(value))}
          onChange={(e) => onChange(e.target.value ? Number(e.target.value) : null)}
          className={fieldClass}
        >
          <option value="">Clear</option>
          {lk.reasons.map((r) => (
            <option key={r.id} value={r.id}>
              {disqReasonLabel(r)}
            </option>
          ))}
        </Select>
      );
    case "checkbox":
      return (
        <Select
          value={value ? "true" : "false"}
          onChange={(e) => onChange(e.target.value === "true")}
          className={fullWidth ? "w-full" : "w-28"}
        >
          <option value="true">Checked</option>
          <option value="false">Unchecked</option>
        </Select>
      );
    case "number":
      return (
        <Input
          type="number"
          value={String(asNum(value))}
          onChange={(e) => onChange(Number(e.target.value))}
          className={fullWidth ? "w-full" : "w-24"}
        />
      );
    case "tags":
      return (
        <Input
          value={Array.isArray(value) ? value.join(", ") : asStr(value)}
          onChange={(e) =>
            onChange(
              e.target.value
                .split(",")
                .map((t) => t.trim())
                .filter(Boolean)
            )
          }
          className={fullWidth ? "w-full" : "min-w-[160px]"}
          placeholder="tag1, tag2"
        />
      );
    default:
      if (def.options?.length) {
        return (
          <Select value={asStr(value)} onChange={(e) => onChange(e.target.value)} className={fieldClass}>
            {def.options.map((o) => (
              <option key={o} value={o}>
                {o}
              </option>
            ))}
          </Select>
        );
      }
      return (
        <Input
          value={asStr(value)}
          onChange={(e) => onChange(e.target.value)}
          className={fullWidth ? "w-full" : "min-w-[140px]"}
          placeholder="value"
        />
      );
  }
}
