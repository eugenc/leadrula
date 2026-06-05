export const COMPENSATION_KINDS = [
  { value: "flat_rate", label: "Flat Rate" },
  { value: "bid", label: "Bid" },
  { value: "rev_share", label: "Rev Share" },
  { value: "profit_share", label: "Profit Share" },
] as const;

export const COMPENSATION_TRIGGERS = [
  { value: "per_lead", label: "Per lead" },
  { value: "buyer_stage", label: "Buyer stage" },
  { value: "manual", label: "Manual" },
] as const;

export const COMPENSATION_DELIVERY = [
  { value: "leads_pipeline", label: "Pipeline" },
  { value: "leads", label: "Leads inbox" },
] as const;

export type CompensationKind = (typeof COMPENSATION_KINDS)[number]["value"];

export function defaultTriggerForKind(kind: CompensationKind): string {
  if (kind === "flat_rate" || kind === "bid") return "per_lead";
  return "buyer_stage";
}
