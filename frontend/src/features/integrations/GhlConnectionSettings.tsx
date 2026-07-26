import { useEffect, useState } from "react";
import { Label, Select, Input } from "@/components/ui/input";
import { Button } from "@/components/ui/button";
import { SectionLabel } from "@/components/layout/SectionLabel";
import { Table, THead, TH, TBody, TR, TD } from "@/components/ui/table";
import { useCustomFields } from "@/features/leads/hooks";
import { CreateCustomFieldDrawer } from "@/features/admin/CreateCustomFieldDrawer";
import { useCreateField } from "@/features/admin/hooks";
import { ADD_CUSTOM_FIELD } from "@/features/admin/customFieldConstants";
import { BUILTIN_FIELD_LABELS } from "@/features/leads/csvMapping";
import {
  GHL_STANDARD_CONTACT_FIELDS,
  GHL_APPOINTMENT_BUILTINS,
  GHL_TIMEZONES,
  DEFAULT_APPOINTMENT_DATETIME,
  isGhlFieldSourceSet,
  isGhlWebhookMode,
  appointmentTitleTemplateFromConfig,
  opportunityTitleTemplateFromConfig,
  type GHLConfig,
  type GHLFieldSource,
} from "@/features/integrations/ghlConstants";
import { GhlCustomFieldMapSection } from "@/features/integrations/GhlCustomFieldMapSection";
import { GhlPipelineStageMapSection } from "@/features/integrations/GhlPipelineStageMapSection";
import { GhlTitleTemplateEditor } from "@/features/integrations/GhlTitleTemplateEditor";
import type { OutboundFieldMapEntry } from "@/types";
import { toast } from "@/store/toastStore";
import { errorMessage } from "@/lib/api";

type GhlPipeline = { id: string; name: string; stages?: { id: string; name: string }[] };
type GhlCalendar = { id: string; name: string };

function fieldSourceToSelectValue(fs?: GHLFieldSource): string {
  if (!fs) return "";
  if (fs.source_type === "static") return `static:${fs.static_value ?? ""}`;
  if (fs.source_type === "custom" && fs.custom_field_id) return `cf:${fs.custom_field_id}`;
  if (fs.source_type === "builtin" && fs.builtin_field) return `builtin:${fs.builtin_field}`;
  return "";
}

function selectValueToFieldSource(v: string): GHLFieldSource {
  if (v.startsWith("static:")) {
    const static_value = v.slice(7);
    return { source_type: "static", static_value };
  }
  if (v.startsWith("cf:")) {
    return { source_type: "custom", custom_field_id: Number(v.slice(3)) };
  }
  const builtin_field = v.startsWith("builtin:") ? v.slice(8) : v;
  return { source_type: "builtin", builtin_field };
}

