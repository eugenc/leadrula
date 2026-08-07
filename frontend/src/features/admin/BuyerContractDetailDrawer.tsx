import { useEffect, useMemo, useState } from "react";
import { Sheet, DrawerHeader, DrawerBody } from "@/components/ui/dialog";
import { ContractMessageButton } from "@/features/messaging/MessageButton";
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
import { ContractStatusBadge, formatContractStatus } from "@/features/admin/contractStatus";
import { COMPENSATION_KINDS, formatCompTrigger } from "@/features/admin/contractCompensation";
import { ContractReturnRulesEditor } from "@/features/admin/ContractReturnRulesEditor";
import { BuyerContractFieldMapSection } from "@/features/admin/BuyerContractFieldMapSection";
import { BuyerContractDeliveryCalendarSection } from "@/features/appointments/BuyerContractDeliveryCalendarSection";
import { BuyerTriggerStageFields } from "@/features/admin/BuyerTriggerStageFields";
import {
  BuyerParticipationDeliveryFields,
  participationDeliveryValid,
  deliverySaveBlockReason,
} from "@/features/admin/BuyerParticipationDeliveryFields";
import type { Contract } from "@/types";

function initialDeliveryFrom(contract: Contract, allowed: string[]) {
  return contract.delivery || (contract.buyer_pipeline_id ? "leads_pipeline" : allowed[0] || "leads");
}

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
  const allowed = useMemo(
    () => contract.allowed_delivery_modes ?? ["leads", "leads_pipeline"],
    [contract.allowed_delivery_modes]
  );
  const [delivery, setDelivery] = useState(() => initialDeliveryFrom(contract, allowed));
  const [pipelineId, setPipelineId] = useState(contract.buyer_pipeline_id ?? 0);
  const [stageId, setStageId] = useState(contract.buyer_target_stage_id ?? 0);
  const [webhookId, setWebhookId] = useState(contract.outbound_webhook_id ?? 0);
  const [integrationId, setIntegrationId] = useState(contract.integration_connection_id ?? 0);

  useEffect(() => {
    setDelivery(initialDeliveryFrom(contract, allowed));
    setPipelineId(contract.buyer_pipeline_id ?? 0);
    setStageId(contract.buyer_target_stage_id ?? 0);
    setWebhookId(contract.outbound_webhook_id ?? 0);
    setIntegrationId(contract.integration_connection_id ?? 0);
  }, [
    contract.id,
    contract.delivery,
    contract.buyer_pipeline_id,
    contract.buyer_target_stage_id,
    contract.outbound_webhook_id,
    contract.integration_connection_id,
    allowed,
  ]);

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

  const publisherPipelineConfigured = (contract.source_pipeline_id ?? 0) > 0;
  const buyerPipelineSelected = pipelineId > 0;
  const deliveryValid = participationDeliveryValid(delivery, pipelineId, stageId, webhookId);
  const returnRoutesValid = !pipelineDelivery || (returnRoutes?.length ?? 0) > 0;
  const hasStaleReturnRoutes = (returnRoutes ?? []).some((r) => r.stale);
  const deliverySaveBlock = deliverySaveBlockReason(delivery, pipelineId, stageId, webhookId);

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
        <div className="mb-4">
          <ContractMessageButton contractId={contract.public_id} />
        </div>
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
            Publisher contract is {formatContractStatus(contract.status)}. Delivery changes are disabled until the publisher resumes the
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
                  <div className="mt-1 text-gray-700">{formatContractStatus(contract.status)}</div>
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
                  showIntegration
                />
                {canEditDelivery && (
                  <Button
                    className="mt-3"
                    variant="secondary"
                    disabled={!deliveryValid || saveDelivery.isPending}
                    title={
                      saveDelivery.isPending
                        ? "Saving…"
                        : deliverySaveBlock ?? undefined
                    }
                    onClick={saveDeliverySettings}
                  >
                    {saveDelivery.isPending ? "Saving…" : "Save delivery"}
                  </Button>
                )}
                {canEditDelivery && deliverySaveBlock && (
                  <p className="mt-2 text-xs text-gray-500">{deliverySaveBlock}</p>
                )}
                {contract.lead_type === "Appointment" && (
                  <BuyerContractDeliveryCalendarSection contract={contract} />
                )}
                {canEditDelivery && deliveryValid && pipelineDelivery && !returnRoutesValid && (
                  <p className="mt-2 text-xs text-amber-700">
                    Add return routes on the Return routes tab so leads can be returned to the publisher.
                  </p>
                )}
              </>
            ),
            returns: pipelineDelivery ? (
              <div className="space-y-3">
                <SectionLabel>Return routes</SectionLabel>
                {!returnRoutesLoading && !buyerPipelineSelected && (
                  <p className="text-sm text-gray-500">Select your destination pipeline under Delivery first.</p>
                )}
                {!returnRoutesLoading && buyerPipelineSelected && !publisherPipelineConfigured && (
                  <p className="text-sm text-gray-500">
                    The publisher must finish pipeline delivery setup before you can add return routes.
                  </p>
                )}
                {(returnRoutesLoading || (buyerPipelineSelected && publisherPipelineConfigured)) && (
                  <>
                    {hasStaleReturnRoutes && (
                      <p className="mb-2 rounded-lg border border-amber-100 bg-amber-50 px-3 py-2 text-sm text-amber-800">
                        Some return routes point at stages from an old buyer pipeline and will not trigger returns.
                        Delete them and re-add after your Delivery pipeline is correct.
                      </p>
                    )}
                    <ContractReturnRulesEditor
                    side="buyer"
                    buyerStages={buyerStages ?? []}
                    publisherStages={[]}
                    rules={returnRoutes ?? []}
                    loading={returnRoutesLoading}
                    onAdd={(buyerStageId) =>
                      addRoute.mutate(
                        {
                          contractId: contract.id,
                          buyerStageId,
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
                  </>
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
