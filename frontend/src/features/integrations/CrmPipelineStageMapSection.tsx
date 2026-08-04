import { Plus, Trash2 } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Label, Select, Input } from "@/components/ui/input";
import { IconButton } from "@/components/layout/IconButton";
import { Table, THead, TH, TBody, TR, TD } from "@/components/ui/table";
import { usePipelines, useStages } from "@/features/leads/hooks";
import {
  type CRMPipelineStageMapEntry,
  type CrmPipelineOption,
  entryCrmPipelineId,
  entryCrmStageId,
} from "@/features/integrations/crmConstants";

export function CrmPipelineStageMapSection({
  entries,
  onChange,
  providerLabel,
  crmPipelines,
  crmPipelinesLoading = false,
  syncEnabled = false,
  defaultLeadrulaPipelineId,
}: {
  entries: CRMPipelineStageMapEntry[];
  onChange: (entries: CRMPipelineStageMapEntry[]) => void;
  providerLabel: string;
  crmPipelines: CrmPipelineOption[];
  crmPipelinesLoading?: boolean;
  syncEnabled?: boolean;
  defaultLeadrulaPipelineId?: number;
}) {
  const { data: pipelines } = usePipelines();

  function addRow() {
    onChange([
      ...entries,
      {
        leadrula_pipeline_id: defaultLeadrulaPipelineId ?? 0,
        leadrula_stage_id: 0,
        crm_pipeline_id: "",
        crm_stage_id: "",
      },
    ]);
  }

  function removeRow(idx: number) {
    onChange(entries.filter((_, i) => i !== idx));
  }

  function updateRow(idx: number, patch: Partial<CRMPipelineStageMapEntry>) {
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
        {syncEnabled
          ? `Map each Leadrula stage to its ${providerLabel} counterpart for inbound stage sync.`
          : `Map Leadrula stages to ${providerLabel} pipeline stages.`}
      </p>
      {!crmPipelinesLoading && crmPipelines.length === 0 && (
        <p className="text-xs text-gray-400">
          {providerLabel} pipeline and stage IDs can be entered manually if pipelines are not loaded yet.
        </p>
      )}
      {entries.length === 0 ? (
        <p className="text-sm text-gray-400">No mappings yet.</p>
      ) : (
        <Table>
          <THead>
            <tr>
              <TH>Leadrula pipeline</TH>
              <TH>Leadrula stage</TH>
              <TH>{providerLabel} pipeline</TH>
              <TH>{providerLabel} stage</TH>
              <TH className="w-12" />
            </tr>
          </THead>
          <TBody>
            {entries.map((e, idx) => (
              <PipelineStageRow
                key={idx}
                entry={e}
                pipelines={pipelines ?? []}
                crmPipelines={crmPipelines}
                providerLabel={providerLabel}
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
  crmPipelines,
  providerLabel,
  lockLeadrulaPipeline,
  onChange,
  onRemove,
}: {
  entry: CRMPipelineStageMapEntry;
  pipelines: { id: number; name: string }[];
  crmPipelines: CrmPipelineOption[];
  providerLabel: string;
  lockLeadrulaPipeline?: number;
  onChange: (patch: Partial<CRMPipelineStageMapEntry>) => void;
  onRemove: () => void;
}) {
  const lrPipelineLocked = lockLeadrulaPipeline != null && lockLeadrulaPipeline > 0;
  const lrPipelineID = lrPipelineLocked ? lockLeadrulaPipeline! : entry.leadrula_pipeline_id;
  const { data: stages } = useStages(lrPipelineID || undefined);
  const crmPipelineId = entryCrmPipelineId(entry);
  const crmPipeline = crmPipelines.find((p) => p.id === crmPipelineId);
  const crmStages = crmPipeline?.stages ?? [];

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
      <TD>
        {crmPipelines.length > 0 ? (
          <Select
            className="!h-8 !text-sm"
            value={crmPipelineId}
            onChange={(ev) => {
              onChange({ crm_pipeline_id: ev.target.value, crm_stage_id: "" });
            }}
          >
            <option value="">Select…</option>
            {crmPipelines.map((p) => (
              <option key={p.id} value={p.id}>
                {p.name}
              </option>
            ))}
          </Select>
        ) : (
          <Input
            className="!h-8 !text-sm font-mono"
            value={entry.crm_pipeline_id ?? ""}
            onChange={(ev) => onChange({ crm_pipeline_id: ev.target.value, crm_stage_id: "" })}
            placeholder={`${providerLabel} pipeline ID`}
          />
        )}
      </TD>
      <TD>
        {crmStages.length > 0 ? (
          <Select
            className="!h-8 !text-sm"
            value={entryCrmStageId(entry)}
            onChange={(ev) => onChange({ crm_stage_id: ev.target.value })}
          >
            <option value="">Select…</option>
            {crmStages.map((s) => (
              <option key={s.id} value={s.id}>
                {s.name}
              </option>
            ))}
          </Select>
        ) : (
          <Input
            className="!h-8 !text-sm font-mono"
            value={entry.crm_stage_id ?? ""}
            onChange={(ev) => onChange({ crm_stage_id: ev.target.value })}
            placeholder={`${providerLabel} stage ID`}
          />
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