export function GhlConnectionSettings({
  config,
  onChange,
  ghlPipelines,
  ghlCalendars,
  ghlPipelinesLoading = false,
  ghlCalendarsLoading = false,
  apiAuth,
}: {
  config: GHLConfig;
  onChange: (config: GHLConfig) => void;
  ghlPipelines: GhlPipeline[];
  ghlCalendars: GhlCalendar[];
  ghlPipelinesLoading?: boolean;
  ghlCalendarsLoading?: boolean;
  apiAuth?: {
    pitToken: string;
    onPitTokenChange: (v: string) => void;
    locationId: string;
    onLocationIdChange: (v: string) => void;
    pitPlaceholder?: string;
    hasPrivateIntegrationToken?: boolean;
  };
}) {
  const { data: customFields } = useCustomFields();
  const createField = useCreateField();
  const [createFieldOpen, setCreateFieldOpen] = useState(false);
  const [createFor, setCreateFor] = useState<"datetime" | "notes" | null>(null);

  const activeCustomFields = (customFields ?? []).filter((f) => f.is_active !== false);

  function patch(p: Partial<GHLConfig>) {
    onChange({ ...config, ...p, create_contact: true });
  }

  function patchFieldSource(key: "appointment_datetime" | "appointment_notes", v: string) {
    patch({ [key]: selectValueToFieldSource(v) });
  }

  function onFieldCreated(field: import("@/types").CustomField) {
    if (createFor === "datetime") patch({ appointment_datetime: { source_type: "custom", custom_field_id: field.id } });
    if (createFor === "notes") patch({ appointment_notes: { source_type: "custom", custom_field_id: field.id } });
    setCreateFieldOpen(false);
    setCreateFor(null);
    return field;
  }

  const outboundMap = (config.outbound_field_map ?? []) as OutboundFieldMapEntry[];
  const webhookMode = isGhlWebhookMode(config);

  useEffect(() => {
    if (!webhookMode && config.create_appointment && !isGhlFieldSourceSet(config.appointment_datetime)) {
      patch({ appointment_datetime: DEFAULT_APPOINTMENT_DATETIME });
    }
  }, [webhookMode, config.create_appointment, config.appointment_datetime]);

  return (
    <div className="space-y-4">
      <div>
        <Label>Delivery method</Label>
        <Select
          value={config.delivery_mode ?? "api"}
          onChange={(e) => {
            const mode = e.target.value as "api" | "webhook";
            patch({
              delivery_mode: mode,
              ...(mode === "webhook"
                ? {
                    create_opportunity: false,
                    create_appointment: false,
                  }
                : {}),
            });
          }}
        >
          <option value="api">API (Private Integration Token)</option>
          <option value="webhook">Webhook (GHL automation URL)</option>
        </Select>
      </div>

      {webhookMode && (
        <div>
          <Label>GHL automation webhook URL</Label>
          <Input
            value={config.webhook_url ?? ""}
            onChange={(e) => patch({ webhook_url: e.target.value })}
            placeholder="https://services.leadconnectorhq.com/hooks/..."
          />
          <p className="mt-1 text-xs text-gray-400">
            Paste the inbound webhook URL from your GHL Workflow → Webhook action. Leadrula POSTs mapped
            lead fields and pipeline/stage metadata on distribute.
          </p>
        </div>
      )}

      {!webhookMode && apiAuth && (
        <>
          <div>
            <Label>Private Integration Token</Label>
            <Input
              type="password"
              value={apiAuth.pitToken}
              onChange={(e) => apiAuth.onPitTokenChange(e.target.value)}
              placeholder={apiAuth.pitPlaceholder}
            />
            {apiAuth.hasPrivateIntegrationToken === true && (
              <p className="mt-1 text-xs text-green-700">
                Token saved on server. Enter a new value only to replace it.
              </p>
            )}
            {apiAuth.hasPrivateIntegrationToken === false && (
              <p className="mt-1 text-xs text-amber-700">
                No token saved. Enter your Private Integration Token to enable push.
              </p>
            )}
          </div>
          <div>
            <Label>Location ID</Label>
            <Input
              value={apiAuth.locationId}
              onChange={(e) => apiAuth.onLocationIdChange(e.target.value)}
            />
          </div>
        </>
      )}

      {!webhookMode && (
        <>
      <SectionLabel>On push, create in GHL</SectionLabel>
      <div className="space-y-2">
        <label className="flex items-center gap-2 text-sm">
          <input type="checkbox" checked disabled className="rounded" />
          Contact (required)
        </label>
        <label className="flex items-center gap-2 text-sm">
          <input
            type="checkbox"
            checked={!!config.create_opportunity}
            onChange={(e) => patch({ create_opportunity: e.target.checked })}
            className="rounded"
          />
          Opportunity
        </label>
        <label className="flex items-center gap-2 text-sm">
          <input
            type="checkbox"
            checked={!!config.create_appointment}
            onChange={(e) => {
              const on = e.target.checked;
              patch({
                create_appointment: on,
                ...(on && !isGhlFieldSourceSet(config.appointment_datetime)
                  ? { appointment_datetime: DEFAULT_APPOINTMENT_DATETIME }
                  : {}),
              });
            }}
            className="rounded"
          />
          Appointment
        </label>
      </div>
        </>
      )}

      {webhookMode && (
        <p className="text-xs text-gray-500">
          Contact fields are sent as JSON to your GHL automation webhook when a lead enters a configured
          trigger stage below. Configure your GHL workflow to create contacts and opportunities from the
          payload.
        </p>
      )}

      <div className="rounded-lg border border-gray-100 p-3">
        <Label>Standard contact fields (fixed)</Label>
        <Table className="mt-2">
          <THead>
            <tr>
              <TH>GHL field</TH>
              <TH>Leadrula field</TH>
            </tr>
          </THead>
          <TBody>
            {GHL_STANDARD_CONTACT_FIELDS.map((row) => (
              <TR key={row.ghl}>
                <TD className="font-mono text-xs">{row.ghl}</TD>
                <TD className="text-xs text-gray-600">{row.leadrula}</TD>
              </TR>
            ))}
          </TBody>
        </Table>
      </div>

      <GhlCustomFieldMapSection
        entries={outboundMap}
        onChange={(entries) => patch({ outbound_field_map: entries })}
      />

      {webhookMode && (
        <GhlPipelineStageMapSection
          entries={config.pipeline_stage_map ?? []}
          onChange={(entries) => patch({ pipeline_stage_map: entries })}
          ghlPipelines={[]}
          triggerOnly
        />
      )}

      {!webhookMode && config.create_opportunity && (
        <div className="space-y-3 rounded-lg border border-gray-100 p-3">
          <GhlTitleTemplateEditor
            label="Opportunity title"
            value={opportunityTitleTemplateFromConfig(config)}
            onChange={(v) => patch({ opportunity_title_template: v })}
          />
          <GhlPipelineStageMapSection
            entries={config.pipeline_stage_map ?? []}
            onChange={(entries) => patch({ pipeline_stage_map: entries })}
            ghlPipelines={ghlPipelines}
            ghlPipelinesLoading={ghlPipelinesLoading}
          />
        </div>
      )}

      {!webhookMode && config.create_appointment && (
        <div className="space-y-3 rounded-lg border border-gray-100 p-3">
          <SectionLabel>Appointment settings</SectionLabel>
          <div>
            <Label>GHL calendar</Label>
            <Select
              value={config.calendar_id ?? ""}
              onChange={(e) => patch({ calendar_id: e.target.value })}
            >
              <option value="">Select…</option>
              {ghlCalendars.map((c) => (
                <option key={c.id} value={c.id}>
                  {c.name}
                </option>
              ))}
            </Select>
            {!ghlCalendarsLoading && ghlCalendars.length === 0 && (
              <p className="mt-1 text-xs text-gray-400">
                Click Test connection to load calendars from GHL.
              </p>
            )}
          </div>
          <FieldSourceSelect
            label="Appointment date/time"
            value={fieldSourceToSelectValue(config.appointment_datetime)}
            builtins={GHL_APPOINTMENT_BUILTINS}
            customFields={activeCustomFields}
            onChange={(v) => {
              if (v === ADD_CUSTOM_FIELD) {
                setCreateFor("datetime");
                setCreateFieldOpen(true);
                return;
              }
              patchFieldSource("appointment_datetime", v);
            }}
          />
          <GhlTitleTemplateEditor
            label="Appointment title"
            value={appointmentTitleTemplateFromConfig(config)}
            onChange={(v) => patch({ appointment_title_template: v })}
          />
          <FieldSourceSelect
            label="Appointment notes (optional)"
            value={fieldSourceToSelectValue(config.appointment_notes)}
            builtins={GHL_APPOINTMENT_BUILTINS}
            customFields={activeCustomFields}
            optional
            onChange={(v) => {
              if (v === ADD_CUSTOM_FIELD) {
                setCreateFor("notes");
                setCreateFieldOpen(true);
                return;
              }
              if (v === "") {
                patch({ appointment_notes: undefined });
                return;
              }
              patchFieldSource("appointment_notes", v);
            }}
          />
          <div>
            <Label>Timezone</Label>
            <Select
              value={config.appointment_timezone ?? "America/New_York"}
              onChange={(e) => patch({ appointment_timezone: e.target.value })}
            >
              {GHL_TIMEZONES.map((tz) => (
                <option key={tz} value={tz}>
                  {tz}
                </option>
              ))}
            </Select>
          </div>
        </div>
      )}

      <CreateCustomFieldDrawer
        open={createFieldOpen}
        onClose={() => {
          setCreateFieldOpen(false);
          setCreateFor(null);
        }}
        isPending={createField.isPending}
        onSubmit={(body) =>
          createField.mutateAsync(body).then(onFieldCreated).catch((err) => {
            toast.error(errorMessage(err));
            throw err;
          })
        }
      />
    </div>
  );
}

