import type { RouteBranch } from "@/types";

export function blankBranch(position: number, accountType: "publisher" | "buyer"): RouteBranch {
  return {
    name: `Route ${position + 1}`,
    position,
    condition_logic: "and",
    conditions: [],
    destination: accountType === "publisher" ? "contract" : "pipeline",
    delivery: "leads_pipeline",
    target_pipeline_id: null,
    target_stage_id: null,
    contract_id: null,
    dest_webhook_id: null,
  };
}

export function reindexBranches(branches: RouteBranch[]): RouteBranch[] {
  return branches.map((b, i) => ({ ...b, position: i }));
}

function branchValid(branch: RouteBranch): boolean {
  switch (branch.destination) {
    case "contract":
      return !!branch.contract_id;
    case "pipeline":
      return branch.delivery === "leads" || (!!branch.target_pipeline_id && !!branch.target_stage_id);
    case "webhook":
      return !!branch.dest_webhook_id;
    case "integration":
      return true;
    default:
      return false;
  }
}

export function branchDestinationValid(branch: RouteBranch, integrationCount: number): boolean {
  if (branch.destination === "integration") return integrationCount > 0;
  return branchValid(branch);
}
