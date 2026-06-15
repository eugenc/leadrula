import { Button } from "@/components/ui/button";
import { Input, Label, Select } from "@/components/ui/input";
import { useCustomFields } from "@/features/leads/hooks";
import {
  CONDITION_OPS,
  conditionFields,
  findField,
  type FieldKind,
} from "@/features/pipelines/ruleFieldRegistry";
import { Plus, Trash2 } from "lucide-react";
import type { RuleCondition, RuleConditionOp } from "@/types";

const PAYLOAD_OPS: { value: RuleConditionOp; label: string }[] = [
  { value: "eq", label: "equals" },
  { value: "neq", label: "not equals" },
  { value: "contains", label: "contains" },
  { value: "empty", label: "is empty" },
  { value: "not_empty", label: "is not empty" },
];

function blankCondition(): RuleCondition {
  return { domain: "lead", field: "source", op: "eq", value: "" };
}

function valueInput(
  kind: FieldKind,
  op: RuleConditionOp,
  value: unknown,
  onChange: (v: unknown) => void,
  disabled?: boolean
) {
  if (op === "empty" || op === "not_empty") return null;
  if (kind === "checkbox") {
    return (
      <Select
        value={value ? "true" : "false"}
        disabled={disabled}
        onChange={(e) => onChange(e.target.value === "true")}
      >
        <option value="true">Yes</option>
        <option value="false">No</option>
      </Select>
    );
  }
  return (
    <Input
      value={value == null ? "" : String(value)}
      disabled={disabled}
      onChange={(e) => onChange(e.target.value)}
      placeholder="Value"
    />
  );
}

export function RouteConditionsEditor({
  conditionLogic,
  conditions,
  onConditionLogicChange,
  onConditionsChange,
  disabled,
  showPayloadDomain,
  embedded,
}: {
  conditionLogic: "and" | "or";
  conditions: RuleCondition[];
  onConditionLogicChange: (v: "and" | "or") => void;
  onConditionsChange: (v: RuleCondition[]) => void;
  disabled?: boolean;
  showPayloadDomain?: boolean;
  embedded?: boolean;
}) {
  const { data: customFields } = useCustomFields();
  const leadFields = conditionFields("lead", customFields ?? []);

  return (
    <div className={embedded ? "space-y-3" : "space-y-3 rounded-lg border border-border p-3"}>
      <p className="text-sm text-muted-foreground">
        Leave empty to always match. First matching route wins.
      </p>
      <div>
        <Label>Match</Label>
        <Select
          value={conditionLogic}
          disabled={disabled}
          onChange={(e) => onConditionLogicChange(e.target.value as "and" | "or")}
        >
          <option value="and">ALL conditions</option>
          <option value="or">ANY condition</option>
        </Select>
      </div>
      {conditions.length === 0 ? (
        <p className="text-sm text-muted-foreground">No conditions — always matches.</p>
      ) : (
        <ul className="space-y-2">
          {conditions.map((c, i) => {
            const isPayload = c.domain === "payload";
            const def = isPayload ? null : findField(leadFields, c.field);
            const kind = def?.kind ?? "text";
            const ops = isPayload ? PAYLOAD_OPS : CONDITION_OPS[kind];
            return (
              <li key={i} className="grid grid-cols-[auto_1fr_1fr_1fr_auto] items-end gap-2">
                <div>
                  <Label className="text-xs">Domain</Label>
                  <Select
                    value={c.domain === "payload" ? "payload" : "lead"}
                    disabled={disabled}
                    onChange={(e) => {
                      const next = [...conditions];
                      next[i] = {
                        ...c,
                        domain: e.target.value as RuleCondition["domain"],
                        field: e.target.value === "payload" ? "lead_source" : "source",
                        op: "eq",
                        value: "",
                      };
                      onConditionsChange(next);
                    }}
                  >
                    <option value="lead">Lead</option>
                    {showPayloadDomain && <option value="payload">Payload</option>}
                  </Select>
                </div>
                <div>
                  <Label className="text-xs">Field</Label>
                  {isPayload ? (
                    <Input
                      value={c.field}
                      disabled={disabled}
                      placeholder="e.g. lead_source"
                      onChange={(e) => {
                        const next = [...conditions];
                        next[i] = { ...c, field: e.target.value };
                        onConditionsChange(next);
                      }}
                    />
                  ) : (
                    <Select
                      value={c.field}
                      disabled={disabled}
                      onChange={(e) => {
                        const next = [...conditions];
                        next[i] = { ...c, field: e.target.value, op: "eq", value: "" };
                        onConditionsChange(next);
                      }}
                    >
                      {leadFields.map((f) => (
                        <option key={f.field} value={f.field}>
                          {f.label}
                        </option>
                      ))}
                    </Select>
                  )}
                </div>
                <div>
                  <Label className="text-xs">Operator</Label>
                  <Select
                    value={c.op}
                    disabled={disabled}
                    onChange={(e) => {
                      const next = [...conditions];
                      next[i] = { ...c, op: e.target.value as RuleConditionOp };
                      onConditionsChange(next);
                    }}
                  >
                    {ops.map((o) => (
                      <option key={o.value} value={o.value}>
                        {o.label}
                      </option>
                    ))}
                  </Select>
                </div>
                <div>
                  <Label className="text-xs">Value</Label>
                  {valueInput(kind, c.op, c.value, (v) => {
                    const next = [...conditions];
                    next[i] = { ...c, value: v };
                    onConditionsChange(next);
                  }, disabled)}
                </div>
                <Button
                  type="button"
                  variant="secondary"
                  size="sm"
                  disabled={disabled}
                  onClick={() => onConditionsChange(conditions.filter((_, j) => j !== i))}
                >
                  <Trash2 className="h-3.5 w-3.5" />
                </Button>
              </li>
            );
          })}
        </ul>
      )}
      <Button
        type="button"
        size="sm"
        variant="outline"
        disabled={disabled}
        onClick={() => onConditionsChange([...conditions, blankCondition()])}
      >
        <Plus className="h-3.5 w-3.5" /> Add condition
      </Button>
    </div>
  );
}

export function summarizeBranchConditions(branch: {
  condition_logic?: "and" | "or";
  conditions?: RuleCondition[];
}): string {
  const conds = branch.conditions ?? [];
  if (conds.length === 0) return "Always";
  const joiner = branch.condition_logic === "or" ? " or " : " and ";
  return conds
    .map((c) => {
      const prefix = c.domain === "payload" ? "payload." : "";
      if (c.op === "empty" || c.op === "not_empty") return `${prefix}${c.field} ${c.op}`;
      return `${prefix}${c.field} ${c.op} ${String(c.value ?? "")}`;
    })
    .join(joiner);
}
