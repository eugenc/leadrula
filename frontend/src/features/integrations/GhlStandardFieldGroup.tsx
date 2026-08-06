import type { ComponentType } from "react";
import { Label, Select, Input } from "@/components/ui/input";
import { Table, THead, TH, TBody, TR, TD } from "@/components/ui/table";
import {
  GHL_FIELD_MAP_BUILTINS,
  GHL_REQUIRED_CONTACT_FIELDS,
  GHL_STANDARD_CONTACT_FIELDS,
  DEFAULT_GHL_CONTACT_STANDARD_FIELDS,
  mergeGhlContactStandardFields,
  type GHLAppointmentStandardFields,
  type GHLContactStandardFields,
  type GHLFieldSource,
  type GHLOpportunityStandardFields,
} from "@/features/integrations/ghlConstants";
import { GHL_MEETING_LOCATION_TYPES } from "@/features/integrations/ghlConstants";
import { builtinFieldLabel } from "@/features/leads/csvMapping";

type FieldSourceSelectProps = {
  label: string;
  value: string;
  builtins: string[];
  customFields: { id: number; name: string }[];
  onChange: (v: string) => void;
  optional?: boolean;
};

function fieldSourceToSelectValue(fs?: GHLFieldSource): string {
  if (!fs) return "";
  if (fs.source_type === "static") return `static:${fs.static_value ?? ""}`;
  if (fs.source_type === "custom" && fs.custom_field_id) return `cf:${fs.custom_field_id}`;
  if (fs.source_type === "builtin" && fs.builtin_field) return `builtin:${fs.builtin_field}`;
  return "";
}

function selectValueToFieldSource(v: string): GHLFieldSource | undefined {
  if (!v) return undefined;
  if (v.startsWith("static:")) {
    return { source_type: "static", static_value: v.slice(7) };
  }
  if (v.startsWith("cf:")) {
    return { source_type: "custom", custom_field_id: Number(v.slice(3)) };
  }
  const builtin_field = v.startsWith("builtin:") ? v.slice(8) : v;
  return { source_type: "builtin", builtin_field };
}

function applyContactSourceTypeChange(
  row: GHLFieldSource | undefined,
  source_type: GHLFieldSource["source_type"]
): GHLFieldSource {
  const base = { source_type };
  if (source_type === "builtin") {
    return { ...base, builtin_field: row?.builtin_field ?? "last_name" };
  }
  if (source_type === "custom") {
    return row?.custom_field_id ? { ...base, custom_field_id: row.custom_field_id } : base;
  }
  return { ...base, static_value: row?.static_value ?? "" };
}

export function GhlContactStandardFieldsSection({
  values,
  configured,
  onChange,
  customFields,
}: {
  values?: GHLContactStandardFields;
  configured?: boolean;
  onChange: (v: GHLContactStandardFields) => void;
  customFields: { id: number; name: string }[];
}) {
  const merged = mergeGhlContactStandardFields(values, configured);

  function patch(key: keyof GHLContactStandardFields, fs: GHLFieldSource | undefined) {
    const next = { ...merged };
    if (!fs?.source_type) {
      delete next[key];
    } else {
      next[key] = fs;
    }
    onChange(next);
  }

  function displaySource(row: GHLFieldSource | undefined, required: boolean): string {
    if (row?.source_type) return row.source_type;
    return required ? "builtin" : "";
  }

  return (
    <div className="space-y-2">
      <Label>Standard contact fields</Label>
      <Table>
        <THead>
          <tr>
            <TH>GHL field</TH>
            <TH>Source</TH>
            <TH>Value</TH>
          </tr>
        </THead>
        <TBody>
          {GHL_STANDARD_CONTACT_FIELDS.map(({ ghl }) => {
            const required = GHL_REQUIRED_CONTACT_FIELDS.includes(ghl);
            const row = merged[ghl];
            const sourceType = displaySource(row, required);
            return (
              <TR key={ghl}>
                <TD className="font-mono text-xs">{ghl}</TD>
                <TD>
                  <Select
                    className="!h-8 !text-sm"
                    value={sourceType}
                    onChange={(ev) => {
                      const v = ev.target.value;
                      if (!v) {
                        patch(ghl, undefined);
                        return;
                      }
                      const source_type = v as GHLFieldSource["source_type"];
                      patch(ghl, applyContactSourceTypeChange(row, source_type));
                    }}
                  >
                    {!required && <option value="">—</option>}
                    <option value="builtin">Lead field</option>
                    <option value="custom">Custom field</option>
                    <option value="static">Static value</option>
                  </Select>
                </TD>
                <TD>
                  {sourceType === "builtin" && (
                    <Select
                      className="!h-8 w-full !text-sm"
                      value={row?.builtin_field ?? DEFAULT_GHL_CONTACT_STANDARD_FIELDS[ghl]?.builtin_field ?? "last_name"}
                      onChange={(ev) =>
                        patch(ghl, {
                          source_type: "builtin",
                          builtin_field: ev.target.value,
                        })
                      }
                      disabled={!sourceType && !required}
                    >
                      {GHL_FIELD_MAP_BUILTINS.map((b) => (
                        <option key={b} value={b}>
                          {builtinFieldLabel(b)}
                        </option>
                      ))}
                    </Select>
                  )}
                  {sourceType === "custom" && (
                    <Select
                      className="!h-8 w-full !text-sm"
                      value={String(row?.custom_field_id ?? "")}
                      onChange={(ev) => {
                        const v = ev.target.value;
                        if (!v) {
                          patch(ghl, { source_type: "custom" });
                          return;
                        }
                        patch(ghl, { source_type: "custom", custom_field_id: Number(v) });
                      }}
                    >
                      <option value="">Select field</option>
                      {customFields.map((f) => (
                        <option key={f.id} value={f.id}>
                          {f.name}
                        </option>
                      ))}
                    </Select>
                  )}
                  {sourceType === "static" && (
                    <Input
                      value={row?.static_value ?? ""}
                      onChange={(ev) =>
                        patch(ghl, { source_type: "static", static_value: ev.target.value })
                      }
                      className="!h-8 !text-sm"
                    />
                  )}
                </TD>
              </TR>
            );
          })}
        </TBody>
      </Table>
    </div>
  );
}

