import type { ComponentType } from "react";
import { Label, Select, Input } from "@/components/ui/input";
import type {
  GHLAppointmentStandardFields,
  GHLFieldSource,
  GHLOpportunityStandardFields,
} from "@/features/integrations/ghlConstants";
import { GHL_MEETING_LOCATION_TYPES } from "@/features/integrations/ghlConstants";

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
