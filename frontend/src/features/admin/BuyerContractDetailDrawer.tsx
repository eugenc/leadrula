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
  const { data: buyerStages, isLoading: buyerStagesLoading } = useStages(contract.buyer_pipeline_id);
  const { data: publisherStages, isLoading: pubStagesLoading } = useContractPublisherStages(
    contract.id,
    true
  );
  const { data: rules, isLoading: rulesLoading } = useReturnRules(contract.id, true);
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

        <div className="mb-4 flex flex-col gap-3 text-sm">
          <div>
            <div className="text-xs font-semibold uppercase tracking-wide text-gray-400">Lead Type</div>
            <div className="mt-1 text-gray-700">{contract.lead_type || "—"}</div>
          </div>
          <div>
            <div className="text-xs font-semibold uppercase tracking-wide text-gray-400">Description</div>
            <div className="mt-1 whitespace-pre-wrap text-gray-700">{contract.description || "—"}</div>
          </div>
        </div>

        <div className="pt-2">
          <SectionLabel className="mb-2">Return Rules</SectionLabel>
          <ContractReturnRulesEditor
            buyerStages={buyerStages ?? []}
            publisherStages={publisherStages ?? []}
            rules={rules ?? []}
            defaultReturnStageId={contract.return_stage_id}
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