export function GhlOpportunityStandardFieldsSection({
  values,
  onChange,
  builtins,
  customFields,
  FieldSourceSelect,
}: {
  values?: GHLOpportunityStandardFields;
  onChange: (v: GHLOpportunityStandardFields) => void;
  builtins: string[];
  customFields: { id: number; name: string }[];
  FieldSourceSelect: ComponentType<FieldSourceSelectProps>;
}) {
  function patch(key: keyof GHLOpportunityStandardFields, fs: GHLFieldSource | undefined) {
    onChange({ ...values, [key]: fs });
  }

  return (
    <div className="space-y-3 rounded-lg border border-gray-50 bg-gray-50/50 p-3">
      <Label>Opportunity standard fields</Label>
      <FieldSourceSelect
        label="Monetary value"
        optional
        value={fieldSourceToSelectValue(values?.monetary_value)}
        builtins={builtins}
        customFields={customFields}
        onChange={(v) => patch("monetary_value", selectValueToFieldSource(v))}
      />
      <FieldSourceSelect
        label="Assigned user (GHL user ID)"
        optional
        value={fieldSourceToSelectValue(values?.assigned_user_id)}
        builtins={builtins}
        customFields={customFields}
        onChange={(v) => patch("assigned_user_id", selectValueToFieldSource(v))}
      />
      <FieldSourceSelect
        label="Status"
        optional
        value={fieldSourceToSelectValue(values?.status)}
        builtins={builtins}
        customFields={customFields}
        onChange={(v) => patch("status", selectValueToFieldSource(v))}
      />
      <p className="text-xs text-gray-400">Defaults to open when status is not mapped.</p>
    </div>
  );
}

export function GhlAppointmentStandardFieldsSection({
  values,
  onChange,
  builtins,
  customFields,
  FieldSourceSelect,
}: {
  values?: GHLAppointmentStandardFields;
  onChange: (v: GHLAppointmentStandardFields) => void;
  builtins: string[];
  customFields: { id: number; name: string }[];
  FieldSourceSelect: ComponentType<FieldSourceSelectProps>;
}) {
  function patch(key: keyof GHLAppointmentStandardFields, val: GHLFieldSource | number | undefined) {
    onChange({ ...values, [key]: val });
  }

  return (
    <div className="space-y-3 rounded-lg border border-gray-50 bg-gray-50/50 p-3">
      <Label>Appointment standard fields</Label>
      <FieldSourceSelect
        label="Description"
        optional
        value={fieldSourceToSelectValue(values?.description)}
        builtins={builtins}
        customFields={customFields}
        onChange={(v) => patch("description", selectValueToFieldSource(v))}
      />
      <FieldSourceSelect
        label="Address"
        optional
        value={fieldSourceToSelectValue(values?.address)}
        builtins={builtins}
        customFields={customFields}
        onChange={(v) => patch("address", selectValueToFieldSource(v))}
      />
      <div>
        <Label>Duration (minutes)</Label>
        <Input
          type="number"
          min={1}
          value={values?.duration_minutes ?? 30}
          onChange={(e) => patch("duration_minutes", Number(e.target.value) || 30)}
          className="!h-8 !text-sm"
        />
      </div>
      <FieldSourceSelect
        label="Assigned user override (GHL user ID)"
        optional
        value={fieldSourceToSelectValue(values?.assigned_user_id)}
        builtins={builtins}
        customFields={customFields}
        onChange={(v) => patch("assigned_user_id", selectValueToFieldSource(v))}
      />
      <div>
        <Label>Meeting location type</Label>
        <Select
          value={
            values?.meeting_location_type?.source_type === "static"
              ? (values.meeting_location_type.static_value ?? "")
              : fieldSourceToSelectValue(values?.meeting_location_type)
          }
          onChange={(e) => {
            const v = e.target.value;
            if (!v) {
              patch("meeting_location_type", undefined);
              return;
            }
            if (GHL_MEETING_LOCATION_TYPES.includes(v)) {
              patch("meeting_location_type", { source_type: "static", static_value: v });
              return;
            }
          }}
        >
          <option value="">— or map from lead field below</option>
          {GHL_MEETING_LOCATION_TYPES.map((t) => (
            <option key={t} value={t}>
              {t}
            </option>
          ))}
        </Select>
        <div className="mt-2">
          <FieldSourceSelect
            label="Meeting location type (from lead field)"
            optional
            value={
              values?.meeting_location_type?.source_type !== "static"
                ? fieldSourceToSelectValue(values?.meeting_location_type)
                : ""
            }
            builtins={builtins}
            customFields={customFields}
            onChange={(v) => patch("meeting_location_type", selectValueToFieldSource(v))}
          />
        </div>
      </div>
    </div>
  );
}
