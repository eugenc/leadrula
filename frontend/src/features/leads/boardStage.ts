import type { AccountType, Lead } from "@/types";

/** Mirrors backend boardStageSQL in leads/repository.go */
export function computeBoardStageId(
  lead: Pick<Lead, "publisher_stage_id" | "owner_account_id" | "publisher_id" | "stage_id">,
  accountType: AccountType | undefined
): number | null {
  if (
    accountType === "publisher" &&
    lead.publisher_stage_id != null &&
    lead.owner_account_id !== lead.publisher_id
  ) {
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

/** Synthetic stage id for leads whose stage is missing or not in the selected pipeline. */
export const UNPLACED_BOARD_STAGE_ID = -1;

export function groupLeadsForBoard(
  leads: Lead[],
  pipelineStageIds: Set<number>,
  accountType: AccountType | undefined
): { grouped: Record<number, Lead[]>; unplaced: Lead[] } {
  const grouped: Record<number, Lead[]> = {};
  const unplaced: Lead[] = [];
  for (const l of leads) {
    const sid = computeBoardStageId(l, accountType);
    if (sid == null || !pipelineStageIds.has(sid)) {
      unplaced.push(l);
    } else {
      (grouped[sid] ??= []).push(l);
    }
  }
  return { grouped, unplaced };
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
