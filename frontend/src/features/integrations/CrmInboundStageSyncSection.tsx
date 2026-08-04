import { Label, Select, Input } from "@/components/ui/input";
import { usePipelines } from "@/features/leads/hooks";
import {
  type CRMInboundConfig,
  type CrmPipelineOption,
  entryCrmPipelineId,
  entryCrmStageId,
  syncCrmPipelineId,
} from "@/features/integrations/crmConstants";

export function CrmInboundStageSyncSection({
  config,
  onChange,
  providerLabel,
  crmPipelines,
  crmPipelinesLoading = false,
}: {
  config: CRMInboundConfig;
  onChange: (config: CRMInboundConfig) => void;
  providerLabel: string;
  crmPipelines: CrmPipelineOption[];
  crmPipelinesLoading?: boolean;
}) {
  const { data: pipelines } = usePipelines();
  const enabled = !!config.inbound_stage_sync_enabled;
  const lrPipelineID = config.inbound_sync_leadrula_pipeline_id ?? 0;
  const mapEntries = config.pipeline_stage_map ?? [];
  const hasMapForPipeline = mapEntries.some(
    (e) =>
      e.leadrula_pipeline_id === lrPipelineID &&
      e.leadrula_stage_id > 0 &&
      entryCrmPipelineId(e) &&
      entryCrmStageId(e)
  );

  const crmPipelineOptions = (() => {
    const ids = new Set<string>();
    for (const p of crmPipelines) ids.add(p.id);
    for (const e of mapEntries) {
      const id = entryCrmPipelineId(e);
      if (id) ids.add(id);
    }
    return [...ids].map((id) => ({
      id,
      name: crmPipelines.find((p) => p.id === id)?.name ?? id,
    }));
  })();

  function patch(p: Partial<CRMInboundConfig>) {
    onChange({ ...config, ...p });
  }

  function onToggle(enabledNext: boolean) {
    patch({ inbound_stage_sync_enabled: enabledNext });
  }

  function onLRPipelineChange(pipelineID: number) {
    const firstMatch = mapEntries.find((e) => e.leadrula_pipeline_id === pipelineID && entryCrmPipelineId(e));
    const currentCrmPipeline = syncCrmPipelineId(config);
    patch({
      inbound_sync_leadrula_pipeline_id: pipelineID,
      inbound_sync_crm_pipeline_id:
        firstMatch?.crm_pipeline_id ??
        firstMatch?.ghl_pipeline_id ??
        (pipelineID === lrPipelineID ? currentCrmPipeline : ""),
    });
  }

  return (
    <div className="space-y-3 rounded-lg border border-gray-100 p-3">
      <label className="flex items-center gap-2 text-sm font-medium text-gray-900">
        <input
          type="checkbox"
          checked={enabled}
          onChange={(e) => onToggle(e.target.checked)}
          className="rounded"
        />
        Auto-sync pipeline stages from {providerLabel}
      </label>
      <p className="text-xs text-gray-500">
        When {providerLabel} moves a deal in the selected pipeline, the matching Leadrula lead moves to the mapped
        stage below.
      </p>

      {enabled && (
        <div className="grid gap-3 sm:grid-cols-2">
          <div>
            <Label>Leadrula pipeline</Label>
            <Select
              value={lrPipelineID}
              onChange={(e) => onLRPipelineChange(Number(e.target.value))}
            >
              <option value={0}>Select…</option>
              {(pipelines ?? []).map((p) => (
                <option key={p.id} value={p.id}>
                  {p.name}
                </option>
              ))}
            </Select>
          </div>
          <div>
            <Label>{providerLabel} pipeline</Label>
            {crmPipelineOptions.length > 0 ? (
              <Select
                value={syncCrmPipelineId(config)}
                onChange={(e) => patch({ inbound_sync_crm_pipeline_id: e.target.value })}
              >
                <option value="">Select…</option>
                {crmPipelineOptions.map((p) => (
                  <option key={p.id} value={p.id}>
                    {p.name}
                  </option>
                ))}
              </Select>
            ) : (
              <Input
                value={syncCrmPipelineId(config)}
                onChange={(e) => patch({ inbound_sync_crm_pipeline_id: e.target.value })}
                placeholder={`${providerLabel} pipeline ID`}
              />
            )}
            {crmPipelineOptions.length === 0 && (
              <p className="mt-1 text-xs text-gray-400">
                {crmPipelinesLoading
                  ? `Loading ${providerLabel} pipelines…`
                  : `Enter the ${providerLabel} pipeline ID from webhook payloads, or wait for pipelines to load.`}
              </p>
            )}
          </div>
          {lrPipelineID > 0 && !hasMapForPipeline && (
            <p className="sm:col-span-2 text-xs text-amber-700">
              Add stage mappings below for this pipeline with both Leadrula and {providerLabel} stages.
            </p>
          )}
        </div>
      )}
    </div>
  );
}
