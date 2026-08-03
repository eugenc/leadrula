import type { ContractFieldMapEntry, ContractFieldMapOptions } from "@/types";

export function entrySourceKey(e: ContractFieldMapEntry): string | null {
  if (e.src_type === "custom" && e.src_custom_field_id != null) {
    return `cf:${e.src_custom_field_id}`;
  }
  if (e.src_type === "builtin" && e.src_builtin) {
    return e.src_builtin;
  }
  return null;
}

export function mappedSourceKeys(entries: ContractFieldMapEntry[] | undefined): Set<string> {
  const keys = new Set<string>();
  for (const e of entries ?? []) {
    const k = entrySourceKey(e);
    if (k) keys.add(k);
  }
  return keys;
}

export function entryMatchesAvailable(
  e: ContractFieldMapEntry,
  af: ContractFieldMapOptions["available_fields"][number]
): boolean {
  const k = entrySourceKey(e);
  return k != null && k === af.key;
}

export function fieldMappingComplete(
  options: ContractFieldMapOptions | undefined,
  entries: ContractFieldMapEntry[] | undefined
): boolean {
  const available = options?.available_fields ?? [];
  if (available.length === 0) return true;
  const mapped = mappedSourceKeys(entries);
  return available.every((af) => mapped.has(af.key));
}
