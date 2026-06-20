import type { CustomField, RuleConditionOp } from "@/types";

// Mirrors the backend field kinds in backend/internal/pipelines/rulefields.go.
export type FieldKind =
  | "text"
  | "number"
  | "date"
  | "status"
  | "stage"
  | "user"
  | "disq"
  | "checkbox"
  | "tags";

export interface FieldDef {
  field: string;
  label: string;
  kind: FieldKind;
  options?: string[];
}

export const LEAD_STATUSES = ["review", "distributed", "returned", "closed"] as const;

export const CONDITION_DOMAINS = [
  { value: "lead", label: "Lead" },
  { value: "pipeline", label: "Pipeline" },
] as const;

export const ACTION_DOMAINS = [
  { value: "lead", label: "Lead" },
  { value: "pipeline", label: "Pipeline" },
  { value: "user", label: "User" },
] as const;

const enumOps: { value: RuleConditionOp; label: string }[] = [
  { value: "eq", label: "equals" },
  { value: "neq", label: "not equals" },
];

const refOps: { value: RuleConditionOp; label: string }[] = [
  ...enumOps,
  { value: "empty", label: "is empty" },
  { value: "not_empty", label: "is not empty" },
];

export const CONDITION_OPS: Record<FieldKind, { value: RuleConditionOp; label: string }[]> = {
  text: [
    { value: "eq", label: "equals" },
    { value: "neq", label: "not equals" },
    { value: "contains", label: "contains" },
    { value: "empty", label: "is empty" },
    { value: "not_empty", label: "is not empty" },
  ],
  number: [
    { value: "eq", label: "=" },
    { value: "gt", label: ">" },
    { value: "lt", label: "<" },
  ],
  date: [
    { value: "lt", label: "before" },
    { value: "gt", label: "after" },
    { value: "eq", label: "on" },
    { value: "empty", label: "is empty" },
    { value: "not_empty", label: "is not empty" },
  ],
  status: enumOps,
  stage: refOps,
  user: refOps,
  disq: refOps,
  checkbox: [{ value: "eq", label: "equals" }],
  tags: [{ value: "contains", label: "contains" }],
};

const LEAD_TEXT_BUILTINS: FieldDef[] = [
  { field: "first_name", label: "First name", kind: "text" },
  { field: "last_name", label: "Last name", kind: "text" },
  { field: "phone", label: "Phone", kind: "text" },
  { field: "email", label: "Email", kind: "text" },
  { field: "address", label: "Address", kind: "text" },
  { field: "city", label: "City", kind: "text" },
  { field: "state", label: "State", kind: "text" },
  { field: "zip", label: "Zip", kind: "text" },
  { field: "source", label: "Source", kind: "text" },
];

function customFieldKind(t: CustomField["type"]): FieldKind {
  if (t === "number") return "number";
  if (t === "date" || t === "datetime") return "date";
  if (t === "checkbox") return "checkbox";
  return "text"; // text, dropdown
}

function customFieldDefs(customFields: CustomField[]): FieldDef[] {
  return customFields
    .filter((f) => f.is_active)
    .map((f) => ({
      field: `custom:${f.field_key}`,
      label: f.name,
      kind: customFieldKind(f.type),
      options: f.type === "dropdown" ? f.options : undefined,
    }));
}

const LEAD_CORE_FIELDS: FieldDef[] = [
  { field: "status", label: "Status", kind: "status" },
  { field: "action_at", label: "Action Date & Time", kind: "date" },
  { field: "assigned_user_id", label: "Assignee", kind: "user" },
  { field: "disqualification_reason_id", label: "Disqualification reason", kind: "disq" },
  { field: "tags", label: "Tags", kind: "tags" },
];

const PIPELINE_CONDITION_FIELDS: FieldDef[] = [
  { field: "previous_stage_id", label: "Previous stage", kind: "stage" },
  { field: "days_in_previous_stage", label: "Days in previous stage", kind: "number" },
];

export function conditionFields(domain: string, customFields: CustomField[]): FieldDef[] {
  if (domain === "pipeline") return PIPELINE_CONDITION_FIELDS;
  return [...LEAD_CORE_FIELDS, ...LEAD_TEXT_BUILTINS, ...customFieldDefs(customFields)];
}

export function actionFields(domain: string, customFields: CustomField[]): FieldDef[] {
  switch (domain) {
    case "pipeline":
      return [{ field: "stage_id", label: "Stage", kind: "stage" }];
    case "user":
      return [{ field: "assigned_user_id", label: "Assignee", kind: "user" }];
    default:
      return [
        { field: "status", label: "Status", kind: "status" },
        { field: "action_at", label: "Action Date & Time", kind: "date" },
        { field: "disqualification_reason_id", label: "Disqualification reason", kind: "disq" },
        { field: "tags", label: "Tags", kind: "tags" },
        ...LEAD_TEXT_BUILTINS,
        ...customFieldDefs(customFields),
      ];
  }
}

export function findField(fields: FieldDef[], field: string): FieldDef | undefined {
  return fields.find((f) => f.field === field);
}

/** Lead + pipeline fields that can be copied from when setting an action value. */
export function sourceFieldsForKind(
  kind: FieldKind,
  customFields: CustomField[],
  excludeField?: string
): FieldDef[] {
  const lead = conditionFields("lead", customFields);
  const pipeline = conditionFields("pipeline", customFields);
  return [...lead, ...pipeline].filter(
    (f) => f.kind === kind && f.field !== excludeField
  );
}
