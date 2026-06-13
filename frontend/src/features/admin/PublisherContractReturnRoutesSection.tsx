import { toast } from "@/store/toastStore";
import { errorMessage } from "@/lib/api";
import {
  useReturnRules,
  useAddReturnRule,
  useUpdateReturnRule,
  useDeleteReturnRule,
} from "@/features/admin/hooks";
import { useStages } from "@/features/leads/hooks";
import { ContractReturnRulesEditor } from "@/features/admin/ContractReturnRulesEditor";
import { returnRulesRequired } from "@/features/admin/contractSectionCompleteness";
import type { ContractDeliveryDraft } from "@/features/admin/contractCompensation";

type Props = {
  contractId: number;
  delivery: ContractDeliveryDraft;
  openOffer?: boolean;
};

export function PublisherContractReturnRoutesSection({ contractId, delivery, openOffer = false }: Props) {
  const { data: buyerStages, isLoading: buyerLoading } = useStages(
    delivery.counterparty_pipeline_id || undefined
  );
  const { data: publisherStages, isLoading: pubLoading } = useStages(
    delivery.source_pipeline_id || undefined
  );
  const { data: rules, isLoading: rulesLoading } = useReturnRules(contractId, false);
  const addRule = useAddReturnRule(false);
  const updateRule = useUpdateReturnRule(false);
  const removeRule = useDeleteReturnRule(false);

  if (openOffer) {
    return (
      <p className="text-sm text-gray-500">
        Return destination is configured under Delivery. Buyers pick return start stages when they accept.
      </p>
    );
  }

  if (!returnRulesRequired(delivery, false)) {
    return (
      <p className="text-sm text-gray-500">
        Return routes apply when delivery mode is Pipeline. Configure delivery first, or use Leads inbox delivery.
      </p>
    );
  }

  if (!delivery.counterparty_pipeline_id || !delivery.source_pipeline_id || !delivery.return_stage_id) {
    return (
      <p className="text-sm text-gray-500">
        Configure source pipeline, buyer pipeline, and return destination under Delivery before adding return routes.
      </p>
    );
  }

  const loading = buyerLoading || pubLoading || rulesLoading;
  const rulesCount = rules?.length ?? 0;

  return (
    <div>
      <p className="mb-3 text-xs text-gray-400">
        When a lead enters a return start stage on the buyer pipeline, it moves to the return destination on your
        pipeline. At least one return route is required for pipeline delivery.
      </p>
      {!loading && rulesCount === 0 && (
        <p className="mb-3 text-sm text-amber-700">Add at least one return route before activating or saving pipeline delivery.</p>
      )}
      <ContractReturnRulesEditor
        side="publisher"
        buyerStages={buyerStages ?? []}
        publisherStages={publisherStages ?? []}
        rules={rules ?? []}
        defaultReturnStageId={delivery.return_stage_id}
        loading={loading}
        onAdd={(buyerStageId, returnStageId) =>
          addRule.mutate(
            { contractId, buyerStageId, returnStageId },
            { onError: (e) => toast.error(errorMessage(e)) }
          )
        }
        onUpdate={(ruleId, buyerStageId, returnStageId) =>
          updateRule.mutate(
            { contractId, ruleId, buyerStageId, returnStageId },
            { onError: (e) => toast.error(errorMessage(e)) }
          )
        }
        onDelete={(ruleId) =>
          removeRule.mutate({ contractId, ruleId }, { onError: (e) => toast.error(errorMessage(e)) })
        }
      />
    </div>
  );
}
