export function flattenPayload(payload: Record<string, unknown>): Record<string, unknown> {
  const out: Record<string, unknown> = {};
  for (const [k, v] of Object.entries(payload)) {
    if (k === "custom") continue;
    out[k] = v;
  }
  const custom = payload.custom;
  if (custom && typeof custom === "object" && !Array.isArray(custom)) {
    for (const [k, v] of Object.entries(custom as Record<string, unknown>)) {
      out[k] = v;
    }
  }
  return out;
}

export function payloadValuePreview(payload: Record<string, unknown>, key: string): string {
  const v = flattenPayload(payload)[key];
  if (v == null) return "—";
  if (typeof v === "string") return v;
  if (typeof v === "number" || typeof v === "boolean") return String(v);
  return JSON.stringify(v);
}
