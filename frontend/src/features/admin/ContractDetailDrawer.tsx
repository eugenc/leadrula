import { useEffect, useState } from "react";
import { Sheet, DrawerHeader, DrawerBody, DrawerFooter } from "@/components/ui/dialog";
import { Button } from "@/components/ui/button";
import { Input, Label, Select, Textarea } from "@/components/ui/input";
import { SectionLabel } from "@/components/layout/SectionLabel";
import { toast } from "@/store/toastStore";
import { errorMessage } from "@/lib/api";
import { formatMoney } from "@/lib/utils";
import {
  useUpdateContract,
  useReturnRules,
  useAddReturnRule,
  useUpdateReturnRule,
  useDeleteReturnRule,
  useBuyerStages,
  useContractCompensations,
  useContractLeadCriteria,
  useSaveContractLeadCriteria,
} from "@/features/admin/hooks";
import {
  ContractLeadCriteriaSection,
  emptyLeadCriteria,
} from "@/features/admin/ContractLeadCriteriaSection";
import { useStages } from "@/features/leads/hooks";
import { CONTRACT_STATUSES, ContractStatusBadge } from "@/features/admin/contractStatus";
import { CONTRACT_LEAD_TYPES, isContractLeadType } from "@/features/admin/contractLeadType";
import { counterpartyLabel, formatBuyerWithType, formatContractType } from "@/features/admin/contractType";
import { ContractCompensationEditor } from "@/features/admin/ContractCompensationEditor";
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
    <Sheet open={!!contract} onClose={onClose} width={720}>
      {contract && <DrawerContent contract={contract} onClose={onClose} />}
    </Sheet>
  );
}

