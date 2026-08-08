export type BuyerDeliveryDraft = {
  delivery: string;
  pipelineId: number;
  stageId: number;
  webhookId: number;
  integrationId: number;
};

type BuyerDeliveryServer = {
  delivery?: string | null;
  buyer_pipeline_id?: number | null;
  buyer_target_stage_id?: number | null;
  outbound_webhook_id?: number | null;
  integration_connection_id?: number | null;
};

export function buyerDeliveryDraftFrom(
  server: BuyerDeliveryServer,
  allowed: string[]
): BuyerDeliveryDraft {
  const delivery =
    server.delivery || (server.buyer_pipeline_id ? "leads_pipeline" : allowed[0] || "leads");
  return {
    delivery,
    pipelineId: server.buyer_pipeline_id ?? 0,
    stageId: server.buyer_target_stage_id ?? 0,
    webhookId: server.outbound_webhook_id ?? 0,
    integrationId: server.integration_connection_id ?? 0,
  };
}

export function buyerDeliveryDirty(
  local: BuyerDeliveryDraft,
  server: BuyerDeliveryServer,
  allowed: string[]
): boolean {
  const saved = buyerDeliveryDraftFrom(server, allowed);
  return (
    local.delivery !== saved.delivery ||
    local.pipelineId !== saved.pipelineId ||
    local.stageId !== saved.stageId ||
    local.webhookId !== saved.webhookId ||
    local.integrationId !== saved.integrationId
  );
}

export function buyerDeliveryBody(local: BuyerDeliveryDraft): Record<string, unknown> {
  const pipelineDelivery = local.delivery === "leads_pipeline";
  const body: Record<string, unknown> = { delivery: local.delivery };
  if (pipelineDelivery) {
    body.buyer_pipeline_id = local.pipelineId;
    body.buyer_target_stage_id = local.stageId;
  }
  if (local.delivery === "webhook" && local.webhookId) {
    body.outbound_webhook_id = local.webhookId;
  }
  if (local.integrationId) {
    body.integration_connection_id = local.integrationId;
  }
  return body;
}
