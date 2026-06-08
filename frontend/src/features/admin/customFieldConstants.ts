export const ADD_CUSTOM_FIELD = "__add_custom_field__";

export const CUSTOM_FIELD_TYPES = [
  { value: "text", label: "Text" },
  { value: "number", label: "Number" },
  { value: "date", label: "Date" },
  { value: "datetime", label: "Date & time" },
  { value: "dropdown", label: "Dropdown" },
  { value: "checkbox", label: "Checkbox" },
] as const;

export const DATE_FORMAT_PRESETS = [
  { value: "yyyy-MM-DD", label: "ISO date (yyyy-MM-DD)" },
  { value: "MM/dd/yyyy", label: "US date (MM/dd/yyyy)" },
  { value: "dd/MM/yyyy", label: "EU date (dd/MM/yyyy)" },
  { value: "MMM d, yyyy", label: "Long date (Jun 8, 2026)" },
] as const;

export const DATETIME_FORMAT_PRESETS = [
  { value: "yyyy-MM-DDTHH:mm", label: "ISO datetime (yyyy-MM-DDTHH:mm)" },
  { value: "yyyy-MM-DD HH:mm", label: "Datetime with space (yyyy-MM-DD HH:mm)" },
  { value: "RFC3339", label: "RFC 3339 (2026-06-08T14:30:00-04:00)" },
  { value: "MM/dd/yyyy h:mm a", label: "US datetime (06/08/2026 2:30 PM)" },
] as const;

export function defaultFormatForType(type: string): string {
  if (type === "datetime") return "yyyy-MM-DDTHH:mm";
  if (type === "date") return "yyyy-MM-DD";
  return "";
}

export function formatPresetsForType(type: string): readonly { value: string; label: string }[] {
  if (type === "datetime") return DATETIME_FORMAT_PRESETS;
  if (type === "date") return DATE_FORMAT_PRESETS;
  return [];
}

export function effectiveFieldFormat(type: string, format?: string | null): string {
  if (format) return format;
  return defaultFormatForType(type);
}

export function slugFieldKey(name: string): string {
  return name
    .trim()
    .toLowerCase()
    .replace(/[^a-z0-9]+/g, "_")
    .replace(/^_|_$/g, "");
}