function DrawerContent({ contract, onClose }: { contract: Contract; onClose: () => void }) {
  const update = useUpdateContract();
  const { data: compensations, isLoading: compsLoading } = useContractCompensations(contract.id);
  const { data: sourceStages } = useStages(contract.source_pipeline_id || undefined);
  const { data: buyerStages, isLoading: stagesLoading } = useBuyerStages(
    contract.buyer_id,
    contract.buyer_pipeline_id
  );
  const { data: rules, isLoading: rulesLoading } = useReturnRules(contract.id);
  const addRule = useAddReturnRule(false);
  const updateRule = useUpdateReturnRule(false);
  const removeRule = useDeleteReturnRule(false);

  const [name, setName] = useState(contract.name);
  const [leadType, setLeadType] = useState(contract.lead_type ?? "");
  const [description, setDescription] = useState(contract.description ?? "");
  const [status, setStatus] = useState(contract.status);
  const { data: leadCriteriaData } = useContractLeadCriteria(contract.id);
  const saveCriteria = useSaveContractLeadCriteria();
  const [leadCriteria, setLeadCriteria] = useState(emptyLeadCriteria());

  useEffect(() => {
    if (leadCriteriaData) setLeadCriteria(leadCriteriaData);
  }, [leadCriteriaData]);

  useEffect(() => {
    setName(contract.name);
    setLeadType(contract.lead_type ?? "");
    setDescription(contract.description ?? "");
    setStatus(contract.status);
  }, [contract]);

  const unchanged =
    name.trim() === contract.name &&
    leadType === (contract.lead_type ?? "") &&
    description === (contract.description ?? "") &&
    status === contract.status;
  const invalid = !name.trim() || !isContractLeadType(leadType);

  function saveDetails() {
    const body: Record<string, unknown> = {};
    const trimmed = name.trim();
    if (trimmed !== contract.name) body.name = trimmed;
    if (leadType !== (contract.lead_type ?? "")) body.lead_type = leadType;
    if (description !== (contract.description ?? "")) body.description = description;
    if (status !== contract.status) body.status = status;
    if (Object.keys(body).length === 0) return;

    update.mutate(
      { id: contract.id, body },
      {
        onSuccess: () => toast.success("Contract saved"),
        onError: (e) => toast.error(errorMessage(e)),
      }
    );
  }

  const primaryRate =
    compensations?.find((c) => c.kind === "flat_rate")?.flat_amount ?? contract.rate_per_lead;

  return (
    <div className="flex h-full flex-col">
      <DrawerHeader
        title={contract.name}
        subtitle={`${formatContractType(contract.contract_type)} · ${formatBuyerWithType(contract.buyer_name, contract.buyer_account_type) || `Buyer #${contract.buyer_id}`} · ${formatMoney(primaryRate)}/lead · ${contract.lead_count ?? 0} distributed`}
        onClose={onClose}
      />

      <DrawerBody>
        <div className="mb-4 flex items-center justify-between rounded-lg border border-gray-100 bg-gray-50 px-3 py-2.5">
          <span className="text-sm text-gray-400">Contract ID</span>
          <div className="flex items-center gap-2">
            <code className="text-sm font-semibold text-gray-800">{contract.handler_id}</code>
            <Button
              variant="secondary"
              className="h-7 px-2 text-xs"
              onClick={() => {
                void navigator.clipboard
                  .writeText(contract.handler_id)
                  .then(() => toast.success("Copied Contract ID"));
              }}
            >
              Copy
            </Button>
          </div>
        </div>
        {contract.mirror_contract_id != null && (
          <p className="mb-3 text-xs text-gray-500">
            Mirrored with publisher contract #{contract.mirror_contract_id}. Edits on shared fields sync on save from the sell side.
          </p>
        )}

        <SectionLabel className="mb-2">Contract Details</SectionLabel>
        <div className="mb-6 flex flex-col gap-2.5">
          <div>
            <Label>Contract type</Label>
            <div className="mt-1 text-sm text-gray-700">{formatContractType(contract.contract_type) || "Sell"}</div>
          </div>
          <div>
            <Label>{counterpartyLabel(contract.contract_type)}</Label>
            <div className="mt-1 text-sm text-gray-700">
              {formatBuyerWithType(contract.buyer_name, contract.buyer_account_type) || `#${contract.buyer_id}`}
            </div>
          </div>
          <div>
            <Label>Name</Label>
            <Input value={name} onChange={(e) => setName(e.target.value)} />
          </div>
          <div>
            <Label>Lead type</Label>
            <Select value={leadType} onChange={(e) => setLeadType(e.target.value)}>
              <option value="">Select…</option>
              {CONTRACT_LEAD_TYPES.map((t) => (
                <option key={t.value} value={t.value}>
                  {t.label}
                </option>
              ))}
            </Select>
          </div>
          <div>
            <Label>Description</Label>
            <Textarea value={description} onChange={(e) => setDescription(e.target.value)} />
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
        </div>

        <SectionLabel className="mb-2">Compensation</SectionLabel>
        <p className="mb-2 text-xs text-gray-400">Each row has its own cap limits and delivery settings below.</p>
        <div className="mb-6">
          {compsLoading ? (
            <p className="text-sm text-gray-400">Loading…</p>
          ) : (
            <ContractCompensationEditor contract={contract} items={compensations ?? []} />
          )}
        </div>

        <div className="mb-6">
          <ContractLeadCriteriaSection
            buyerId={contract.buyer_id}
            buyerPipelineId={contract.buyer_pipeline_id}
            value={leadCriteria}
            onChange={setLeadCriteria}
          />
          <Button
            className="mt-3"
            variant="secondary"
            disabled={saveCriteria.isPending}
            onClick={() =>
              saveCriteria.mutate(
                { contractId: contract.id, body: leadCriteria },
                {
                  onSuccess: () => toast.success("Lead criteria saved"),
                  onError: (e) => toast.error(errorMessage(e)),
                }
              )
            }
          >
            Save lead criteria
          </Button>
        </div>

        <SectionLabel className="mb-2">Return Rules</SectionLabel>
        <ContractReturnRulesEditor
          buyerStages={buyerStages ?? []}
          publisherStages={sourceStages ?? []}
          rules={rules ?? []}
          defaultReturnStageId={contract.return_stage_id}
          loading={stagesLoading || rulesLoading}
          onAdd={(buyerStageId, returnStageId) =>
            addRule.mutate(
              { contractId: contract.id, buyerStageId, returnStageId },
              { onError: (e) => toast.error(errorMessage(e)) }
            )
          }
          onUpdate={(ruleId, buyerStageId, returnStageId) =>
            updateRule.mutate(
              { contractId: contract.id, ruleId, buyerStageId, returnStageId },
              { onError: (e) => toast.error(errorMessage(e)) }
            )
          }
          onDelete={(ruleId) =>
            removeRule.mutate(
              { contractId: contract.id, ruleId },
              { onError: (e) => toast.error(errorMessage(e)) }
            )
          }
        />
      </DrawerBody>

      <DrawerFooter>
        <Button disabled={unchanged || invalid || update.isPending} onClick={saveDetails}>
          Save details
        </Button>
      </DrawerFooter>
    </div>
  );
}
