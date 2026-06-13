import type { Route } from "@/types";

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

export function formatRouteTarget(r: Route) {
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
