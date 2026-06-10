export const CONTRACT_TYPES = [
  { value: "sell", label: "Sell" },
  { value: "buy", label: "Buy" },
] as const;

export type ContractType = (typeof CONTRACT_TYPES)[number]["value"];

export function formatContractType(value: string | undefined): string {
  if (!value) return "";
  return CONTRACT_TYPES.find((t) => t.value === value)?.label ?? value;
}

export function isContractType(value: string): value is ContractType {
  return value === "buy" || value === "sell";
}

export function counterpartyLabel(_contractType?: string): string {
  return "Buyer";
}

export function counterpartyPipelineLabel(contractType?: string): string {
  return contractType === "buy" ? "Publisher pipeline" : "Buyer pipeline";
}

export function formatAccountTypeLabel(type: string | undefined): string {
  if (type === "publisher") return "Publisher";
  return "Buyer";
}

export function formatBuyerWithType(name: string | undefined, type: string | undefined): string {
  if (!name) return "";
  return type ? `${name} · ${formatAccountTypeLabel(type)}` : name;
}
