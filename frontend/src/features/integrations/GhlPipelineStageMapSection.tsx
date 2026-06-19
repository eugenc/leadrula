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
  manualGhlIds = false,
}: {
  entries: GHLPipelineStageMapEntry[];
  onChange: (entries: GHLPipelineStageMapEntry[]) => void;
  ghlPipelines: GhlPipeline[];
  ghlPipelinesLoading?: boolean;
  manualGhlIds?: boolean;
}) {
  const { data: pipelines } = usePipelines();

  function addRow() {
    onChange([
      ...entries,
      { leadrula_pipeline_id: 0, leadrula_stage_id: 0, ghl_pipeline_id: "", ghl_pipeline_stage_id: "" },
    ]);
  }

  function removeRow(idx: number) {
    onChange(entries.filter((_, i) => i !== idx));
  }

  function updateRow(idx: number, patch: Partial<GHLPipelineStageMapEntry>) {
    const next = [...entries];
    next[idx] = { ...next[idx], ...patch };
    onChange(next);
  }

  return (
    <div className="space-y-3 rounded-lg border border-gray-100 p-3">
      <div className="flex items-center justify-between">
        <Label>Pipeline / stage mapping</Label>
        <Button size="sm" variant="secondary" onClick={addRow}>
          <Plus className="h-3.5 w-3.5" /> Add row
        </Button>
      </div>
      <p className="text-xs text-gray-400">
        {manualGhlIds
          ? "Map each Leadrula stage to GHL pipeline/stage IDs included in the webhook payload."
          : "Map each Leadrula stage to a GHL pipeline stage when pushing opportunities."}
      </p>
      {!manualGhlIds && !ghlPipelinesLoading && ghlPipelines.length === 0 && (
        <p className="text-xs text-gray-400">Click Test connection to load pipelines from GHL.</p>
      )}
      {entries.length === 0 ? (
        <p className="text-sm text-gray-400">No mappings yet.</p>
      ) : (
        <Table>
          <THead>
            <tr>
              <TH>Leadrula pipeline</TH>
              <TH>Leadrula stage</TH>
              <TH>GHL pipeline</TH>
              <TH>GHL stage</TH>
              <TH className="w-12" />
            </tr>
          </THead>
          <TBody>
            {entries.map((e, idx) => (
              <PipelineStageRow
                key={idx}
                entry={e}
                pipelines={pipelines ?? []}
                ghlPipelines={ghlPipelines}
                manualGhlIds={manualGhlIds}
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
  manualGhlIds,
  onChange,
  onRemove,
}: {
  entry: GHLPipelineStageMapEntry;
  pipelines: { id: number; name: string }[];
  ghlPipelines: GhlPipeline[];
  manualGhlIds?: boolean;
  onChange: (patch: Partial<GHLPipelineStageMapEntry>) => void;
  onRemove: () => void;
}) {
  const { data: stages } = useStages(entry.leadrula_pipeline_id || undefined);
  const ghlPipeline = ghlPipelines.find((p) => p.id === entry.ghl_pipeline_id);
  const ghlStages = ghlPipeline?.stages ?? [];

  return (
    <TR>
      <TD>
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
      <TD>
        {manualGhlIds ? (
          <Input
            className="!h-8 !text-sm"
            value={entry.ghl_pipeline_id}
            onChange={(ev) => onChange({ ghl_pipeline_id: ev.target.value, ghl_pipeline_stage_id: "" })}
            placeholder="GHL pipeline ID"
          />
        ) : (
          <Select
            className="!h-8 !text-sm"
            value={entry.ghl_pipeline_id}
            onChange={(ev) => {
              onChange({ ghl_pipeline_id: ev.target.value, ghl_pipeline_stage_id: "" });
            }}
          >
            <option value="">Select…</option>
            {ghlPipelines.map((p) => (
              <option key={p.id} value={p.id}>
                {p.name}
              </option>
            ))}
          </Select>
        )}
      </TD>
      <TD>
        {manualGhlIds ? (
          <Input
            className="!h-8 !text-sm"
            value={entry.ghl_pipeline_stage_id}
            onChange={(ev) => onChange({ ghl_pipeline_stage_id: ev.target.value })}
            placeholder="GHL stage ID"
          />
        ) : (
          <Select
            className="!h-8 !text-sm"
            value={entry.ghl_pipeline_stage_id}
            onChange={(ev) => onChange({ ghl_pipeline_stage_id: ev.target.value })}
          >
            <option value="">Select…</option>
            {ghlStages.map((s) => (
              <option key={s.id} value={s.id}>
                {s.name}
              </option>
            ))}
          </Select>
        )}
      </TD>
      <TD>
        <IconButton variant="danger" aria-label="Remove" onClick={onRemove}>
          <Trash2 className="h-4 w-4" />
        </IconButton>
      </TD>
    </TR>
  );
}
