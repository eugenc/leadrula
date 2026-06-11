import { Label, Select } from "@/components/ui/input";
import { SectionLabel } from "@/components/layout/SectionLabel";
import {
  useParticipationCompensations,
  useUpdateBuyerCompensationTriggerStage,
  useUpdateParticipationCompensationTriggerStage,
  useContractCompensations,
  useContractPublisherStages,
} from "@/features/admin/hooks";
import { useStages } from "@/features/leads/hooks";
import { toast } from "@/store/toastStore";
import { errorMessage } from "@/lib/api";
import { COMPENSATION_KINDS, formatCompTrigger } from "@/features/admin/contractCompensation";
import type { ContractCompensation } from "@/types";

function TriggerRow({
  comp,
  stages,
  onSave,
  pending,
}: {
  comp: ContractCompensation;
  stages: { id: number; name: string }[];
  onSave: (stageId: number) => void;
  pending: boolean;
}) {
  return (
    <div className="rounded border border-gray-100 px-3 py-2 text-sm">
      <div className="font-semibold text-gray-800">
        {COMPENSATION_KINDS.find((k) => k.value === comp.kind)?.label ?? comp.kind}
      </div>
      <div className="mb-2 text-gray-500">{formatCompTrigger(comp.trigger)}</div>
      <Label>Trigger stage</Label>
      <Select
        value={comp.trigger_stage_id ?? 0}
        onChange={(e) => {
          const id = Number(e.target.value);
          if (id) onSave(id);
        }}
        disabled={pending}
      >
        <option value={0}>{comp.trigger_stage_id ? "Change…" : "Select…"}</option>
        {stages.map((s) => (
          <option key={s.id} value={s.id}>
            {s.name}
          </option>
        ))}
      </Select>
    </div>
  );
}

export function BuyerTriggerStageFields({
  contractId,
  participationId,
  buyerPipelineId,
}: {
  contractId?: number;
  participationId?: number;
  buyerPipelineId?: number | null;
}) {
  const isParticipation = participationId != null;
  const { data: contractComps } = useContractCompensations(!isParticipation ? contractId ?? null : null, true);
  const { data: partComps } = useParticipationCompensations(isParticipation ? participationId : null);
  const updateContract = useUpdateBuyerCompensationTriggerStage();
  const updateParticipation = useUpdateParticipationCompensationTriggerStage();

  const { data: buyerStages } = useStages(buyerPipelineId || undefined);
  const { data: publisherStages } = useContractPublisherStages(
    !isParticipation && contractId ? contractId : undefined,
    true
  );

  const comps = (isParticipation ? partComps : contractComps) ?? [];
  const stageRows = (buyerStages ?? publisherStages ?? []).filter(
    (s) => !("stage_type" in s) || s.stage_type === "standard" || s.stage_type === "action"
  );

  const triggerComps = comps.filter(
    (c) => (c.kind === "rev_share" || c.kind === "profit_share") && c.trigger === "buyer_stage"
  );

  if (triggerComps.length === 0) return null;

  function save(compId: number, stageId: number) {
    const onError = (e: unknown) => toast.error(errorMessage(e));
    if (isParticipation && participationId) {
      updateParticipation.mutate({ participationId, compId, triggerStageId: stageId }, { onError });
    } else if (contractId) {
      updateContract.mutate({ contractId, compId, triggerStageId: stageId }, { onError });
    }
  }

  return (
    <div className="space-y-2">
      <SectionLabel>Rev / profit share triggers</SectionLabel>
      <p className="text-xs text-gray-400">Choose which pipeline stage triggers payout (optional).</p>
      {triggerComps.map((c) => (
        <TriggerRow
          key={c.id}
          comp={c}
          stages={stageRows}
          pending={updateContract.isPending || updateParticipation.isPending}
          onSave={(stageId) => save(c.id, stageId)}
        />
      ))}
    </div>
  );
}
