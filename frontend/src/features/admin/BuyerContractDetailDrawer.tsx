import { Sheet, DrawerHeader, DrawerBody } from "@/components/ui/dialog";
import { Button } from "@/components/ui/button";
import { SectionLabel } from "@/components/layout/SectionLabel";
import { toast } from "@/store/toastStore";
import { formatMoney } from "@/lib/utils";
import { useReturnRules } from "@/features/admin/hooks";
import { useStages } from "@/features/leads/hooks";
import { ContractStatusBadge } from "@/features/admin/contractStatus";
import { formatCapPeriod, formatContractCap } from "@/features/admin/contractCap";
import { formatContractLeadType } from "@/features/admin/contractLeadType";
import { COMPENSATION_KINDS, formatCompTrigger } from "@/features/admin/contractCompensation";
import { useContractCompensations } from "@/features/admin/hooks";
import { BuyerContractFieldMapSection } from "@/features/admin/BuyerContractFieldMapSection";
import { BuyerTriggerStageFields } from "@/features/admin/BuyerTriggerStageFields";
import type { Contract, ReturnRule, Stage } from "@/types";

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

function stageName(stages: Stage[], stageId: number): string {
  return stages.find((s) => s.id === stageId)?.name ?? `Stage #${stageId}`;
}

function DrawerContent({ contract, onClose }: { contract: Contract; onClose: () => void }) {
  const { data: buyerStages, isLoading: buyerStagesLoading } = useStages(contract.buyer_pipeline_id ?? undefined);
  const { data: rules, isLoading: rulesLoading } = useReturnRules(contract.id, true);
  const { data: compensations, isLoading: compsLoading } = useContractCompensations(contract.id, true);

  const loading = buyerStagesLoading || rulesLoading;

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

        <div className="mb-4">
          <SectionLabel className="mb-2">Field mapping</SectionLabel>
          <BuyerContractFieldMapSection contractId={contract.id} />
        </div>

        <div className="mb-4">
          <BuyerTriggerStageFields
            contractId={contract.id}
            buyerPipelineId={contract.buyer_pipeline_id}
          />
        </div>

        <div className="pt-2">
          <SectionLabel className="mb-2">Return Routes</SectionLabel>
          <p className="mb-3 text-xs text-gray-400">
            Return routes are configured by the publisher. When a lead enters a listed stage on your pipeline, it is
            returned to the publisher automatically.
          </p>
          {loading ? (
            <p className="text-sm text-gray-400">Loading…</p>
          ) : (rules ?? []).length === 0 ? (
            <p className="text-sm text-gray-500">No return routes configured yet.</p>
          ) : (
            <ReturnRulesReadOnlyList rules={rules ?? []} buyerStages={buyerStages ?? []} />
          )}
        </div>
      </DrawerBody>
    </div>
  );
}

function ReturnRulesReadOnlyList({ rules, buyerStages }: { rules: ReturnRule[]; buyerStages: Stage[] }) {
  return (
    <ul className="space-y-2">
      {rules.map((rule) => (
        <li
          key={rule.id}
          className="rounded-md border border-gray-100 px-3 py-2 text-sm text-gray-700"
        >
          <span className="font-medium">{stageName(buyerStages, rule.buyer_stage_id)}</span>
          <span className="text-gray-400"> → returned to publisher</span>
        </li>
      ))}
    </ul>
  );
}
