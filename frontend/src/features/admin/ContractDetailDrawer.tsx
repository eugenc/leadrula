import { useEffect, useState } from "react";
import { Sheet, DrawerHeader, DrawerBody, DrawerFooter } from "@/components/ui/dialog";
import { Button } from "@/components/ui/button";
import { Input, Label, Select } from "@/components/ui/input";
import { SectionLabel } from "@/components/layout/SectionLabel";
import { toast } from "@/store/toastStore";
import { apiError } from "@/lib/api";
import { formatMoney } from "@/lib/utils";
import {
  useUpdateContract,
  useBuyerPipelines,
  useReturnRules,
  useAddReturnRule,
  useDeleteReturnRule,
  useBuyerStages,
} from "@/features/admin/hooks";
import { usePipelines, useStages } from "@/features/leads/hooks";
import { CONTRACT_STATUSES, ContractStatusBadge } from "@/features/admin/contractStatus";
import { ContractReturnRulesEditor } from "@/features/admin/ContractReturnRulesEditor";
import type { Contract } from "@/types";

export function ContractDetailDrawer({
  contract,
  onClose,
}: {
  contract: Contract | null;
  onClose: () => void;
}) {
  return (
    <Sheet open={!!contract} onClose={onClose} width={520}>
      {contract && <DrawerContent contract={contract} onClose={onClose} />}
    </Sheet>
  );
}

function DrawerContent({ contract, onClose }: { contract: Contract; onClose: () => void }) {
  const update = useUpdateContract();
  const { data: pubPipelines } = usePipelines();
  const { data: sourceStages } = useStages(contract.source_pipeline_id || undefined);
  const { data: buyerPipelines } = useBuyerPipelines(contract.buyer_id || null);
  const { data: buyerStages, isLoading: stagesLoading } = useBuyerStages(
    contract.buyer_id,
    contract.buyer_pipeline_id
  );
  const { data: rules, isLoading: rulesLoading } = useReturnRules(contract.id);
  const addRule = useAddReturnRule(false);
  const removeRule = useDeleteReturnRule(false);

  const [name, setName] = useState(contract.name);
  const [rate, setRate] = useState(contract.rate_per_lead);
  const [status, setStatus] = useState(contract.status);

  useEffect(() => {
    setName(contract.name);
    setRate(contract.rate_per_lead);
    setStatus(contract.status);
  }, [contract]);

  const sourcePipeline = (pubPipelines ?? []).find((p) => p.id === contract.source_pipeline_id);
  const sourceStage = (sourceStages ?? []).find((s) => s.id === contract.source_stage_id);
  const returnStage = (sourceStages ?? []).find((s) => s.id === contract.return_stage_id);
  const buyerPipeline = (buyerPipelines ?? []).find((p) => p.id === contract.buyer_pipeline_id);
  const ruleByStage = new Map((rules ?? []).map((r) => [r.buyer_stage_id, r.id]));

  const unchanged =
    name.trim() === contract.name && rate === contract.rate_per_lead && status === contract.status;
  const invalid = !name.trim() || rate < 0;

  function save() {
    const body: Record<string, unknown> = {};
    const trimmed = name.trim();
    if (trimmed !== contract.name) body.name = trimmed;
    if (rate !== contract.rate_per_lead) body.rate_per_lead = rate;
    if (status !== contract.status) body.status = status;
    if (Object.keys(body).length === 0) return;

    update.mutate(
      { id: contract.id, body },
      {
        onSuccess: () => {
          toast.success("Contract saved");
          onClose();
        },
        onError: (e) => toast.error(apiError(e).message),
      }
    );
  }

  function toggleReturnRule(stageId: number, on: boolean) {
    if (on) {
      addRule.mutate(
        { contractId: contract.id, buyerStageId: stageId },
        { onError: (e) => toast.error(apiError(e).message) }
      );
    } else {
      const ruleId = ruleByStage.get(stageId);
      if (ruleId) removeRule.mutate(ruleId, { onError: (e) => toast.error(apiError(e).message) });
    }
  }

  return (
    <div className="flex h-full flex-col">
      <DrawerHeader
        title={contract.name}
        subtitle={`${contract.buyer_name ?? `Buyer #${contract.buyer_id}`} · ${formatMoney(contract.rate_per_lead)}/lead`}
        onClose={onClose}
      />

      <DrawerBody>
        <div className="flex flex-col gap-2.5">
          <div>
            <Label>Name</Label>
            <Input value={name} onChange={(e) => setName(e.target.value)} />
          </div>
          <div>
            <Label>Rate per lead (USD)</Label>
            <Input type="number" min={0} step={0.01} value={rate} onChange={(e) => setRate(Number(e.target.value))} />
          </div>
          <div>
            <Label>Status</Label>
            <Select value={status} onChange={(e) => setStatus(e.target.value)}>
              {CONTRACT_STATUSES.map((s) => (
                <option key={s.value} value={s.value}>
                  {s.label}
                </option>
              ))}
            </Select>
            <div className="mt-2">
              <ContractStatusBadge status={status} />
            </div>
          </div>

          <div className="pt-2">
            <SectionLabel className="mb-2">Routing (read-only)</SectionLabel>
            <div className="flex flex-col gap-3 text-sm">
              <div>
                <div className="text-xs font-semibold uppercase tracking-wide text-gray-400">Buyer</div>
                <div className="mt-1 text-gray-700">{contract.buyer_name ?? `Buyer #${contract.buyer_id}`}</div>
              </div>
              <div>
                <div className="text-xs font-semibold uppercase tracking-wide text-gray-400">Source pipeline</div>
                <div className="mt-1 text-gray-700">{sourcePipeline?.name ?? `#${contract.source_pipeline_id}`}</div>
              </div>
              <div>
                <div className="text-xs font-semibold uppercase tracking-wide text-gray-400">Source stage</div>
                <div className="mt-1 text-gray-700">{sourceStage?.name ?? `#${contract.source_stage_id}`}</div>
              </div>
              <div>
                <div className="text-xs font-semibold uppercase tracking-wide text-gray-400">Buyer pipeline</div>
                <div className="mt-1 text-gray-700">{buyerPipeline?.name ?? `#${contract.buyer_pipeline_id}`}</div>
              </div>
              <div>
                <div className="text-xs font-semibold uppercase tracking-wide text-gray-400">Return stage</div>
                <div className="mt-1 text-gray-700">{returnStage?.name ?? `#${contract.return_stage_id}`}</div>
              </div>
            </div>
          </div>

          <div className="pt-2">
            <SectionLabel className="mb-2">Return Rules</SectionLabel>
            <ContractReturnRulesEditor
              stages={buyerStages ?? []}
              rules={rules ?? []}
              loading={stagesLoading || rulesLoading}
              onToggle={toggleReturnRule}
            />
          </div>
        </div>
      </DrawerBody>

      <DrawerFooter>
        <Button disabled={unchanged || invalid || update.isPending} onClick={save}>
          Save
        </Button>
      </DrawerFooter>
    </div>
  );
}
