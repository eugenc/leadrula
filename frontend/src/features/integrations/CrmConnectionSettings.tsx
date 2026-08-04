import {
  type CRMInboundConfig,
  type CrmPipelineOption,
  crmProviderLabel,
} from "@/features/integrations/crmConstants";
import { CrmInboundStageSyncSection } from "@/features/integrations/CrmInboundStageSyncSection";
import { CrmPipelineStageMapSection } from "@/features/integrations/CrmPipelineStageMapSection";

export function CrmConnectionSettings({
  providerSlug,
  config,
  onChange,
  crmPipelines,
  crmPipelinesLoading = false,
}: {
  providerSlug: string;
  config: CRMInboundConfig;
  onChange: (config: CRMInboundConfig) => void;
  crmPipelines: CrmPipelineOption[];
  crmPipelinesLoading?: boolean;
}) {
  const providerLabel = crmProviderLabel(providerSlug);
  const inboundSyncEnabled = !!config.inbound_stage_sync_enabled;
  const inboundSyncPipelineID = config.inbound_sync_leadrula_pipeline_id ?? 0;
  const filteredStageMap =
    inboundSyncEnabled && inboundSyncPipelineID > 0
      ? (config.pipeline_stage_map ?? []).filter((e) => e.leadrula_pipeline_id === inboundSyncPipelineID)
      : (config.pipeline_stage_map ?? []);

  function patch(p: Partial<CRMInboundConfig>) {
    onChange({ ...config, ...p });
  }

  function patchStageMap(entries: CRMInboundConfig["pipeline_stage_map"]) {
    const rows = entries ?? [];
    if (!inboundSyncEnabled || inboundSyncPipelineID <= 0) {
      patch({ pipeline_stage_map: rows });
      return;
    }
    const other = (config.pipeline_stage_map ?? []).filter((e) => e.leadrula_pipeline_id !== inboundSyncPipelineID);
    patch({ pipeline_stage_map: [...other, ...rows] });
  }

  return (
    <div className="space-y-4">
      <CrmInboundStageSyncSection
        config={config}
        onChange={onChange}
        providerLabel={providerLabel}
        crmPipelines={crmPipelines}
        crmPipelinesLoading={crmPipelinesLoading}
      />
      <CrmPipelineStageMapSection
        entries={inboundSyncEnabled ? filteredStageMap : (config.pipeline_stage_map ?? [])}
        onChange={patchStageMap}
        providerLabel={providerLabel}
        crmPipelines={crmPipelines}
        crmPipelinesLoading={crmPipelinesLoading}
        syncEnabled={inboundSyncEnabled}
        defaultLeadrulaPipelineId={inboundSyncEnabled ? inboundSyncPipelineID : undefined}
      />
    </div>
  );
}
