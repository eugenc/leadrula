import { toast } from "@/store/toastStore";
import { errorMessage } from "@/lib/api";
import {
  useReturnRules,
  useContractParticipationReturnRoutes,
  useUpdateParticipationReturnRuleDestination,
  useUpdateContractReturnRuleDestination,
} from "@/features/admin/hooks";
import { useStages } from "@/features/leads/hooks";
import type { ContractDeliveryDraft } from "@/features/admin/contractCompensation";
import { Select } from "@/components/ui/input";
import { Spinner } from "@/components/ui/misc";
import type { ParticipationReturnRule, ReturnRule } from "@/types";

type Props = {
  contractId: number;
  delivery: ContractDeliveryDraft;
  openOffer?: boolean;
};

function ReturnRouteRow({
  rule,
  sortedPublisher,
  onUpdate,
}: {
  rule: { id: number; buyer_stage_id: number; buyer_stage_name?: string; return_stage_id?: number | null };
  sortedPublisher: { id: number; name: string }[];
  onUpdate: (ruleId: number, returnStageId: number) => void;
}) {
  const pending = !rule.return_stage_id;
  return (
    <div className="flex flex-wrap items-end gap-2 rounded-md border border-gray-100 px-3 py-2">
      <div className="min-w-[120px] flex-1">
        <div className="mb-1 text-xs font-semibold text-gray-500">Return start</div>
        <div className="rounded border border-gray-100 bg-gray-50 px-2 py-1.5 text-sm text-gray-700">
          {rule.buyer_stage_name || `#${rule.buyer_stage_id}`}
        </div>
      </div>
      <div className="min-w-[120px] flex-1">
        <div className="mb-1 text-xs font-semibold text-gray-500">Return destination</div>
        {pending ? (
          <Select value={0} onChange={(e) => onUpdate(rule.id, Number(e.target.value))}>
            <option value={0}>Pending — select stage…</option>
            {sortedPublisher.map((s) => (
              <option key={s.id} value={s.id}>
                {s.name}
              </option>
            ))}
          </Select>
        ) : (
          <Select
            value={rule.return_stage_id ?? 0}
            onChange={(e) => onUpdate(rule.id, Number(e.target.value))}
          >
            {sortedPublisher.map((s) => (
              <option key={s.id} value={s.id}>
                {s.name}
              </option>
            ))}
          </Select>
        )}
      </div>
    </div>
  );
}

function groupByBuyer(rules: ParticipationReturnRule[]) {
  const groups = new Map<number, { buyerName: string; rules: ParticipationReturnRule[] }>();
  for (const rule of rules) {
    const participationId = rule.participation_id ?? 0;
    const existing = groups.get(participationId);
    if (existing) {
      existing.rules.push(rule);
    } else {
      groups.set(participationId, { buyerName: rule.buyer_name || `Buyer #${participationId}`, rules: [rule] });
    }
  }
  return [...groups.values()];
}

function OpenOfferReturnRoutes({
  contractId,
  delivery,
}: {
  contractId: number;
  delivery: ContractDeliveryDraft;
}) {
  const { data: publisherStages, isLoading: pubLoading } = useStages(
    delivery.source_pipeline_id || undefined
  );
  const { data: rules, isLoading: rulesLoading } = useContractParticipationReturnRoutes(contractId);
  const updateDestination = useUpdateParticipationReturnRuleDestination();

  if (!delivery.source_pipeline_id) {
    return (
      <p className="text-sm text-gray-500">
        Configure source pipeline under Delivery before mapping buyer return routes.
      </p>
    );
  }

  const loading = pubLoading || rulesLoading;
  if (loading) {
    return <Spinner className="h-5 w-5" />;
  }

  const groups = groupByBuyer(rules ?? []);
  const sortedPublisher = [...(publisherStages ?? [])].sort((a, b) => a.position - b.position);

  return (
    <div>
      <p className="mb-3 text-xs text-gray-400">
        Buyers pick return start stages when they accept. Map each buyer return start stage to a stage on your
        pipeline.
      </p>
      {groups.length === 0 ? (
        <p className="text-sm text-gray-500">No buyer return routes yet. Routes appear after buyers accept.</p>
      ) : (
        <div className="space-y-4">
          {groups.map((group) => (
            <div key={group.rules[0]?.participation_id ?? group.buyerName}>
              <div className="mb-2 text-sm font-semibold text-gray-700">{group.buyerName}</div>
              <div className="space-y-2">
                {group.rules.map((rule) => (
                  <ReturnRouteRow
                    key={rule.id}
                    rule={rule}
                    sortedPublisher={sortedPublisher}
                    onUpdate={(ruleId, returnStageId) =>
                      updateDestination.mutate(
                        { ruleId, returnStageId },
                        { onError: (err) => toast.error(errorMessage(err)) }
                      )
                    }
                  />
                ))}
              </div>
            </div>
          ))}
        </div>
      )}
    </div>
  );
}

function DirectContractReturnRoutes({
  contractId,
  delivery,
}: {
  contractId: number;
  delivery: ContractDeliveryDraft;
}) {
  const { data: publisherStages, isLoading: pubLoading } = useStages(
    delivery.source_pipeline_id || undefined
  );
  const { data: rules, isLoading: rulesLoading } = useReturnRules(contractId, false);
  const updateDestination = useUpdateContractReturnRuleDestination();

  if (!delivery.source_pipeline_id) {
    return (
      <p className="text-sm text-gray-500">
        Configure source pipeline under Delivery before mapping buyer return routes.
      </p>
    );
  }

  const loading = pubLoading || rulesLoading;
  if (loading) {
    return <Spinner className="h-5 w-5" />;
  }

  const sortedPublisher = [...(publisherStages ?? [])].sort((a, b) => a.position - b.position);
  const routeList = (rules ?? []) as ReturnRule[];

  return (
    <div>
      <p className="mb-3 text-xs text-gray-400">
        The buyer configures trigger stages on their contract. Map each buyer return start stage to a stage on your
        pipeline.
      </p>
      {routeList.length === 0 ? (
        <p className="text-sm text-gray-500">No return routes yet. The buyer configures trigger stages on their contract.</p>
      ) : (
        <div className="space-y-2">
          {routeList.map((rule) => (
            <ReturnRouteRow
              key={rule.id}
              rule={rule}
              sortedPublisher={sortedPublisher}
              onUpdate={(ruleId, returnStageId) =>
                updateDestination.mutate(
                  { ruleId, returnStageId },
                  { onError: (err) => toast.error(errorMessage(err)) }
                )
              }
            />
          ))}
        </div>
      )}
    </div>
  );
}

export function PublisherContractReturnRoutesSection({ contractId, delivery, openOffer = false }: Props) {
  if (openOffer) {
    return <OpenOfferReturnRoutes contractId={contractId} delivery={delivery} />;
  }

  if (delivery.delivery !== "leads_pipeline") {
    return (
      <p className="text-sm text-gray-500">
        Return routes apply when delivery mode is Pipeline. Configure delivery first, or use Leads inbox delivery.
      </p>
    );
  }

  return <DirectContractReturnRoutes contractId={contractId} delivery={delivery} />;
}
