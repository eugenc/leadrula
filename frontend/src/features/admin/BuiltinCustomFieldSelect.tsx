import { Label, Select } from "@/components/ui/input";
import { ADD_CUSTOM_FIELD } from "./customFieldConstants";

import { BUILTIN_FIELD_LABELS, MAP_BUILTIN_FIELDS } from "@/features/leads/csvMapping";

const DEFAULT_BUILTINS = MAP_BUILTIN_FIELDS;

export function BuiltinCustomFieldSelect({
  value,
  onChange,
  customFields,
  label,
  builtins = DEFAULT_BUILTINS,
  onAddCustomField,
}: {
  value: string;
  onChange: (v: string) => void;
  customFields: { id: number; name: string; is_active?: boolean }[];
  label: string;
  builtins?: string[];
  onAddCustomField: () => void;
}) {
  return (
    <div>
      <Label>{label}</Label>
      <Select
        value={value}
        onChange={(e) => {
          if (e.target.value === ADD_CUSTOM_FIELD) {
            onAddCustomField();
            return;
          }
          onChange(e.target.value);
        }}
      >
        <optgroup label="Built-in">
          {builtins.map((b) => (
            <option key={b} value={b}>
              {BUILTIN_FIELD_LABELS[b] ?? b}
            </option>
          ))}
        </optgroup>
        <optgroup label="Custom">
          {customFields
            .filter((f) => f.is_active !== false)
            .map((f) => (
              <option key={f.id} value={`cf:${f.id}`}>
                {f.name}
              </option>
            ))}
          <option value={ADD_CUSTOM_FIELD}>+ Add custom field…</option>
        </optgroup>
      </Select>
    </div>
  );
}
