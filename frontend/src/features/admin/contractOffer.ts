export const PUBLISHER_DELIVERY_MODES = [
  { value: "leads", label: "Leads Only" },
  { value: "leads_pipeline", label: "Pipeline" },
  { value: "webhook", label: "Webhook" },
] as const;

export const DISTRIBUTION_STRATEGIES = [
  { value: "round_robin", label: "Round robin" },
  { value: "highest_price", label: "Highest price" },
  { value: "largest_spread", label: "Largest spread" },
] as const;

export const REQUIRED_OFFER_DELIVERY_MODE = "leads";

export type ContractOfferDraft = {
  allowed_delivery_modes: string[];
  distribution_strategy: string;
};

export function normalizeContractOffer(offer: ContractOfferDraft): ContractOfferDraft {
  const modes = new Set(offer.allowed_delivery_modes);
  modes.add(REQUIRED_OFFER_DELIVERY_MODE);
  return { ...offer, allowed_delivery_modes: [...modes] };
}

export function emptyContractOffer(): ContractOfferDraft {
  return normalizeContractOffer({
    allowed_delivery_modes: [REQUIRED_OFFER_DELIVERY_MODE],
    distribution_strategy: "round_robin",
  });
}

export function offerFromContractModes(
  modes: string[] | undefined,
  strategy: string | undefined
): ContractOfferDraft {
  return normalizeContractOffer({
    allowed_delivery_modes: modes ?? emptyContractOffer().allowed_delivery_modes,
    distribution_strategy: strategy ?? "round_robin",
  });
}

export function offerDraftValid(offer: ContractOfferDraft): boolean {
  return offer.allowed_delivery_modes.includes(REQUIRED_OFFER_DELIVERY_MODE);
}

export function formatParticipationStatus(status: string): string {
  switch (status) {
    case "pending":
      return "Pending acceptance";
    case "active":
      return "Active";
    case "declined":
      return "Declined";
    case "counter_pending":
      return "Counter-offer";
    case "superseded":
      return "Superseded";
    case "paused":
      return "Paused";
    case "withdrawn":
      return "Withdrawn";
    default:
      return status;
  }
}
