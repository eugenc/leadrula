export const COMPENSATION_KINDS = [
  { value: "flat_rate", label: "Static Rate" },
  { value: "bid", label: "Bid" },
  { value: "rev_share", label: "Rev Share" },
  { value: "profit_share", label: "Profit Share" },
] as const;

export const COMPENSATION_TRIGGERS = [
  { value: "per_lead", label: "Per Lead" },
  { value: "buyer_stage", label: "Buyer stage" },
  { value: "manual", label: "Manual" },
] as const;

export const COMPENSATION_DELIVERY = [
  { value: "leads_pipeline", label: "Pipeline" },
  { value: "leads", label: "Leads inbox" },
] as const;

export type CompensationKind = (typeof COMPENSATION_KINDS)[number]["value"];

export function flatRateAmountLabel(leadType: string): string {
  if (leadType === "Appointment") return "Price per appointment (USD)";
  return "Flat amount (USD)";
}

export function defaultTriggerForKind(kind: CompensationKind): string {
  if (kind === "flat_rate" || kind === "bid") return "per_lead";
  return "buyer_stage";
}

export function formatCompTrigger(trigger: string | undefined): string {
  if (!trigger) return "";
  return COMPENSATION_TRIGGERS.find((t) => t.value === trigger)?.label ?? trigger;
}

export const PAYOUT_FREQUENCIES = [
  { value: "daily", label: "Daily" },
  { value: "weekly", label: "Weekly" },
  { value: "monthly", label: "Monthly" },
] as const;

export const PAYOUT_WEEKDAYS = [
  { value: 0, label: "Sunday" },
  { value: 1, label: "Monday" },
  { value: 2, label: "Tuesday" },
  { value: 3, label: "Wednesday" },
  { value: 4, label: "Thursday" },
  { value: 5, label: "Friday" },
  { value: 6, label: "Saturday" },
] as const;

export const PAYOUT_MONTH_DAYS = Array.from({ length: 28 }, (_, i) => ({
  value: i + 1,
  label: String(i + 1),
}));

export type PayoutFrequency = (typeof PAYOUT_FREQUENCIES)[number]["value"];

export type PipelineDraftFields = {
  source_pipeline_id: number;
  source_stage_id: number;
  counterparty_pipeline_id: number;
  counterparty_stage_id: number;
  return_stage_id: number;
};

export type ContractDeliveryDraft = PipelineDraftFields & { delivery: string };

export function emptyContractDelivery(): ContractDeliveryDraft {
  return {
    delivery: "leads_pipeline",
    source_pipeline_id: 0,
    source_stage_id: 0,
    counterparty_pipeline_id: 0,
    counterparty_stage_id: 0,
    return_stage_id: 0,
  };
}

export function deliveryDraftFromContract(
  contract: {
    source_pipeline_id?: number | null;
    source_stage_id?: number | null;
    buyer_pipeline_id?: number | null;
    return_stage_id?: number | null;
  },
  delivery?: string
): ContractDeliveryDraft {
  const sourcePipeline = contract.source_pipeline_id ?? 0;
  const buyerPipeline = contract.buyer_pipeline_id ?? 0;
  const returnStage = contract.return_stage_id ?? 0;
  const inferred =
    delivery ??
    (sourcePipeline === 0 && buyerPipeline === 0 && returnStage === 0 ? "leads" : "leads_pipeline");
  return {
    delivery: inferred,
    source_pipeline_id: sourcePipeline,
    source_stage_id: contract.source_stage_id ?? 0,
    counterparty_pipeline_id: buyerPipeline,
    counterparty_stage_id: 0,
    return_stage_id: returnStage,
  };
}

export function deliveryDraftToBody(d: ContractDeliveryDraft): Record<string, unknown> {
  const leadsInbox = d.delivery === "leads";
  return {
    delivery: d.delivery,
    source_pipeline_id: leadsInbox ? 0 : d.source_pipeline_id,
    source_stage_id: leadsInbox ? 0 : d.source_stage_id,
    buyer_pipeline_id: leadsInbox ? 0 : d.counterparty_pipeline_id,
    return_stage_id: leadsInbox ? 0 : d.return_stage_id,
  };
}

export function deliveryDraftValid(d: ContractDeliveryDraft): boolean {
  if (d.delivery === "leads") return true;
  return !!d.source_stage_id && !!d.counterparty_pipeline_id && !!d.return_stage_id;
}

export function pipelineDraftWithoutLeads<T extends PipelineDraftFields & { delivery: string }>(draft: T): T {
  if (draft.delivery !== "leads") return draft;
  return {
    ...draft,
    source_pipeline_id: 0,
    source_stage_id: 0,
    counterparty_pipeline_id: 0,
    counterparty_stage_id: 0,
    return_stage_id: 0,
  };
}

export function pipelineFieldsToBody(
  delivery: string,
  d: PipelineDraftFields
): Record<string, number | null> {
  if (delivery === "leads") {
    return {
      source_pipeline_id: null,
      source_stage_id: null,
      counterparty_pipeline_id: null,
      counterparty_stage_id: null,
      return_stage_id: null,
    };
  }
  return {
    source_pipeline_id: d.source_pipeline_id || null,
    source_stage_id: d.source_stage_id || null,
    counterparty_pipeline_id: d.counterparty_pipeline_id || null,
    counterparty_stage_id: d.counterparty_stage_id || null,
    return_stage_id: d.return_stage_id || null,
  };
}

export function payoutFieldsToBody(freq: string, weekday: number, monthDay: number): Record<string, unknown> {
  if (!freq) {
    return { payout_frequency: null, payout_weekday: null, payout_month_day: null };
  }
  return {
    payout_frequency: freq,
    payout_weekday: freq === "weekly" ? weekday : null,
    payout_month_day: freq === "monthly" ? monthDay : null,
  };
}
