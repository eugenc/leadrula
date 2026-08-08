import type { Lead } from "@/types";

const BUILTIN_KEYS = ["first_name", "last_name", "phone", "email"] as const;
const ADDRESS_KEYS = ["address", "city", "state", "zip", "country"] as const;

export function contactFieldsFromLead(lead: Lead): Record<string, string> {
  const f: Record<string, string> = {};
  for (const key of BUILTIN_KEYS) {
    f[key] = (lead[key] as string) ?? "";
  }
  for (const key of ADDRESS_KEYS) {
    f[key] = lead[key] ?? "";
  }
  f.address_place_id = lead.address_place_id ?? "";
  return f;
}

function leadFieldValue(lead: Lead, key: string): string {
  if (key === "address_place_id") return lead.address_place_id ?? "";
  return (lead[key as keyof Lead] as string) ?? "";
}

export function dirtyContactPatch(
  fields: Record<string, string>,
  lead: Lead
): { fields: Record<string, unknown> } | null {
  const patch: Record<string, unknown> = {};

  for (const key of BUILTIN_KEYS) {
    const current = fields[key] ?? "";
    if (current !== leadFieldValue(lead, key)) {
      patch[key] = current;
    }
  }

  let addressDirty = false;
  for (const key of ADDRESS_KEYS) {
    const current = fields[key] ?? "";
    if (current !== leadFieldValue(lead, key)) {
      patch[key] = current;
      addressDirty = true;
    }
  }

  const placeId = fields.address_place_id ?? "";
  const leadPlaceId = lead.address_place_id ?? "";
  if (placeId !== leadPlaceId && !addressDirty) {
    patch.address_place_id = placeId || null;
  }
  if (addressDirty) {
    patch.address_place_id = null;
  }

  return Object.keys(patch).length > 0 ? { fields: patch } : null;
}
