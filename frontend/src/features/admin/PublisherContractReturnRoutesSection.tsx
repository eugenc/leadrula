import { useEffect, useState } from "react";
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
import { ReturnScheduleFields } from "@/features/admin/ReturnScheduleFields";
import { scheduleFromRule, type ReturnScheduleDraft } from "@/features/admin/returnSchedule";
import { Input, Select } from "@/components/ui/input";
import { Spinner } from "@/components/ui/misc";
import type { ParticipationReturnRule, ReturnRule } from "@/types";

type Props = {
  contractId: number;
  delivery: ContractDeliveryDraft;
  openOffer?: boolean;
};

type ReturnRouteUpdate = {
  returnStageId?: number;
  schedule?: ReturnScheduleDraft;
  label?: string;
};

function ReturnRouteRow({
  rule,
  sortedPublisher,
  onUpdate,
}: {
  rule: {
    id: number;
    stale?: boolean;
    return_stage_id?: number | null;
  } & ReturnRule;
  sortedPublisher: { id: number; name: string }[];
  onUpdate: (ruleId: number, patch: ReturnRouteUpdate) => void;
}) {
  const pending = !rule.return_stage_id;
  const [schedule, setSchedule] = useState(() => scheduleFromRule(rule));
  const [label, setLabel] = useState(rule.label ?? "");

  useEffect(() => {
    setSchedule(scheduleFromRule(rule));
    setLabel(rule.label ?? "");
  }, [
    rule.id,
    rule.return_stage_id,
    rule.label,
    rule.return_schedule_mode,
    rule.return_delay_value,
    rule.return_delay_unit,
    rule.return_delay_seconds,
    rule.return_time,
    rule.return_weekdays,
  ]);

  function save(patch: ReturnRouteUpdate) {
    onUpdate(rule.id, {
      returnStageId: rule.return_stage_id ?? undefined,
      label,
      ...patch,
    });
  }

  function saveDestination(returnStageId: number) {
    save({ returnStageId });
  }

  function saveSchedule(next: ReturnScheduleDraft) {
    setSchedule(next);
    if (rule.return_stage_id) {
      save({ schedule: next });
    }
  }

  function saveLabel(next: string) {
    setLabel(next);
    save({ label: next });
  }

  return (
    <div className="space-y-2 rounded-md border border-gray-100 px-3 py-2">
      <div className="flex flex-wrap items-end gap-2">
        <div className="min-w-[120px] flex-1">
          <div className="mb-1 text-xs font-semibold text-gray-500">Return destination</div>
          {pending ? (
            <Select value={0} onChange={(e) => saveDestination(Number(e.target.value))}>
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
              onChange={(e) => saveDestination(Number(e.target.value))}
            >
              {sortedPublisher.map((s) => (
                <option key={s.id} value={s.id}>
                  {s.name}
                </option>
              ))}
            </Select>
          )}
        </div>
        <div className="min-w-[120px] flex-1">
          <div className="mb-1 text-xs font-semibold text-gray-500">Label</div>
          <Input
            value={label}
            placeholder="Optional label"
            onChange={(e) => setLabel(e.target.value)}
            onBlur={(e) => saveLabel(e.target.value)}
          />
          {rule.stale && (
            <p className="mt-1 text-xs text-amber-700">
              This return route is stale — buyer must re-add it before returns will trigger.
            </p>
          )}
        </div>
      </div>
      {!pending && (
        <ReturnScheduleFields
          compact
          radioGroupName={`return-schedule-${rule.id}`}
          value={schedule}
          onChange={saveSchedule}
        />
      )}
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
        Select Distribute from Pipeline under Distribution before mapping buyer return routes.
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
        Buyers configure return routes when they accept. Map each route to a stage on your pipeline.
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
                    onUpdate={(ruleId, patch) =>
                      updateDestination.mutate(
                        { ruleId, ...patch },
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
        Select Distribute from Pipeline under Distribution before mapping buyer return routes.
      </p>
    );
  }

  const loading = pubLoading || rulesLoading;
  if (loading) {
    return <Spinner className="h-5 w-5" />;
  }

  const sortedPublisher = [...(publisherStages ?? [])].sort((a, b) => a.position - b.position);
  const routeList = (rules ?? []) as ReturnRule[];
  const hasStaleReturnRoutes = routeList.some((r) => r.stale);

  return (
    <div>
      <p className="mb-3 text-xs text-gray-400">
        The buyer configures return routes on their contract. Map each route to a stage on your pipeline.
      </p>
      {hasStaleReturnRoutes && (
        <p className="mb-3 rounded-lg border border-amber-100 bg-amber-50 px-3 py-2 text-sm text-amber-800">
          Some return routes are stale and will not trigger returns until the buyer re-adds them.
        </p>
      )}
      {routeList.length === 0 ? (
        <p className="text-sm text-gray-500">No return routes yet. The buyer configures return routes on their contract.</p>
      ) : (
        <div className="space-y-2">
          {routeList.map((rule) => (
            <ReturnRouteRow
              key={rule.id}
              rule={rule}
              sortedPublisher={sortedPublisher}
              onUpdate={(ruleId, patch) =>
                updateDestination.mutate(
                  { ruleId, ...patch },
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
