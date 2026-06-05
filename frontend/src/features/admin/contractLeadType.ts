export const CONTRACT_LEAD_TYPES = [
  { value: "Data", label: "Data" },
  { value: "Appointment", label: "Appointment" },
  { value: "Call", label: "Call" },
] as const;

export type ContractLeadType = (typeof CONTRACT_LEAD_TYPES)[number]["value"];

const VALUES = new Set<string>(CONTRACT_LEAD_TYPES.map((t) => t.value));

export function isContractLeadType(value: string): value is ContractLeadType {
  return VALUES.has(value);
}

export function formatContractLeadType(value: string | undefined): string {
  if (!value) return "";
  return value;
}
