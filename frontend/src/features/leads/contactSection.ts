import type { CustomFieldFolder } from "@/types";

export const CONTACT_SYSTEM_KEY = "contact";

export const CONTACT_SYSTEM_FIELDS = [
  { key: "first_name", label: "First Name" },
  { key: "last_name", label: "Last Name" },
  { key: "phone", label: "Phone" },
  { key: "email", label: "Email" },
  { key: "address", label: "Address" },
  { key: "tags", label: "Tags" },
] as const;

export type ContactFieldKey = (typeof CONTACT_SYSTEM_FIELDS)[number]["key"];

export const CONTACT_LOCKED_BUILTIN_KEYS = ["first_name", "last_name"] as const;

export const CONTACT_REORDERABLE_BUILTIN_KEYS = ["phone", "email", "address", "tags"] as const;

export const DEFAULT_CONTACT_BUILTIN_ORDER: ContactFieldKey[] = CONTACT_SYSTEM_FIELDS.map(
  (f) => f.key
);

const CONTACT_FIELD_BY_KEY = Object.fromEntries(
  CONTACT_SYSTEM_FIELDS.map((f) => [f.key, f])
) as Record<ContactFieldKey, (typeof CONTACT_SYSTEM_FIELDS)[number]>;

export function isContactFolder(folder: Pick<CustomFieldFolder, "is_system" | "system_key">): boolean {
  return folder.is_system === true && folder.system_key === CONTACT_SYSTEM_KEY;
}

export function isLockedContactField(key: ContactFieldKey): boolean {
  return (CONTACT_LOCKED_BUILTIN_KEYS as readonly string[]).includes(key);
}

export function resolveContactBuiltinTail(order?: string[] | null): ContactFieldKey[] {
  const reorderable = new Set<string>(CONTACT_REORDERABLE_BUILTIN_KEYS);
  const seen = new Set<string>();
  const tail: ContactFieldKey[] = [];
  for (const key of order ?? []) {
    if (!reorderable.has(key) || seen.has(key)) continue;
    seen.add(key);
    tail.push(key as ContactFieldKey);
  }
  for (const key of CONTACT_REORDERABLE_BUILTIN_KEYS) {
    if (!seen.has(key)) tail.push(key);
  }
  return tail;
}

export function resolveContactBuiltinOrder(order?: string[] | null): ContactFieldKey[] {
  return [...CONTACT_LOCKED_BUILTIN_KEYS, ...resolveContactBuiltinTail(order)];
}

export function isDefaultContactBuiltinOrder(order?: string[] | null): boolean {
  const resolved = resolveContactBuiltinOrder(order);
  return DEFAULT_CONTACT_BUILTIN_ORDER.every((key, i) => resolved[i] === key);
}

export function orderedContactSystemFields(order?: string[] | null) {
  return resolveContactBuiltinOrder(order).map((key) => CONTACT_FIELD_BY_KEY[key]);
}

export function orderedLockedContactSystemFields() {
  return CONTACT_LOCKED_BUILTIN_KEYS.map((key) => CONTACT_FIELD_BY_KEY[key]);
}

export function orderedReorderableContactSystemFields(order?: string[] | null) {
  return resolveContactBuiltinTail(order).map((key) => CONTACT_FIELD_BY_KEY[key]);
}

export function contactBuiltinOrderFromFolder(
  folder: Pick<CustomFieldFolder, "contact_builtin_order"> | undefined
): ContactFieldKey[] {
  return resolveContactBuiltinOrder(folder?.contact_builtin_order);
}
