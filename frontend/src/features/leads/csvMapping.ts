import { ADD_CUSTOM_FIELD } from "@/features/admin/customFieldConstants";

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
  phoneno: "phone",
  phonenum: "phone",
  mobile: "phone",
  mobilephone: "phone",
  cellphone: "phone",
  tel: "phone",
  telephone: "phone",
  cell: "phone",
  businessphone: "phone",
  workphone: "phone",
  homephone: "phone",
  officephone: "phone",
  directphone: "phone",
  primaryphone: "phone",
  mainphone: "phone",
  contactphone: "phone",
  dayphone: "phone",
  eveningphone: "phone",
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
  campaign: "source",
  campaignname: "source",
  campaign_name: "source",
  source: "source",
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
  { value: "source", label: "Source" },
  { value: "tags", label: "Tags" },
];

export function normalizeHeader(h: string): string {
  return h.trim().toLowerCase().replace(/[^a-z0-9]/g, "");
}

function isPhoneLikeHeader(header: string): boolean {
  const norm = normalizeHeader(header);
  if (ALIASES[norm] === "phone") return true;
  return (
    norm.includes("phone") ||
    norm.includes("mobile") ||
    norm.includes("cellphone") ||
    norm.endsWith("tel") ||
    norm.startsWith("tel")
  );
}

export function guessTarget(header: string, customFields: { id: number; name: string }[]): string {
  const norm = normalizeHeader(header);
  if (ALIASES[norm]) return ALIASES[norm];
  if (isPhoneLikeHeader(header)) return "phone";
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
  if (!Object.values(out).includes("phone")) {
    for (const h of headers) {
      if (isPhoneLikeHeader(h)) {
        out[h] = "phone";
        break;
      }
    }
  }
  return out;
}

export function mappingTargetsWithCustom(customFields: { id: number; name: string; is_active?: boolean }[]) {
  const custom = customFields
    .filter((f) => f.is_active !== false)
    .map((f) => ({ value: `custom_${f.id}`, label: f.name }));
  return [...MAPPING_TARGETS, ...custom, { value: ADD_CUSTOM_FIELD, label: "+ Add custom field…" }];
}
