import { useEffect, useMemo } from "react";
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
import type { ContractCompensation, Stage } from "@/types";

function TriggerRow({
  comp,
  wonStage,
  onSave,
  pending,
}: {
  comp: ContractCompensation;
  wonStage: Stage | undefined;
  onSave: (stageId: number) => void;
  pending: boolean;
}) {
  const selected = wonStage && comp.trigger_stage_id === wonStage.id;

  return (
    <div className="rounded border border-gray-100 px-3 py-2 text-sm">
      <div className="font-semibold text-gray-800">
        {COMPENSATION_KINDS.find((k) => k.value === comp.kind)?.label ?? comp.kind}
      </div>
      <div className="mb-2 text-gray-500">{formatCompTrigger(comp.trigger)}</div>
      <Label>Trigger stage</Label>
      {!wonStage ? (
        <p className="text-xs text-amber-700">Add a Won stage to your pipeline to enable payout.</p>
      ) : (
        <Select
          value={selected ? wonStage.id : 0}
          onChange={(e) => {
            const id = Number(e.target.value);
            if (id) onSave(id);
          }}
          disabled={pending || selected}
        >
          <option value={0}>{selected ? wonStage.name : "Select…"}</option>
          <option value={wonStage.id}>{wonStage.name}</option>
        </Select>
      )}
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

  const comps = useMemo(
    () => (isParticipation ? partComps : contractComps) ?? [],
    [isParticipation, partComps, contractComps]
  );
  const allStages = buyerStages ?? publisherStages ?? [];
  const wonStage = allStages.find((s) => s.stage_type === "won");

  const triggerComps = useMemo(
    () =>
      comps.filter(
        (c) => (c.kind === "rev_share" || c.kind === "profit_share") && c.trigger === "buyer_stage"
      ),
    [comps]
  );
  const unsetTriggerKey = triggerComps
    .filter((c) => c.trigger_stage_id !== wonStage?.id)
    .map((c) => c.id)
    .join(",");

  const pending = updateContract.isPending || updateParticipation.isPending;

  function save(compId: number, stageId: number) {
    const onError = (e: unknown) => toast.error(errorMessage(e));
    if (isParticipation && participationId) {
      updateParticipation.mutate({ participationId, compId, triggerStageId: stageId }, { onError });
    } else if (contractId) {
      updateContract.mutate({ contractId, compId, triggerStageId: stageId }, { onError });
    }
  }

  useEffect(() => {
    if (!wonStage || pending || !unsetTriggerKey) return;
    const onError = (e: unknown) => toast.error(errorMessage(e));
    for (const c of triggerComps) {
      if (c.trigger_stage_id === wonStage.id) continue;
      if (isParticipation && participationId) {
        updateParticipation.mutate(
          { participationId, compId: c.id, triggerStageId: wonStage.id },
          { onError }
        );
      } else if (contractId) {
        updateContract.mutate({ contractId, compId: c.id, triggerStageId: wonStage.id }, { onError });
      }
    }
  }, [
    buyerPipelineId,
    wonStage,
    pending,
    unsetTriggerKey,
    isParticipation,
    participationId,
    contractId,
    updateContract,
    updateParticipation,
    triggerComps,
  ]);

  if (triggerComps.length === 0) return null;

  return (
    <div className="space-y-2">
      <SectionLabel>Rev / profit share triggers</SectionLabel>
      <p className="text-xs text-gray-400">Payout triggers when a lead reaches Won on your pipeline.</p>
      {triggerComps.map((c) => (
        <TriggerRow
          key={c.id}
          comp={c}
          wonStage={wonStage}
          pending={pending}
          onSave={(stageId) => save(c.id, stageId)}
        />
      ))}
    </div>
  );
}
