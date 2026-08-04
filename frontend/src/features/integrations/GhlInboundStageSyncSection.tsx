import { Label, Select, Input } from "@/components/ui/input";
import { usePipelines } from "@/features/leads/hooks";
import type { GHLConfig } from "@/features/integrations/ghlConstants";

type GhlPipeline = { id: string; name: string };

export function GhlInboundStageSyncSection({
  config,
  onChange,
  ghlPipelines,
  ghlPipelinesLoading = false,
  webhookMode = false,
}: {
  config: GHLConfig;
  onChange: (config: GHLConfig) => void;
  ghlPipelines: GhlPipeline[];
  ghlPipelinesLoading?: boolean;
  webhookMode?: boolean;
}) {
  const { data: pipelines } = usePipelines();
  const enabled = !!config.inbound_stage_sync_enabled;
  const lrPipelineID = config.inbound_sync_leadrula_pipeline_id ?? 0;
  const mapEntries = config.pipeline_stage_map ?? [];
  const hasMapForPipeline = mapEntries.some(
    (e) =>
      e.leadrula_pipeline_id === lrPipelineID &&
      e.leadrula_stage_id > 0 &&
      e.ghl_pipeline_id &&
      e.ghl_pipeline_stage_id
  );

  const ghlPipelineOptions = (() => {
    const ids = new Set<string>();
    for (const p of ghlPipelines) ids.add(p.id);
    for (const e of mapEntries) {
      if (e.ghl_pipeline_id) ids.add(e.ghl_pipeline_id);
    }
    return [...ids].map((id) => ({
      id,
      name: ghlPipelines.find((p) => p.id === id)?.name ?? id,
    }));
  })();

  function patch(p: Partial<GHLConfig>) {
    onChange({ ...config, ...p });
  }

  function onToggle(enabledNext: boolean) {
    if (!enabledNext) {
      patch({ inbound_stage_sync_enabled: false });
      return;
    }
    patch({ inbound_stage_sync_enabled: true });
  }

  function onLRPipelineChange(pipelineID: number) {
    const firstMatch = mapEntries.find((e) => e.leadrula_pipeline_id === pipelineID && e.ghl_pipeline_id);
    patch({
      inbound_sync_leadrula_pipeline_id: pipelineID,
      inbound_sync_ghl_pipeline_id:
        firstMatch?.ghl_pipeline_id ?? (config.inbound_sync_ghl_pipeline_id && pipelineID === lrPipelineID ? config.inbound_sync_ghl_pipeline_id : ""),
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
        Auto-sync pipeline stages from GHL
      </label>
      <p className="text-xs text-gray-500">
        When GHL moves an opportunity in the selected pipeline, the matching Leadrula lead moves to the mapped
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
            <Label>GHL pipeline</Label>
            {ghlPipelineOptions.length > 0 ? (
              <Select
                value={config.inbound_sync_ghl_pipeline_id ?? ""}
                onChange={(e) => patch({ inbound_sync_ghl_pipeline_id: e.target.value })}
              >
                <option value="">Select…</option>
                {ghlPipelineOptions.map((p) => (
                  <option key={p.id} value={p.id}>
                    {p.name}
                  </option>
                ))}
              </Select>
            ) : (
              <Input
                value={config.inbound_sync_ghl_pipeline_id ?? ""}
                onChange={(e) => patch({ inbound_sync_ghl_pipeline_id: e.target.value })}
                placeholder="GHL pipeline ID from workflow payload"
              />
            )}
            {webhookMode && ghlPipelineOptions.length === 0 && (
              <p className="mt-1 text-xs text-gray-400">
                {ghlPipelinesLoading
                  ? "Loading GHL pipelines…"
                  : "Enter the GHL pipeline ID from your workflow payload, or test the API connection to load pipelines."}
              </p>
            )}
          </div>
          {lrPipelineID > 0 && !hasMapForPipeline && (
            <p className="sm:col-span-2 text-xs text-amber-700">
              Add stage mappings below for this pipeline with both Leadrula and GHL stages.
            </p>
          )}
        </div>
      )}
    </div>
  );
}
