const ALIASES: Record<string, string> = {
  firstname: "first_name",
  first_name: "first_name",
  fname: "first_name",
  givenname: "first_name",
  lastname: "last_name",
  last_name: "last_name",
  lname: "last_name",
  surname: "last_name",
  phone: "phone",
  phonenumber: "phone",
  phone_number: "phone",
  mobile: "phone",
  tel: "phone",
  telephone: "phone",
  cell: "phone",
  email: "email",
  emailaddress: "email",
  email_address: "email",
  address: "address",
  street: "address",
  streetaddress: "address",
  city: "city",
  state: "state",
  province: "state",
  zip: "zip",
  zipcode: "zip",
  postal: "zip",
  postalcode: "zip",
  campaign: "campaign_name",
  campaignname: "campaign_name",
  campaign_name: "campaign_name",
  tags: "tags",
  tag: "tags",
};

export const MAPPING_TARGETS: { value: string; label: string }[] = [
  { value: "skip", label: "Skip" },
  { value: "first_name", label: "First Name" },
  { value: "last_name", label: "Last Name" },
  { value: "phone", label: "Phone" },
  { value: "email", label: "Email" },
  { value: "address", label: "Address" },
  { value: "city", label: "City" },
  { value: "state", label: "State" },
  { value: "zip", label: "Zip" },
  { value: "campaign_name", label: "Campaign" },
  { value: "tags", label: "Tags" },
];

export function normalizeHeader(h: string): string {
  return h.toLowerCase().replace(/[^a-z0-9]/g, "");
}

export function guessTarget(header: string, customFields: { id: number; name: string }[]): string {
  const norm = normalizeHeader(header);
  if (ALIASES[norm]) return ALIASES[norm];
  for (const f of customFields) {
    if (normalizeHeader(f.name) === norm) return `custom_${f.id}`;
  }
  return "skip";
}

export function buildInitialMapping(
  headers: string[],
  customFields: { id: number; name: string }[]
): Record<string, string> {
  const out: Record<string, string> = {};
  for (const h of headers) {
    out[h] = guessTarget(h, customFields);
  }
  return out;
}

export function mappingTargetsWithCustom(customFields: { id: number; name: string; is_active?: boolean }[]) {
  const custom = customFields
    .filter((f) => f.is_active !== false)
    .map((f) => ({ value: `custom_${f.id}`, label: f.name }));
  return [...MAPPING_TARGETS, ...custom];
}
