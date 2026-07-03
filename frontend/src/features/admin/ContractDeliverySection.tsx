import { Label, Select } from "@/components/ui/input";
import { SectionLabel } from "@/components/layout/SectionLabel";
import {
  COMPENSATION_DELIVERY,
  pipelineDraftWithoutLeads,
  type ContractDeliveryDraft,
} from "@/features/admin/contractCompensation";
import { usePipelines, useStages } from "@/features/leads/hooks";

export function ContractDeliverySection({
  value,
  onChange,
  directContract = false,
}: {
  value: ContractDeliveryDraft;
  onChange: (v: ContractDeliveryDraft) => void;
  directContract?: boolean;
}) {
  const { data: pubPipelines } = usePipelines();
  const { data: sourceStages } = useStages(value.source_pipeline_id || undefined);

  function set<K extends keyof ContractDeliveryDraft>(k: K, v: ContractDeliveryDraft[K]) {
    onChange({ ...value, [k]: v });
  }

  return (
    <div className="flex flex-col gap-2.5">
      <SectionLabel>Delivery</SectionLabel>
      <p className="text-xs text-gray-400">Default for this contract. Routes can override Lead vs Pipeline.</p>
      <div>
        <Label>Delivery mode</Label>
        <Select
          value={value.delivery}
          onChange={(e) => onChange(pipelineDraftWithoutLeads({ ...value, delivery: e.target.value }))}
        >
          {COMPENSATION_DELIVERY.map((d) => (
            <option key={d.value} value={d.value}>
              {d.label}
            </option>
          ))}
        </Select>
      </div>
      {value.delivery === "leads_pipeline" && directContract && (
        <p className="text-xs text-gray-500">
          The buyer chooses their destination pipeline on the contract after it is active.
        </p>
      )}
      {value.delivery === "leads_pipeline" && (
        <div className="grid grid-cols-2 gap-3">
          <div>
            <Label>Source pipeline</Label>
            <Select value={value.source_pipeline_id} onChange={(e) => set("source_pipeline_id", Number(e.target.value))}>
              <option value={0}>Select…</option>
              {(pubPipelines ?? []).map((p) => (
                <option key={p.id} value={p.id}>
                  {p.name}
                </option>
              ))}
            </Select>
          </div>
          <div>
            <Label>Source stage</Label>
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
      )}
    </div>
  );
}
