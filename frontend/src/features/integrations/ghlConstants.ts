import type { OutboundFieldMapEntry } from "@/types";

export const GHL_STANDARD_CONTACT_FIELDS: { ghl: string; leadrula: string }[] = [
  { ghl: "firstName", leadrula: "first_name" },
  { ghl: "lastName", leadrula: "last_name" },
  { ghl: "phone", leadrula: "phone" },
  { ghl: "email", leadrula: "email" },
  { ghl: "address1", leadrula: "address" },
  { ghl: "city", leadrula: "city" },
  { ghl: "state", leadrula: "state" },
  { ghl: "postalCode", leadrula: "zip" },
  { ghl: "source", leadrula: "source" },
];

export type GHLFieldSource = {
  source_type: "builtin" | "custom" | "static";
  builtin_field?: string;
  custom_field_id?: number;
  static_value?: string;
};

export type GHLPipelineStageMapEntry = {
  leadrula_pipeline_id: number;
  leadrula_stage_id: number;
  ghl_pipeline_id: string;
  ghl_pipeline_stage_id: string;
};

export type GHLDeliveryMode = "api" | "webhook";

export type GHLMapSection = "contact" | "opportunity" | "appointment";

export type GHLOpportunityStandardFields = {
  monetary_value?: GHLFieldSource;
  assigned_user_id?: GHLFieldSource;
  status?: GHLFieldSource;
};

export type GHLAppointmentStandardFields = {
  description?: GHLFieldSource;
  address?: GHLFieldSource;
  duration_minutes?: number;
  assigned_user_id?: GHLFieldSource;
  meeting_location_type?: GHLFieldSource;
};

export type GHLConfig = {
  delivery_mode?: GHLDeliveryMode;
  webhook_url?: string;
  location_id?: string;
  create_contact?: boolean;
  create_opportunity?: boolean;
  create_appointment?: boolean;
  calendar_id?: string;
  appointment_timezone?: string;
  appointment_datetime?: GHLFieldSource;
  /** @deprecated use appointment_title_template */
  appointment_title?: GHLFieldSource;
  appointment_title_template?: string;
  opportunity_title_template?: string;
  appointment_notes?: GHLFieldSource;
  pipeline_stage_map?: GHLPipelineStageMapEntry[];
  outbound_field_map?: OutboundFieldMapEntry[];
  opportunity_standard_fields?: GHLOpportunityStandardFields;
  appointment_standard_fields?: GHLAppointmentStandardFields;
  inbound_stage_sync_enabled?: boolean;
  inbound_sync_leadrula_pipeline_id?: number;
  inbound_sync_ghl_pipeline_id?: string;
};

export function ghlEntryBelongsToSection(e: OutboundFieldMapEntry, section: GHLMapSection): boolean {
  if (e.ghl_map_section) return e.ghl_map_section === section;
  if (section === "contact") return !e.ghl_field_model || e.ghl_field_model === "contact";
  if (section === "opportunity") return e.ghl_field_model === "opportunity";
  return false;
}

export function ghlMapSectionEntries(all: OutboundFieldMapEntry[], section: GHLMapSection): OutboundFieldMapEntry[] {
  return all.filter((e) => ghlEntryBelongsToSection(e, section));
}

export function mergeGhlMapSection(
  all: OutboundFieldMapEntry[],
  section: GHLMapSection,
  sectionEntries: OutboundFieldMapEntry[]
): OutboundFieldMapEntry[] {
  const kept = all.filter((e) => !ghlEntryBelongsToSection(e, section));
  const tagged = sectionEntries.map((e) => ({
    ...e,
    ghl_map_section: section,
    ...(section === "opportunity" ? { ghl_field_model: "opportunity" as const } : {}),
    ...(section === "contact" && !e.ghl_field_model ? { ghl_field_model: "contact" as const } : {}),
  }));
  return [...kept, ...tagged];
}

export const GHL_MEETING_LOCATION_TYPES = ["custom", "zoom", "gmeet", "phone", "address", "ms_teams", "google"];

export const GHL_OPPORTUNITY_STATUS_VALUES = ["open", "won", "lost", "abandoned"];

export const GHL_TITLE_BUILTINS = [
  "first_name",
  "last_name",
  "phone",
  "email",
  "source",
  "action_at",
  "address",
  "city",
  "state",
  "zip",
];

export const GHL_APPOINTMENT_BUILTINS = ["action_at", "first_name", "last_name", "phone", "email", "source"];

export const DEFAULT_APPOINTMENT_DATETIME: GHLFieldSource = {
  source_type: "builtin",
  builtin_field: "action_at",
};

export function isGhlWebhookMode(config: GHLConfig): boolean {
  return config.delivery_mode === "webhook";
}

export function isGhlFieldSourceSet(fs?: GHLFieldSource): boolean {
  if (!fs) return false;
  if (fs.source_type === "static") return !!fs.static_value?.trim();
  if (fs.source_type === "builtin") return !!fs.builtin_field?.trim();
  if (fs.source_type === "custom") return !!fs.custom_field_id;
  return false;
}

function fieldSourceToTemplate(fs?: GHLFieldSource): string | null {
  if (!fs) return null;
  if (fs.source_type === "static" && fs.static_value) return fs.static_value;
  if (fs.source_type === "builtin" && fs.builtin_field) return `{{${fs.builtin_field}}}`;
  if (fs.source_type === "custom" && fs.custom_field_id) return `{{custom:${fs.custom_field_id}}}`;
  return null;
}

export function appointmentTitleTemplateFromConfig(config: GHLConfig): string {
  if (config.appointment_title_template?.trim()) return config.appointment_title_template;
  return fieldSourceToTemplate(config.appointment_title) ?? "{{first_name}}";
}

export function opportunityTitleTemplateFromConfig(config: GHLConfig): string {
  if (config.opportunity_title_template?.trim()) return config.opportunity_title_template;
  return "{{first_name}} {{last_name}}";
}

export function normalizeGhlConfig(config: GHLConfig): GHLConfig {
  const appointment_datetime =
    config.create_appointment && !isGhlFieldSourceSet(config.appointment_datetime)
      ? DEFAULT_APPOINTMENT_DATETIME
      : config.appointment_datetime;
  return {
    ...config,
    appointment_datetime,
    appointment_title_template: appointmentTitleTemplateFromConfig(config),
    opportunity_title_template: opportunityTitleTemplateFromConfig(config),
  };
}

export const DEFAULT_GHL_CONFIG = (locationId: string): GHLConfig => ({
  delivery_mode: "api",
  location_id: locationId,
  create_contact: true,
  create_opportunity: false,
  create_appointment: false,
  appointment_timezone: "America/New_York",
  appointment_datetime: DEFAULT_APPOINTMENT_DATETIME,
  appointment_title_template: "{{first_name}}",
  opportunity_title_template: "{{first_name}} {{last_name}}",
  pipeline_stage_map: [],
  outbound_field_map: [],
  inbound_stage_sync_enabled: false,
});

export const GHL_TIMEZONES = [
  "America/New_York",
  "America/Chicago",
  "America/Denver",
  "America/Los_Angeles",
  "America/Phoenix",
  "UTC",
];
