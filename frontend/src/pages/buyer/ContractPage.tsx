import {
  useMyContract,
  useReturnRules,
  useAddReturnRule,
  useDeleteReturnRule,
} from "@/features/admin/hooks";
import { useStages } from "@/features/leads/hooks";
import { PageBody } from "@/components/layout/PageBody";
import { Card, Spinner } from "@/components/ui/misc";
import { formatMoney } from "@/lib/utils";
import { ContractStatusBadge } from "@/features/admin/contractStatus";
import { ContractReturnRulesEditor } from "@/features/admin/ContractReturnRulesEditor";
import { toast } from "@/store/toastStore";
import { apiError } from "@/lib/api";

export function ContractPage() {
  const { data: contract, isLoading } = useMyContract();
  const { data: stages, isLoading: stagesLoading } = useStages(contract?.buyer_pipeline_id);
  const { data: rules, isLoading: rulesLoading } = useReturnRules(null, true);
  const add = useAddReturnRule(true);
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

  const ruleByStage = new Map((rules ?? []).map((r) => [r.buyer_stage_id, r.id]));

  function toggle(stageId: number, on: boolean) {
    if (on) {
      add.mutate(
        { contractId: null, buyerStageId: stageId },
        { onError: (e) => toast.error(apiError(e).message) }
      );
    } else {
      const ruleId = ruleByStage.get(stageId);
      if (ruleId) remove.mutate(ruleId, { onError: (e) => toast.error(apiError(e).message) });
    }
  }

  return (
    <PageBody className="max-w-2xl space-y-5">
      <Card className="p-5">
        <div className="flex items-center justify-between">
          <div>
            <div className="text-lg font-semibold text-gray-800">{contract.name}</div>
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
          stages={stages ?? []}
          rules={rules ?? []}
          loading={stagesLoading || rulesLoading}
          onToggle={toggle}
          description="When a lead enters one of these stages, it is automatically returned to the publisher (no charge to you on return)."
        />
      </Card>
    </PageBody>
  );
}
