import { compensationsValid, type CompensationDraft } from "@/features/admin/CreateContractCompensationList";
import {
  deliveryDraftValid,
  openOfferPipelineRequired,
  publisherPipelineDraftValid,
  type ContractDeliveryDraft,
} from "@/features/admin/contractCompensation";
import { isContractLeadType } from "@/features/admin/contractLeadType";
import { isContractType } from "@/features/admin/contractType";
import { offerDraftValid, type ContractOfferDraft } from "@/features/admin/contractOffer";
import type { ContractLeadCriteria } from "@/types";

export type ContractTabId = "details" | "compensation" | "delivery" | "criteria" | "returns" | "buyers" | "fieldmap" | "triggers";

export function isOpenSellOffer(form: { contract_type: string; buyer_id: number }): boolean {
  return form.contract_type === "sell" && !form.buyer_id;
}

export function detailsBaseComplete(form: {
  contract_type: string;
  name: string;
  lead_type: string;
}): boolean {
  return !!form.name.trim() && isContractType(form.contract_type) && isContractLeadType(form.lead_type);
}

export function detailsSectionComplete(
  form: {
    contract_type: string;
    buyer_id: number;
    name: string;
    lead_type: string;
  },
  offer?: ContractOfferDraft
): boolean {
  const base = detailsBaseComplete(form);
  if (isOpenSellOffer(form)) {
    return base && offerDraftValid(offer ?? { allowed_delivery_modes: [], distribution_strategy: "round_robin" });
  }
  return base && !!form.buyer_id;
}

export function compensationSectionComplete(items: CompensationDraft[]): boolean {
  return compensationsValid(items);
}

export function deliverySectionComplete(delivery: ContractDeliveryDraft): boolean {
  return deliveryDraftValid(delivery);
}

export function leadCriteriaSectionComplete(criteria: ContractLeadCriteria): boolean {
  const fields = criteria.required_fields ?? [];
  let hasName = false;
  let hasPhoneOrEmail = false;
  for (const r of fields) {
    if (r.field_type !== "builtin") continue;
    if (r.builtin_field === "first_name") hasName = true;
    if (r.builtin_field === "phone" || r.builtin_field === "email") hasPhoneOrEmail = true;
  }
  return hasName && hasPhoneOrEmail;
}

export function returnRulesSectionComplete(): boolean {
  return true;
}

export function openOfferDeliveryComplete(offer: ContractOfferDraft, delivery: ContractDeliveryDraft): boolean {
  if (!offerDraftValid(offer)) return false;
  if (!openOfferPipelineRequired(offer.allowed_delivery_modes)) return true;
  return publisherPipelineDraftValid(delivery);
}

export function allRequiredSectionsComplete(args: {
  form: { contract_type: string; buyer_id: number; name: string; lead_type: string };
  compensations: CompensationDraft[];
  delivery: ContractDeliveryDraft;
  leadCriteria: ContractLeadCriteria;
  offer?: ContractOfferDraft;
}): boolean {
  if (isOpenSellOffer(args.form)) {
    return (
      detailsSectionComplete(args.form, args.offer) &&
      openOfferDeliveryComplete(args.offer ?? { allowed_delivery_modes: [], distribution_strategy: "round_robin" }, args.delivery)
    );
  }
  return (
    detailsSectionComplete(args.form) &&
    compensationSectionComplete(args.compensations) &&
    deliverySectionComplete(args.delivery) &&
    leadCriteriaSectionComplete(args.leadCriteria)
  );
}

export function sectionComplete(
  tab: ContractTabId,
  args: {
    form: { contract_type: string; buyer_id: number; name: string; lead_type: string };
    compensations: CompensationDraft[];
    delivery: ContractDeliveryDraft;
    leadCriteria: ContractLeadCriteria;
    offer?: ContractOfferDraft;
  }
): boolean {
  const openOffer = isOpenSellOffer(args.form);
  switch (tab) {
    case "details":
      if (openOffer) return detailsBaseComplete(args.form);
      return detailsSectionComplete(args.form);
    case "compensation":
      return compensationSectionComplete(args.compensations);
    case "delivery":
      if (openOffer) {
        return openOfferDeliveryComplete(
          args.offer ?? { allowed_delivery_modes: [], distribution_strategy: "round_robin" },
          args.delivery
        );
      }
      return deliverySectionComplete(args.delivery);
    case "criteria":
      return leadCriteriaSectionComplete(args.leadCriteria);
    case "returns":
      return returnRulesSectionComplete();
    case "buyers":
      return true;
    case "fieldmap":
    case "triggers":
      return true;
  }
}
