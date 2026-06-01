import {
  useMyContract,
  useReturnRules,
  useAddReturnRule,
  useUpdateReturnRule,
  useDeleteReturnRule,
  useContractPublisherStages,
} from "@/features/admin/hooks";
import { useStages } from "@/features/leads/hooks";
import { PageBody } from "@/components/layout/PageBody";
import { Card, Spinner } from "@/components/ui/misc";
import { formatMoney } from "@/lib/utils";
import { ContractStatusBadge } from "@/features/admin/contractStatus";
import { ContractReturnRulesEditor } from "@/features/admin/ContractReturnRulesEditor";
import { toast } from "@/store/toastStore";
import { errorMessage } from "@/lib/api";

export function ContractPage() {
  const { data: contract, isLoading } = useMyContract();
  const { data: buyerStages, isLoading: buyerStagesLoading } = useStages(contract?.buyer_pipeline_id);
  const { data: publisherStages, isLoading: pubStagesLoading } = useContractPublisherStages(true);
  const { data: rules, isLoading: rulesLoading } = useReturnRules(null, true);
  const add = useAddReturnRule(true);
  const update = useUpdateReturnRule(true);
  const remove = useDeleteReturnRule(true);

  if (isLoading) {
    return (
      <PageBody>
        <Spinner className="h-6 w-6" />
      </PageBody>
    );
  }

  if (!contract) {
    return (
      <PageBody>
        <p className="text-sm text-gray-400">No contract assigned yet.</p>
      </PageBody>
    );
  }

  const loading = buyerStagesLoading || pubStagesLoading || rulesLoading;

  return (
    <PageBody className="max-w-2xl space-y-5">
      <Card className="p-5">
        <div className="flex items-center justify-between">
          <div>
            <div className="text-lg font-semibold text-gray-800">{contract.name}</div>
            <div className="mt-1 font-mono text-xs text-gray-400">{contract.handler_id}</div>
            <ContractStatusBadge status={contract.status} />
          </div>
          <div className="text-right">
            <div className="text-xs font-semibold uppercase tracking-wide text-gray-400">
              Rate per lead
            </div>
            <div className="text-2xl font-bold text-gray-800">{formatMoney(contract.rate_per_lead)}</div>
          </div>
        </div>
      </Card>

      <Card className="p-5">
        <div className="mb-1 text-sm font-semibold text-gray-800">Return Rules</div>
        <ContractReturnRulesEditor
          buyerStages={buyerStages ?? []}
          publisherStages={publisherStages ?? []}
          rules={rules ?? []}
          defaultReturnStageId={contract.return_stage_id}
          loading={loading}
          onAdd={(buyerStageId, returnStageId) =>
            add.mutate(
              { contractId: null, buyerStageId, returnStageId },
              { onError: (e) => toast.error(errorMessage(e)) }
            )
          }
          onUpdate={(ruleId, buyerStageId, returnStageId) =>
            update.mutate(
              { ruleId, buyerStageId, returnStageId },
              { onError: (e) => toast.error(errorMessage(e)) }
            )
          }
          onDelete={(ruleId) =>
            remove.mutate(ruleId, { onError: (e) => toast.error(errorMessage(e)) })
          }
          description="When a lead enters the From Stage on your pipeline, it is automatically returned to the publisher (no charge to you on return)."
        />
      </Card>
    </PageBody>
  );
}
