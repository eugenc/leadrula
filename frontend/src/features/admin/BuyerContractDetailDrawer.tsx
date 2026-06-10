import { Sheet, DrawerHeader, DrawerBody } from "@/components/ui/dialog";
import { Button } from "@/components/ui/button";
import { SectionLabel } from "@/components/layout/SectionLabel";
import { toast } from "@/store/toastStore";
import { errorMessage } from "@/lib/api";
import { formatMoney } from "@/lib/utils";
import {
  useReturnRules,
  useAddReturnRule,
  useUpdateReturnRule,
  useDeleteReturnRule,
  useContractPublisherStages,
} from "@/features/admin/hooks";
import { useStages } from "@/features/leads/hooks";
import { ContractStatusBadge } from "@/features/admin/contractStatus";
import { formatCapPeriod, formatContractCap } from "@/features/admin/contractCap";
import { formatContractLeadType } from "@/features/admin/contractLeadType";
import { COMPENSATION_KINDS, formatCompTrigger } from "@/features/admin/contractCompensation";
import { useContractCompensations } from "@/features/admin/hooks";
import { ContractReturnRulesEditor } from "@/features/admin/ContractReturnRulesEditor";
import type { Contract } from "@/types";

export function BuyerContractDetailDrawer({
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
  const { data: buyerStages, isLoading: buyerStagesLoading } = useStages(contract.buyer_pipeline_id ?? undefined);
  const { data: publisherStages, isLoading: pubStagesLoading } = useContractPublisherStages(
    contract.id,
    true
  );
  const { data: rules, isLoading: rulesLoading } = useReturnRules(contract.id, true);
  const { data: compensations, isLoading: compsLoading } = useContractCompensations(contract.id, true);
  const addRule = useAddReturnRule(true);
  const updateRule = useUpdateReturnRule(true);
  const removeRule = useDeleteReturnRule(true);

  const loading = buyerStagesLoading || pubStagesLoading || rulesLoading;

  return (
    <div className="flex h-full flex-col">
      <DrawerHeader
        title={contract.name}
        subtitle={`${contract.publisher_name ?? "Publisher"} · ${formatMoney(contract.rate_per_lead)}/lead · ${contract.lead_count ?? 0} received`}
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

        <div className="mb-4 flex items-center justify-between">
          <ContractStatusBadge status={contract.status} />
          <div className="text-right">
            <div className="text-xs font-semibold uppercase tracking-wide text-gray-400">
              Rate per lead
            </div>
            <div className="text-2xl font-bold text-gray-800">{formatMoney(contract.rate_per_lead)}</div>
          </div>
        </div>

        <SectionLabel className="mb-2">Contract Details</SectionLabel>
        <div className="mb-4 flex flex-col gap-3 text-sm">
          <div>
            <div className="text-xs font-semibold uppercase tracking-wide text-gray-400">Type</div>
            <div className="mt-1 text-gray-700">Buy</div>
          </div>
          <div>
            <div className="text-xs font-semibold uppercase tracking-wide text-gray-400">Publisher</div>
            <div className="mt-1 text-gray-700">{contract.publisher_name ?? "—"}</div>
          </div>
          <div>
            <div className="text-xs font-semibold uppercase tracking-wide text-gray-400">Lead Type</div>
            <div className="mt-1 text-gray-700">{formatContractLeadType(contract.lead_type) || "—"}</div>
          </div>
          <div>
            <div className="text-xs font-semibold uppercase tracking-wide text-gray-400">Cap limits</div>
            <div className="mt-1 text-gray-700">{formatContractCap(contract)}</div>
          </div>
          <div>
            <div className="text-xs font-semibold uppercase tracking-wide text-gray-400">Description</div>
            <div className="mt-1 whitespace-pre-wrap text-gray-700">{contract.description || "—"}</div>
          </div>
        </div>

        <SectionLabel className="mb-2">Compensation</SectionLabel>
        <div className="mb-4 flex flex-col gap-2">
          {compsLoading ? (
            <p className="text-sm text-gray-400">Loading…</p>
          ) : (compensations ?? []).length === 0 ? (
            <p className="text-sm text-gray-400">—</p>
          ) : (
            (compensations ?? []).map((c) => (
              <div key={c.id} className="rounded border border-gray-100 px-3 py-2 text-sm">
                <div className="font-semibold text-gray-800">
                  {COMPENSATION_KINDS.find((k) => k.value === c.kind)?.label ?? c.kind}
                </div>
                <div className="text-gray-500">
                  {formatCompTrigger(c.trigger)} · {formatCapPeriod(c.cap_period)}
                  {c.flat_amount != null ? ` · ${formatMoney(c.flat_amount)}/lead` : ""}
                </div>
              </div>
            ))
          )}
        </div>

        <div className="pt-2">
          <SectionLabel className="mb-2">Return Rules</SectionLabel>
          <ContractReturnRulesEditor
            buyerStages={buyerStages ?? []}
            publisherStages={publisherStages ?? []}
            rules={rules ?? []}
            defaultReturnStageId={contract.return_stage_id ?? 0}
            loading={loading}
            description="When a lead enters the From Stage on your pipeline, it is automatically returned to the publisher (no charge to you on return)."
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
        </div>
      </DrawerBody>
    </div>
  );
}
