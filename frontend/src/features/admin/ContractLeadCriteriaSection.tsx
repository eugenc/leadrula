import { Button } from "@/components/ui/button";
import { Input, Label, Select } from "@/components/ui/input";
import { SectionLabel } from "@/components/layout/SectionLabel";
import { IconButton } from "@/components/layout/IconButton";
import { BuiltinCustomFieldSelect } from "@/features/admin/BuiltinCustomFieldSelect";
import {
  fieldsSectionCopy,
  filtersSectionCopy,
} from "@/features/admin/contractLeadCriteriaLabels";
import { useCustomFields } from "@/features/leads/hooks";
import { Trash2 } from "lucide-react";
import type { ContractLeadCriteria } from "@/types";

const FILTER_OPS = [
  { value: "eq", label: "equals" },
  { value: "neq", label: "not equals" },
  { value: "contains", label: "contains" },
  { value: "not_empty", label: "not empty" },
  { value: "gt", label: "More than" },
  { value: "lt", label: "Less than" },
];

function parseFieldKey(key: string): { field_type: string; builtin_field?: string; custom_field_id?: number } {
  if (key.startsWith("cf:")) {
    return { field_type: "custom", custom_field_id: Number(key.slice(3)) };
  }
  return { field_type: "builtin", builtin_field: key };
}

function fieldKey(field_type: string, builtin?: string, customId?: number | null): string {
  if (field_type === "custom" && customId) return `cf:${customId}`;
  return builtin ?? "";
}

export function emptyLeadCriteria(): ContractLeadCriteria {
  return { required_fields: [], field_map: [], filter_rules: [], quality_rules: [] };
}

type LeadCriteriaSectionProps = {
  buyerId?: number;
  buyerPipelineId?: number;
  value: ContractLeadCriteria;
  onChange: (v: ContractLeadCriteria) => void;
  contractType?: string;
  leadType?: string;
};

export function ContractLeadFieldsSection({
  value,
  onChange,
  contractType = "sell",
  leadType = "",
}: LeadCriteriaSectionProps) {
  const { data: customFields } = useCustomFields();
  const fields = customFields ?? [];
  const labels = fieldsSectionCopy(leadType, contractType);

  return (
    <div className="flex flex-col gap-4">
      <SectionLabel>{labels.title}</SectionLabel>
      <p className="text-xs text-gray-400">{labels.intro}</p>

      <div>
        {(value.required_fields ?? []).map((r, i) => (
          <div key={i} className="mb-2 flex gap-2">
            <div className="flex-1">
              <BuiltinCustomFieldSelect
                label="Field"
                value={fieldKey(r.field_type, r.builtin_field, r.custom_field_id)}
                onChange={(k) => {
                  const parsed = parseFieldKey(k);
                  const next = [...(value.required_fields ?? [])];
                  next[i] = { ...parsed, field_type: parsed.field_type };
                  onChange({ ...value, required_fields: next });
                }}
                customFields={fields}
                onAddCustomField={() => {}}
              />
            </div>
            <IconButton
              variant="danger"
              className="mt-6 shrink-0"
              aria-label={labels.removeAria}
              onClick={() =>
                onChange({
                  ...value,
                  required_fields: (value.required_fields ?? []).filter((_, j) => j !== i),
                })
              }
            >
              <Trash2 className="h-4 w-4" />
            </IconButton>
          </div>
        ))}
        <Button
          variant="secondary"
          className="text-xs"
          onClick={() =>
            onChange({
              ...value,
              required_fields: [...(value.required_fields ?? []), { field_type: "builtin", builtin_field: "phone" }],
            })
          }
        >
          {labels.addButton}
        </Button>
      </div>
    </div>
  );
}

export function ContractLeadFilterRulesSection({
  value,
  onChange,
}: Pick<LeadCriteriaSectionProps, "value" | "onChange">) {
  const { data: customFields } = useCustomFields();
  const fields = customFields ?? [];
  const labels = filtersSectionCopy();

  return (
    <div className="flex flex-col gap-4">
      <SectionLabel>{labels.title}</SectionLabel>
      <p className="text-xs text-gray-400">{labels.intro}</p>

      <div>
        {(value.filter_rules ?? []).map((r, i) => (
          <div key={i} className="relative mb-2 rounded border border-gray-100 p-2 pr-10">
            <IconButton
              variant="danger"
              className="absolute right-1 top-1"
              aria-label={labels.removeAria}
              onClick={() => onChange({ ...value, filter_rules: (value.filter_rules ?? []).filter((_, j) => j !== i) })}
            >
              <Trash2 className="h-4 w-4" />
            </IconButton>
            <div className="grid grid-cols-3 gap-2">
              <BuiltinCustomFieldSelect
                label="Field"
                value={fieldKey(r.field_type, r.builtin_field, r.custom_field_id)}
                onChange={(k) => {
                  const parsed = parseFieldKey(k);
                  const next = [...(value.filter_rules ?? [])];
                  next[i] = { ...next[i], field_type: parsed.field_type, builtin_field: parsed.builtin_field, custom_field_id: parsed.custom_field_id };
                  onChange({ ...value, filter_rules: next });
                }}
                customFields={fields}
                onAddCustomField={() => {}}
              />
              <div>
                <Label>Operator</Label>
                <Select
                  value={r.operator}
                  onChange={(e) => {
                    const next = [...(value.filter_rules ?? [])];
                    next[i] = { ...next[i], operator: e.target.value };
                    onChange({ ...value, filter_rules: next });
                  }}
                >
                  {FILTER_OPS.map((o) => (
                    <option key={o.value} value={o.value}>
                      {o.label}
                    </option>
                  ))}
                </Select>
              </div>
              <div>
                <Label>Value</Label>
                <Input
                  value={r.value}
                  onChange={(e) => {
                    const next = [...(value.filter_rules ?? [])];
                    next[i] = { ...next[i], value: e.target.value };
                    onChange({ ...value, filter_rules: next });
                  }}
                />
              </div>
            </div>
          </div>
        ))}
        <Button
          variant="secondary"
          className="text-xs"
          onClick={() =>
            onChange({
              ...value,
              filter_rules: [...(value.filter_rules ?? []), { field_type: "builtin", builtin_field: "state", operator: "eq", value: "" }],
            })
          }
        >
          {labels.addButton}
        </Button>
      </div>
    </div>
  );
}

export function ContractLeadCriteriaSaveButton({
  disabled,
  pending,
  onClick,
}: {
  disabled: boolean;
  pending: boolean;
  onClick: () => void;
}) {
  return (
    <Button className="mt-3" variant="secondary" disabled={disabled || pending} onClick={onClick}>
      Save lead criteria
    </Button>
  );
}
