import type { Route, RouteBranch } from "@/types";
import { summarizeBranchConditions } from "./RouteConditionsEditor";

export function pipelineStage(pipeline?: string | null, stage?: string | null) {
  if (!pipeline) return "—";
  if (!stage) return `${pipeline} > First stage`;
  return `${pipeline} > ${stage}`;
}

export function formatRouteOrigin(r: Route) {
  switch (r.origin) {
    case "source":
      return `Source: ${r.source_name ?? `#${r.source_id}`}`;
    case "pipeline":
      return `Pipeline: ${pipelineStage(r.origin_pipeline_name, r.origin_stage_name)}`;
    case "webhook":
      return `Webhook: ${r.origin_webhook_name ?? `#${r.origin_webhook_id}`}`;
    case "integration":
      return `Integration: ${r.origin_connection_name ?? `#${r.origin_connection_id}`}`;
    default:
      return "—";
  }
}

export function branchLabel(branch: RouteBranch, index: number): string {
  const n = branch.name?.trim();
  return n || `Route ${index + 1}`;
}

export function formatBranchTarget(branch: RouteBranch): string {
  switch (branch.destination) {
    case "contract":
      if (branch.delivery === "leads") return "Contract (lead)";
      return "Contract (pipeline)";
    case "pipeline":
      if (branch.delivery === "leads") return "Pipeline (lead)";
      return "Pipeline";
    case "webhook":
      return `Webhook #${branch.dest_webhook_id ?? "?"}`;
    case "integration":
      return "Integrations";
    default:
      return "—";
  }
}

export function formatRouteBranchesSummary(r: Route): string {
  const branches = r.branches ?? [];
  if (branches.length === 0) return "—";
  if (branches.length === 1) return summarizeBranchConditions(branches[0]);
  const parts = branches.slice(0, 3).map((b, i) => branchLabel(b, i));
  const suffix = branches.length > 3 ? ` · +${branches.length - 3} more` : "";
  return `${branches.length} routes · ${parts.join(" · ")}${suffix}`;
}

export function formatRouteTargetsSummary(r: Route): string {
  const branches = r.branches ?? [];
  if (branches.length === 0) return formatRouteTargetLegacy(r);
  if (branches.length === 1) return formatBranchTarget(branches[0]);
  return branches.map((b, i) => `${branchLabel(b, i)} → ${formatBranchTarget(b)}`).join(" · ");
}

function formatRouteTargetLegacy(r: Route) {
  switch (r.destination) {
    case "contract":
      if (r.delivery === "leads") return `Contract: ${r.contract_name ?? r.buyer_name ?? "Lead inbox"}`;
      return `Contract: ${pipelineStage(r.target_pipeline_name ?? r.contract_name, r.target_stage_name)}`;
    case "pipeline":
      if (r.delivery === "leads") return "Pipeline (lead)";
      return `Pipeline: ${pipelineStage(r.target_pipeline_name, r.target_stage_name)}`;
    case "webhook":
      return `Webhook: ${r.dest_webhook_name ?? `#${r.dest_webhook_id}`}`;
    case "integration":
      return "Integrations";
    default:
      return "—";
  }
}

/** @deprecated use formatRouteTargetsSummary */
export function formatRouteTarget(r: Route) {
  return formatRouteTargetsSummary(r);
}

export function routeEditableByBuyer(r: Route) {
  return !!r.buyer_id;
}

export const PUBLISHER_ORIGINS = ["source", "pipeline", "webhook", "integration"] as const;
export const BUYER_ORIGINS = ["pipeline", "webhook", "integration"] as const;
export const PUBLISHER_DESTINATIONS = ["contract", "pipeline", "webhook", "integration"] as const;
export const BUYER_DESTINATIONS = ["pipeline", "webhook", "integration"] as const;

export const ORIGIN_LABELS: Record<string, string> = {
  source: "Source",
  pipeline: "Pipeline",
  webhook: "Webhook",
  integration: "Integration",
};

export const DESTINATION_LABELS: Record<string, string> = {
  contract: "Contract",
  pipeline: "Pipeline",
  webhook: "Webhook",
  integration: "Integrations",
};
