import type { AccountType, Lead } from "@/types";

/** Mirrors backend boardStageExpr in leads/repository.go */
export function computeBoardStageId(
  lead: Pick<Lead, "publisher_stage_id" | "owner_account_id" | "publisher_id" | "stage_id">
): number | null {
  if (lead.publisher_stage_id != null && lead.owner_account_id !== lead.publisher_id) {
    return lead.publisher_stage_id;
  }
  return lead.stage_id ?? null;
}

/** Distributed lead shown on publisher board (buyer owns it). */
export function isPublisherTrackedLead(
  lead: Pick<Lead, "owner_account_id" | "publisher_id">
): boolean {
  return lead.owner_account_id !== lead.publisher_id;
}

export function isBoardDraggable(
  lead: Pick<Lead, "owner_account_id" | "publisher_id">,
  accountType: AccountType | undefined
): boolean {
  if (accountType === "publisher") {
    return lead.owner_account_id === lead.publisher_id;
  }
  return true;
}
