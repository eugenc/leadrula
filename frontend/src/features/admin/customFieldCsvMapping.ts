const ALIASES: Record<string, string> = {
  name: "name",
  fieldname: "name",
  displayname: "name",
  field_key: "field_key",
  fieldkey: "field_key",
  key: "field_key",
  type: "type",
  fieldtype: "type",
  field_type: "type",
  options: "options",
  choices: "options",
  format: "format",
  dateformat: "format",
  date_format: "format",
  is_active: "is_active",
  isactive: "is_active",
  active: "is_active",
};

export const FIELD_MAPPING_TARGETS: { value: string; label: string }[] = [
  { value: "skip", label: "Skip" },
  { value: "name", label: "Name" },
  { value: "field_key", label: "Field Key" },
  { value: "type", label: "Type" },
  { value: "options", label: "Options" },
  { value: "format", label: "Format" },
  { value: "is_active", label: "Active" },
];

export function normalizeHeader(h: string): string {
  return h.toLowerCase().replace(/[^a-z0-9]/g, "");
}

export function guessFieldTarget(header: string): string {
  const norm = normalizeHeader(header);
  return ALIASES[norm] ?? "skip";
}

export function buildFieldInitialMapping(headers: string[]): Record<string, string> {
  const out: Record<string, string> = {};
  for (const h of headers) {
    out[h] = guessFieldTarget(h);
  }
  return out;
}

export const REQUIRED_FIELD_TARGETS = ["name", "field_key"] as const;

export function missingRequiredMappings(mapping: Record<string, string>): string[] {
  const mapped = new Set(Object.values(mapping).filter((v) => v && v !== "skip"));
  return REQUIRED_FIELD_TARGETS.filter((t) => !mapped.has(t));
}
