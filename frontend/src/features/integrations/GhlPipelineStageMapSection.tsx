import { Plus, Trash2 } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Label, Select, Input } from "@/components/ui/input";
import { IconButton } from "@/components/layout/IconButton";
import { Table, THead, TH, TBody, TR, TD } from "@/components/ui/table";
import { usePipelines, useStages } from "@/features/leads/hooks";
import type { GHLPipelineStageMapEntry } from "@/features/integrations/ghlConstants";

type GhlPipeline = { id: string; name: string; stages?: { id: string; name: string }[] };

export function GhlPipelineStageMapSection({
  entries,
  onChange,
  ghlPipelines,
  ghlPipelinesLoading = false,
  triggerOnly = false,
  syncEnabled = false,
  defaultLeadrulaPipelineId,
}: {
  entries: GHLPipelineStageMapEntry[];
  onChange: (entries: GHLPipelineStageMapEntry[]) => void;
  ghlPipelines: GhlPipeline[];
  ghlPipelinesLoading?: boolean;
  triggerOnly?: boolean;
  syncEnabled?: boolean;
  defaultLeadrulaPipelineId?: number;
}) {
  const { data: pipelines } = usePipelines();
  const rows = entries ?? [];
  const ghlOpts = ghlPipelines ?? [];
  const showGHLFields = !triggerOnly;

  function addRow() {
    onChange([
      ...rows,
      {
        leadrula_pipeline_id: defaultLeadrulaPipelineId ?? 0,
        leadrula_stage_id: 0,
        ghl_pipeline_id: "",
        ghl_pipeline_stage_id: "",
      },
    ]);
  }

  function removeRow(idx: number) {
    onChange(rows.filter((_, i) => i !== idx));
  }

  function updateRow(idx: number, patch: Partial<GHLPipelineStageMapEntry>) {
    const next = [...rows];
    next[idx] = { ...next[idx], ...patch };
    onChange(next);
  }

  return (
    <div className="space-y-3 rounded-lg border border-gray-100 p-3">
      <div className="flex items-center justify-between">
        <Label>{triggerOnly ? "Outbound trigger stages" : "Pipeline / stage mapping"}</Label>
        <Button size="sm" variant="secondary" onClick={addRow}>
          <Plus className="h-3.5 w-3.5" /> Add row
        </Button>
      </div>
      <p className="text-xs text-gray-400">
        {triggerOnly
          ? "Push to GHL when a lead enters these Leadrula pipeline stages. At least one trigger stage is required."
          : syncEnabled
            ? "Map each Leadrula stage to its GHL counterpart for bidirectional sync."
            : "Map each Leadrula stage to a GHL pipeline stage when pushing opportunities."}
      </p>
      {showGHLFields && !ghlPipelinesLoading && ghlOpts.length === 0 && (
        <p className="text-xs text-gray-400">
          GHL pipeline and stage IDs can be entered manually, or click Test connection to load pipelines from GHL.
        </p>
      )}
      {!triggerOnly && !ghlPipelinesLoading && ghlOpts.length === 0 && !syncEnabled && (
        <p className="text-xs text-gray-400">Click Test connection to load pipelines from GHL.</p>
      )}
      {rows.length === 0 ? (
        <p className="text-sm text-gray-400">No mappings yet.</p>
      ) : (
        <Table>
          <THead>
            <tr>
              <TH>Leadrula pipeline</TH>
              <TH>Leadrula stage</TH>
              {!triggerOnly && (
                <>
                  <TH>GHL pipeline</TH>
                  <TH>GHL stage</TH>
                </>
              )}
              <TH className="w-12" />
            </tr>
          </THead>
          <TBody>
            {rows.map((e, idx) => (
              <PipelineStageRow
                key={idx}
                entry={e}
                pipelines={pipelines ?? []}
                ghlPipelines={ghlOpts}
                triggerOnly={triggerOnly}
                showGHLFields={showGHLFields}
                lockLeadrulaPipeline={defaultLeadrulaPipelineId}
                onChange={(patch) => updateRow(idx, patch)}
                onRemove={() => removeRow(idx)}
              />
            ))}
          </TBody>
        </Table>
      )}
    </div>
  );
}