function FieldSourceSelect({
  label,
  value,
  builtins,
  customFields,
  onChange,
  optional,
  allowStatic,
}: {
  label: string;
  value: string;
  builtins: string[];
  customFields: { id: number; name: string }[];
  onChange: (v: string) => void;
  optional?: boolean;
  allowStatic?: boolean;
}) {
  const [staticMode, setStaticMode] = useState(value.startsWith("static:"));
  const staticVal = value.startsWith("static:") ? value.slice(7) : "";

  if (staticMode && allowStatic) {
    return (
      <div>
        <Label>{label}</Label>
        <Input
          value={staticVal}
          onChange={(e) => onChange(`static:${e.target.value}`)}
          placeholder="Static title"
        />
        <button type="button" className="mt-1 text-xs text-indigo-600" onClick={() => setStaticMode(false)}>
          Use lead field instead
        </button>
      </div>
    );
  }

  return (
    <div>
      <Label>{label}</Label>
      <Select value={value} onChange={(e) => onChange(e.target.value)}>
        {optional && <option value="">—</option>}
        <optgroup label="Built-in">
          {builtins.map((b) => (
            <option key={b} value={`builtin:${b}`}>
              {BUILTIN_FIELD_LABELS[b] ?? b}
            </option>
          ))}
        </optgroup>
        <optgroup label="Custom">
          {customFields.map((f) => (
            <option key={f.id} value={`cf:${f.id}`}>
              {f.name}
            </option>
          ))}
          <option value={ADD_CUSTOM_FIELD}>+ Add custom field…</option>
        </optgroup>
      </Select>
      {allowStatic && (
        <button type="button" className="mt-1 text-xs text-indigo-600" onClick={() => setStaticMode(true)}>
          Use static text instead
        </button>
      )}
    </div>
  );
}
