import { useState } from "react";
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
  useUpdateBuyerContractDelivery,
  useContractCompensations,
} from "@/features/admin/hooks";
import { useStages } from "@/features/leads/hooks";
import { ContractFormTabs } from "@/features/admin/ContractFormTabs";
import { emptyCompensationDraft } from "@/features/admin/CreateContractCompensationList";
import { emptyContractDelivery } from "@/features/admin/contractCompensation";
import { formatCapPeriod, formatContractCap } from "@/features/admin/contractCap";
import { formatContractLeadType } from "@/features/admin/contractLeadType";
import { ContractStatusBadge } from "@/features/admin/contractStatus";
import { COMPENSATION_KINDS, formatCompTrigger } from "@/features/admin/contractCompensation";
import { ContractReturnRulesEditor } from "@/features/admin/ContractReturnRulesEditor";
import { BuyerContractFieldMapSection } from "@/features/admin/BuyerContractFieldMapSection";
import { BuyerTriggerStageFields } from "@/features/admin/BuyerTriggerStageFields";
import {
  BuyerParticipationDeliveryFields,
  participationDeliveryValid,
} from "@/features/admin/BuyerParticipationDeliveryFields";
import type { Contract } from "@/types";

export function BuyerContractDetailDrawer({
  contract,
  onClose,
}: {
  contract: Contract | null;
  onClose: () => void;
}) {
  return (
    <Sheet open={!!contract} onClose={onClose} width={640}>
      {contract && <DrawerContent key={contract.id} contract={contract} onClose={onClose} />}
    </Sheet>
  );
}

