import { Label, Select } from "@/components/ui/input";
import { SectionLabel } from "@/components/layout/SectionLabel";
import type { ContractDeliveryDraft } from "@/features/admin/contractCompensation";
import { usePipelines, useStages } from "@/features/leads/hooks";

export function ContractPublisherPipelineSection({
  value,
  onChange,
}: {
  value: ContractDeliveryDraft;
  onChange: (v: ContractDeliveryDraft) => void;
}) {
  const { data: pubPipelines } = usePipelines();
  const { data: sourceStages } = useStages(value.source_pipeline_id || undefined);

  function set<K extends keyof ContractDeliveryDraft>(k: K, v: ContractDeliveryDraft[K]) {
    onChange({ ...value, [k]: v });
  }

  return (
    <div className="flex flex-col gap-2.5">
      <SectionLabel>Publisher pipeline</SectionLabel>
      <p className="text-xs text-gray-400">
        Required when pipeline delivery is allowed. Leads distribute from the source stage; return destinations are
        mapped per route below.
      </p>
      <div className="grid grid-cols-2 gap-3">
        <div>
          <Label>Source pipeline</Label>
          <Select
            value={value.source_pipeline_id}
            onChange={(e) => {
              const pipelineId = Number(e.target.value);
              onChange({
                ...value,
                source_pipeline_id: pipelineId,
                source_stage_id: 0,
              });
            }}
          >
            <option value={0}>Select…</option>
            {(pubPipelines ?? []).map((p) => (
              <option key={p.id} value={p.id}>
                {p.name}
              </option>
            ))}
          </Select>
        </div>
        <div>
          <Label>Distribute from stage</Label>
          <Select value={value.source_stage_id} onChange={(e) => set("source_stage_id", Number(e.target.value))}>
            <option value={0}>Select…</option>
            {(sourceStages ?? []).map((s) => (
              <option key={s.id} value={s.id}>
                {s.name}
              </option>
            ))}
          </Select>
        </div>
      </div>
    </div>
  );
}