function PipelineStageRow({
  entry,
  pipelines,
  ghlPipelines,
  triggerOnly,
  showGHLFields,
  lockLeadrulaPipeline,
  onChange,
  onRemove,
}: {
  entry: GHLPipelineStageMapEntry;
  pipelines: { id: number; name: string }[];
  ghlPipelines: GhlPipeline[];
  triggerOnly?: boolean;
  showGHLFields?: boolean;
  lockLeadrulaPipeline?: number;
  onChange: (patch: Partial<GHLPipelineStageMapEntry>) => void;
  onRemove: () => void;
}) {
  const lrPipelineLocked = lockLeadrulaPipeline != null && lockLeadrulaPipeline > 0;
  const lrPipelineID = lrPipelineLocked ? lockLeadrulaPipeline! : entry.leadrula_pipeline_id;
  const { data: stages } = useStages(lrPipelineID || undefined);
  const ghlPipeline = ghlPipelines.find((p) => p.id === entry.ghl_pipeline_id);
  const ghlStages = ghlPipeline?.stages ?? [];

  return (
    <TR>
      <TD>
        {lrPipelineLocked ? (
          <span className="text-sm text-gray-700">
            {pipelines.find((p) => p.id === lockLeadrulaPipeline)?.name ?? `Pipeline ${lockLeadrulaPipeline}`}
          </span>
        ) : (
          <Select
            className="!h-8 !text-sm"
            value={entry.leadrula_pipeline_id}
            onChange={(ev) => {
              onChange({ leadrula_pipeline_id: Number(ev.target.value), leadrula_stage_id: 0 });
            }}
          >
            <option value={0}>Select…</option>
            {pipelines.map((p) => (
              <option key={p.id} value={p.id}>
                {p.name}
              </option>
            ))}
          </Select>
        )}
      </TD>
      <TD>
        <Select
          className="!h-8 !text-sm"
          value={entry.leadrula_stage_id}
          onChange={(ev) => onChange({ leadrula_stage_id: Number(ev.target.value) })}
        >
          <option value={0}>Select…</option>
          {(stages ?? []).map((s) => (
            <option key={s.id} value={s.id}>
              {s.name}
            </option>
          ))}
        </Select>
      </TD>
      {!triggerOnly && showGHLFields && (
        <>
          <TD>
            {ghlPipelines.length > 0 ? (
              <Select
                className="!h-8 !text-sm"
                value={entry.ghl_pipeline_id}
                onChange={(ev) => {
                  onChange({ ghl_pipeline_id: ev.target.value, ghl_pipeline_stage_id: "", ghl_stage_name: "" });
                }}
              >
                <option value="">Select…</option>
                {ghlPipelines.map((p) => (
                  <option key={p.id} value={p.id}>
                    {p.name}
                  </option>
                ))}
              </Select>
            ) : (
              <Input
                className="!h-8 !text-sm font-mono"
                value={entry.ghl_pipeline_id}
                onChange={(ev) => onChange({ ghl_pipeline_id: ev.target.value, ghl_pipeline_stage_id: "", ghl_stage_name: "" })}
                placeholder="GHL pipeline ID"
              />
            )}
          </TD>
          <TD>
            {ghlStages.length > 0 ? (
              <div className="space-y-0.5">
                <Select
                  className="!h-8 !text-sm"
                  value={entry.ghl_pipeline_stage_id}
                  onChange={(ev) => {
                    const stage = ghlStages.find((s) => s.id === ev.target.value);
                    onChange({
                      ghl_pipeline_stage_id: ev.target.value,
                      ghl_stage_name: stage?.name ?? "",
                    });
                  }}
                >
                  <option value="">Select…</option>
                  {ghlStages.map((s) => (
                    <option key={s.id} value={s.id}>
                      {s.name}
                    </option>
                  ))}
                </Select>
                {entry.ghl_pipeline_stage_id && (
                  <p className="truncate font-mono text-[10px] text-gray-400" title={entry.ghl_pipeline_stage_id}>
                    {entry.ghl_pipeline_stage_id}
                  </p>
                )}
              </div>
            ) : (
              <Input
                className="!h-8 !text-sm font-mono"
                value={entry.ghl_pipeline_stage_id}
                onChange={(ev) => onChange({ ghl_pipeline_stage_id: ev.target.value, ghl_stage_name: "" })}
                placeholder="GHL stage ID"
              />
            )}
          </TD>
        </>
      )}
      <TD>
        <IconButton variant="danger" aria-label="Remove" onClick={onRemove}>
          <Trash2 className="h-4 w-4" />
        </IconButton>
      </TD>
    </TR>
  );
}