function DrawerContent({ contract, onClose }: { contract: Contract; onClose: () => void }) {
  const allowed = contract.allowed_delivery_modes ?? ["leads", "leads_pipeline"];
  const initialDelivery =
    contract.delivery ||
    (contract.buyer_pipeline_id ? "leads_pipeline" : allowed[0] || "leads");
  const [delivery, setDelivery] = useState(initialDelivery);
  const [pipelineId, setPipelineId] = useState(contract.buyer_pipeline_id ?? 0);
  const [stageId, setStageId] = useState(contract.buyer_target_stage_id ?? 0);
  const [webhookId, setWebhookId] = useState(contract.outbound_webhook_id ?? 0);
  const [integrationId, setIntegrationId] = useState(contract.integration_connection_id ?? 0);

  const pipelineDelivery = delivery === "leads_pipeline";
  const contractActive = contract.status === "active";
  const canEditDelivery = contractActive;

  const { data: compensations, isLoading: compsLoading } = useContractCompensations(contract.id, true);
  const { data: buyerStages, isLoading: buyerStagesLoading } = useStages(pipelineId || undefined);
  const { data: returnRoutes, isLoading: routesLoading } = useReturnRules(
    pipelineDelivery ? contract.id : null,
    true
  );
  const returnRoutesLoading = routesLoading || buyerStagesLoading;

  const saveDelivery = useUpdateBuyerContractDelivery();
  const addRoute = useAddReturnRule(true);
  const updateRoute = useUpdateReturnRule(true);
  const removeRoute = useDeleteReturnRule(true);

  const publisherReturnStageId = contract.return_stage_id ?? 0;
  const publisherPipelineConfigured =
    (contract.source_pipeline_id ?? 0) > 0 && publisherReturnStageId > 0;
  const buyerPipelineSelected = pipelineId > 0;
  const deliveryValid = participationDeliveryValid(delivery, pipelineId, stageId, webhookId);
  const returnRoutesValid = !pipelineDelivery || (returnRoutes?.length ?? 0) > 0;

  const hasTriggerComps = (compensations ?? []).some(
    (c) => (c.kind === "rev_share" || c.kind === "profit_share") && c.trigger === "buyer_stage"
  );

  function buildDeliveryBody() {
    const body: Record<string, unknown> = { delivery };
    if (pipelineDelivery) {
      body.buyer_pipeline_id = pipelineId;
      body.buyer_target_stage_id = stageId;
    }
    if (delivery === "webhook" && webhookId) body.outbound_webhook_id = webhookId;
    if (integrationId) body.integration_connection_id = integrationId;
    return body;
  }

  function saveDeliverySettings() {
    saveDelivery.mutate(
      { contractId: contract.id, body: buildDeliveryBody() },
      {
        onSuccess: () => toast.success("Delivery saved"),
        onError: (e) => toast.error(errorMessage(e)),
      }
    );
  }

  const emptyCriteria = { required_fields: [], field_map: [], filter_rules: [], quality_rules: [] };

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

        {!contractActive && (
          <p className="mb-3 rounded-lg border border-amber-100 bg-amber-50 px-3 py-2 text-sm text-amber-800">
            Publisher contract is {contract.status}. Delivery changes are disabled until the publisher resumes the
            contract.
          </p>
        )}

        <ContractFormTabs
          showCheckmarks={false}
          form={{ contract_type: "sell", buyer_id: contract.buyer_id ?? 0, name: contract.name, lead_type: contract.lead_type ?? "" }}
          compensations={[emptyCompensationDraft()]}
          delivery={emptyContractDelivery()}
          leadCriteria={emptyCriteria}
          panels={{
            details: (
              <div className="flex flex-col gap-3 text-sm">
                <div className="flex items-center justify-between">
                  <ContractStatusBadge status={contract.status} />
                </div>
                <div>
                  <div className="text-xs font-semibold uppercase tracking-wide text-gray-400">Type</div>
                  <div className="mt-1 text-gray-700">Buy</div>
                </div>
                <div>
                  <div className="text-xs font-semibold uppercase tracking-wide text-gray-400">Publisher</div>
                  <div className="mt-1 text-gray-700">{contract.publisher_name ?? "—"}</div>
                </div>
                <div>
                  <div className="text-xs font-semibold uppercase tracking-wide text-gray-400">Lead type</div>
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
                <div>
                  <div className="text-xs font-semibold uppercase tracking-wide text-gray-400">Contract status</div>
                  <div className="mt-1 text-gray-700">{contract.status}</div>
                </div>
              </div>
            ),
            compensation: (
              <div className="flex flex-col gap-2">
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
            ),
            delivery: (
              <>
                <BuyerParticipationDeliveryFields
                  allowedModes={allowed}
                  delivery={delivery}
                  onDeliveryChange={setDelivery}
                  pipelineId={pipelineId}
                  onPipelineIdChange={setPipelineId}
                  stageId={stageId}
                  onStageIdChange={setStageId}
                  webhookId={webhookId}
                  onWebhookIdChange={setWebhookId}
                  integrationId={integrationId}
                  onIntegrationIdChange={setIntegrationId}
                  showIntegration={false}
                />
                {canEditDelivery && (
                  <Button
                    className="mt-3"
                    variant="secondary"
                    disabled={
                      !deliveryValid ||
                      (pipelineDelivery && !returnRoutesValid) ||
                      saveDelivery.isPending
                    }
                    onClick={saveDeliverySettings}
                  >
                    Save delivery
                  </Button>
                )}
                {pipelineDelivery && !returnRoutesValid && (
                  <p className="mt-2 text-xs text-gray-500">
                    Add at least one return route before saving pipeline delivery.
                  </p>
                )}
              </>
            ),
            returns: pipelineDelivery ? (
              <div className="space-y-3">
                <SectionLabel>Return routes</SectionLabel>
                <p className="text-xs text-gray-400">
                  When a lead enters a stage on your pipeline, it is returned to the publisher. Return destination is
                  set by the publisher on the contract offer.
                </p>
                {!returnRoutesLoading && !buyerPipelineSelected && (
                  <p className="text-sm text-gray-500">Select your destination pipeline under Delivery first.</p>
                )}
                {!returnRoutesLoading && buyerPipelineSelected && !publisherPipelineConfigured && (
                  <p className="text-sm text-gray-500">
                    The publisher has not configured a return destination on this offer yet.
                  </p>
                )}
                {(returnRoutesLoading || (buyerPipelineSelected && publisherPipelineConfigured)) && (
                  <ContractReturnRulesEditor
                    side="buyer"
                    buyerStages={buyerStages ?? []}
                    publisherStages={[]}
                    rules={returnRoutes ?? []}
                    defaultReturnStageId={publisherReturnStageId}
                    loading={returnRoutesLoading}
                    onAdd={(buyerStageId) =>
                      addRoute.mutate(
                        {
                          contractId: contract.id,
                          buyerStageId,
                          returnStageId: publisherReturnStageId,
                          buyerPipelineId: pipelineId || undefined,
                        },
                        { onError: (e) => toast.error(errorMessage(e)) }
                      )
                    }
                    onUpdate={(ruleId, buyerStageId) =>
                      updateRoute.mutate(
                        {
                          contractId: contract.id,
                          ruleId,
                          buyerStageId,
                          returnStageId: publisherReturnStageId,
                          buyerPipelineId: pipelineId || undefined,
                        },
                        { onError: (e) => toast.error(errorMessage(e)) }
                      )
                    }
                    onDelete={(ruleId) =>
                      removeRoute.mutate(
                        { contractId: contract.id, ruleId },
                        { onError: (e) => toast.error(errorMessage(e)) }
                      )
                    }
                  />
                )}
              </div>
            ) : (
              <p className="text-sm text-gray-500">Return routes apply when delivery mode is Pipeline.</p>
            ),
            fieldmap: <BuyerContractFieldMapSection contractId={contract.id} />,
            ...(hasTriggerComps
              ? {
                  triggers: (
                    <BuyerTriggerStageFields
                      contractId={contract.id}
                      buyerPipelineId={pipelineId || contract.buyer_pipeline_id}
                    />
                  ),
                }
              : {}),
          }}
          extraTabs={[
            { id: "fieldmap", label: "Field mapping" },
            ...(hasTriggerComps ? [{ id: "triggers" as const, label: "Rev / profit share" }] : []),
          ]}
        />
      </DrawerBody>
    </div>
  );
}
