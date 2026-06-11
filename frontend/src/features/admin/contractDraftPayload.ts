import {
  compensationDraftToBody,
  type CompensationDraft,
} from "@/features/admin/CreateContractCompensationList";
import { deliveryDraftToBody, type ContractDeliveryDraft } from "@/features/admin/contractCompensation";
import { normalizeContractOffer, type ContractOfferDraft } from "@/features/admin/contractOffer";
import type { ContractLeadCriteria } from "@/types";

export function buildContractPayload(args: {
  status: "draft" | "active";
  form: {
    contract_type: string;
    buyer_id: number;
    name: string;
    lead_type: string;
    description: string;
  };
  compensations: CompensationDraft[];
  delivery: ContractDeliveryDraft;
  leadCriteria: ContractLeadCriteria;
  offer?: ContractOfferDraft;
}): Record<string, unknown> {
  const first = args.compensations[0];
  const rate = first?.kind === "flat_rate" && first.flat_amount !== "" ? Number(first.flat_amount) : 0;
  const deliveryBody = deliveryDraftToBody(args.delivery);
  const body: Record<string, unknown> = {
    status: args.status,
    contract_type: args.form.contract_type,
    name: args.form.name.trim(),
    description: args.form.description,
    rate_per_lead: rate,
    ...deliveryBody,
    lead_criteria: { ...args.leadCriteria, field_map: [] },
  };
  if (args.form.buyer_id) body.buyer_id = args.form.buyer_id;
  if (args.form.lead_type) body.lead_type = args.form.lead_type;
  if (args.offer && args.form.contract_type === "sell" && !args.form.buyer_id) {
    const offer = normalizeContractOffer(args.offer);
    body.allowed_delivery_modes = offer.allowed_delivery_modes;
    body.distribution_strategy = offer.distribution_strategy;
  }
  if (first) {
    body.cap_period = first.cap_period ?? "one_time";
    body.cap_total = compensationDraftToBody(first, args.delivery).cap_total;
    body.cap_max_daily = compensationDraftToBody(first, args.delivery).cap_max_daily;
  }
  if (args.compensations.length > 0) {
    body.compensations = args.compensations.map((d) => compensationDraftToBody(d, args.delivery));
  }
  return body;
}
