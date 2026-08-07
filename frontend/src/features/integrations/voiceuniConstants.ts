import type { OutboundFieldMapEntry } from "@/types";

export const DEFAULT_VOICEUNI_FIELD_MAP: OutboundFieldMapEntry[] = [
  { dest_key: "external_id", source_type: "builtin", builtin_field: "external_id" },
  { dest_key: "first_name", source_type: "builtin", builtin_field: "first_name" },
  { dest_key: "last_name", source_type: "builtin", builtin_field: "last_name" },
  { dest_key: "phone", source_type: "builtin", builtin_field: "phone" },
  { dest_key: "email", source_type: "builtin", builtin_field: "email" },
  { dest_key: "source", source_type: "builtin", builtin_field: "source" },
];

export function voiceuniFieldMapFromConfig(config: Record<string, unknown> | undefined): OutboundFieldMapEntry[] {
  const raw = config?.outbound_field_map;
  if (Array.isArray(raw) && raw.length > 0) {
    return raw as OutboundFieldMapEntry[];
  }
  return DEFAULT_VOICEUNI_FIELD_MAP;
}
