export type CRMPipelineStageMapEntry = {
  leadrula_pipeline_id: number;
  leadrula_stage_id: number;
  crm_pipeline_id?: string;
  crm_stage_id?: string;
  ghl_pipeline_id?: string;
  ghl_pipeline_stage_id?: string;
};

export type CRMInboundConfig = {
  inbound_stage_sync_enabled?: boolean;
  inbound_sync_leadrula_pipeline_id?: number;
  inbound_sync_crm_pipeline_id?: string;
  inbound_sync_ghl_pipeline_id?: string;
  pipeline_stage_map?: CRMPipelineStageMapEntry[];
};

export const CONFIGURABLE_CRM_SLUGS = new Set(["pipedrive", "hubspot", "zoho_crm"]);

export function isConfigurableCrm(slug: string): boolean {
  return CONFIGURABLE_CRM_SLUGS.has(slug);
}

export function crmProviderLabel(slug: string): string {
  switch (slug) {
    case "pipedrive":
      return "Pipedrive";
    case "hubspot":
      return "HubSpot";
    case "zoho_crm":
      return "Zoho CRM";
    case "ghl":
      return "GoHighLevel";
    default:
      return slug;
  }
}

export function entryCrmPipelineId(e: CRMPipelineStageMapEntry): string {
  return (e.crm_pipeline_id || e.ghl_pipeline_id || "").trim();
}

export function entryCrmStageId(e: CRMPipelineStageMapEntry): string {
  return (e.crm_stage_id || e.ghl_pipeline_stage_id || "").trim();
}

export function syncCrmPipelineId(config: CRMInboundConfig): string {
  return (config.inbound_sync_crm_pipeline_id || config.inbound_sync_ghl_pipeline_id || "").trim();
}

export function normalizeCrmConfig(raw: Record<string, unknown>): CRMInboundConfig {
  const map = (raw.pipeline_stage_map as CRMPipelineStageMapEntry[] | undefined) ?? [];
  return {
    inbound_stage_sync_enabled: !!raw.inbound_stage_sync_enabled,
    inbound_sync_leadrula_pipeline_id: Number(raw.inbound_sync_leadrula_pipeline_id) || undefined,
    inbound_sync_crm_pipeline_id:
      typeof raw.inbound_sync_crm_pipeline_id === "string" ? raw.inbound_sync_crm_pipeline_id : undefined,
    inbound_sync_ghl_pipeline_id:
      typeof raw.inbound_sync_ghl_pipeline_id === "string" ? raw.inbound_sync_ghl_pipeline_id : undefined,
    pipeline_stage_map: map,
  };
}

export type CrmPipelineOption = {
  id: string;
  name: string;
  stages?: { id: string; name: string }[];
};

export function crmPipelinesToOptions(
  pipelines: { external_id: string; name: string; stages: { external_id: string; name: string }[] }[]
): CrmPipelineOption[] {
  return pipelines.map((p) => ({
    id: p.external_id,
    name: p.name,
    stages: p.stages.map((s) => ({ id: s.external_id, name: s.name })),
  }));
}
