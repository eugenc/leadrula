import { useState } from "react";
import { Sheet, DrawerHeader, DrawerBody, DrawerFooter } from "@/components/ui/dialog";
import { Button } from "@/components/ui/button";
import { SectionLabel } from "@/components/layout/SectionLabel";
import { toast } from "@/store/toastStore";
import { errorMessage } from "@/lib/api";
import { formatMoney } from "@/lib/utils";
import {
  useParticipationCompensations,
  useParticipationReturnRoutes,
  useAddParticipationReturnRoute,
  useUpdateParticipationReturnRoute,
  useDeleteParticipationReturnRoute,
  useUpdateParticipationDelivery,
  useUpdateParticipationStatus,
} from "@/features/admin/hooks";
import { useStages } from "@/features/leads/hooks";
import { ContractFormTabs } from "@/features/admin/ContractFormTabs";
import { emptyCompensationDraft } from "@/features/admin/CreateContractCompensationList";
import { emptyContractDelivery } from "@/features/admin/contractCompensation";
import { formatCapPeriod, formatContractCap } from "@/features/admin/contractCap";
import { formatContractLeadType } from "@/features/admin/contractLeadType";
import { formatParticipationStatus } from "@/features/admin/contractOffer";
import { COMPENSATION_KINDS, formatCompTrigger } from "@/features/admin/contractCompensation";
import { ContractReturnRulesEditor } from "@/features/admin/ContractReturnRulesEditor";
import { BuyerContractFieldMapSection } from "@/features/admin/BuyerContractFieldMapSection";
import { BuyerTriggerStageFields } from "@/features/admin/BuyerTriggerStageFields";
import {
  BuyerParticipationDeliveryFields,
  participationDeliveryValid,
} from "@/features/admin/BuyerParticipationDeliveryFields";
import type { ContractParticipation } from "@/types";

export function BuyerParticipationDetailDrawer({
  participation,
  onClose,
}: {
  participation: ContractParticipation | null;
  onClose: () => void;
}) {
  return (
    <Sheet open={!!participation} onClose={onClose} width={640}>
      {participation && <DrawerContent key={participation.id} participation={participation} onClose={onClose} />}
    </Sheet>
  );
}

function DrawerContent({
  participation,
  onClose,
}: {
  participation: ContractParticipation;
  onClose: () => void;
}) {
  const allowed = participation.allowed_delivery_modes ?? ["leads", "leads_pipeline"];
  const [delivery, setDelivery] = useState(participation.delivery || allowed[0] || "leads");
  const [pipelineId, setPipelineId] = useState(participation.buyer_pipeline_id ?? 0);
  const [stageId, setStageId] = useState(participation.buyer_target_stage_id ?? 0);
  const [webhookId, setWebhookId] = useState(participation.outbound_webhook_id ?? 0);
  const [integrationId, setIntegrationId] = useState(participation.integration_connection_id ?? 0);
  const [status, setStatus] = useState(participation.status);

  const pipelineDelivery = delivery === "leads_pipeline";
  const editable = status === "active" || status === "paused";
  const contractActive = participation.contract_status === "active";
  const canEditDelivery = editable && contractActive;

  const { data: compensations, isLoading: compsLoading } = useParticipationCompensations(participation.id);
  const { data: buyerStages, isLoading: buyerStagesLoading } = useStages(pipelineId || undefined);
  const { data: returnRoutes, isLoading: routesLoading } = useParticipationReturnRoutes(
    pipelineDelivery ? participation.id : null
  );
  const returnRoutesLoading = routesLoading || buyerStagesLoading;

  const saveDelivery = useUpdateParticipationDelivery();
  const updateStatus = useUpdateParticipationStatus();
  const addRoute = useAddParticipationReturnRoute();
  const updateRoute = useUpdateParticipationReturnRoute();
  const removeRoute = useDeleteParticipationReturnRoute();

  const publisherReturnStageId = participation.return_stage_id ?? 0;
  const publisherPipelineConfigured =
    (participation.source_pipeline_id ?? 0) > 0 && publisherReturnStageId > 0;
  const buyerPipelineSelected = pipelineId > 0;
  const deliveryValid = participationDeliveryValid(delivery, pipelineId, stageId, webhookId);
  const returnRoutesValid = !pipelineDelivery || (returnRoutes?.length ?? 0) > 0;

  const primaryRate =
    compensations?.find((c) => c.kind === "flat_rate")?.flat_amount ??
    compensations?.find((c) => c.trigger === "per_lead")?.flat_amount ??
    0;

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
      { id: participation.id, body: buildDeliveryBody() },
      {
        onSuccess: () => toast.success("Delivery saved"),
        onError: (e) => toast.error(errorMessage(e)),
      }
    );
  }

  function setParticipationStatus(next: string, successMessage: string) {
    updateStatus.mutate(
      { id: participation.id, status: next },
      {
        onSuccess: () => {
          setStatus(next);
          toast.success(successMessage);
          if (next === "withdrawn") onClose();
        },
        onError: (e) => toast.error(errorMessage(e)),
      }
    );
  }

  function withdrawContract() {
    if (!window.confirm("Cancel this contract permanently? You will stop receiving leads and cannot resume without a new invitation.")) {
      return;
    }
    setParticipationStatus("withdrawn", "Contract cancelled");
  }

  const emptyCriteria = { required_fields: [], field_map: [], filter_rules: [], quality_rules: [] };

  return (
    <div className="flex h-full flex-col">
      <DrawerHeader
        title={participation.contract_name ?? "Contract"}
        subtitle={`${participation.publisher_name ?? "Publisher"} · ${formatParticipationStatus(status)} · ${formatMoney(primaryRate)}/lead`}
        onClose={onClose}
      />

      <DrawerBody>
        {participation.contract_handler_id && (
          <div className="mb-4 flex items-center justify-between rounded-lg border border-gray-100 bg-gray-50 px-3 py-2.5">
            <span className="text-sm text-gray-400">Contract ID</span>
            <div className="flex items-center gap-2">
              <code className="text-sm font-semibold text-gray-800">{participation.contract_handler_id}</code>
              <Button
                variant="secondary"
                className="h-7 px-2 text-xs"
                onClick={() => {
                  void navigator.clipboard
                    .writeText(participation.contract_handler_id!)
                    .then(() => toast.success("Copied Contract ID"));
                }}
              >
                Copy
              </Button>
            </div>
          </div>
        )}

        {participation.contract_status && participation.contract_status !== "active" && (
          <p className="mb-3 rounded-lg border border-amber-100 bg-amber-50 px-3 py-2 text-sm text-amber-800">
            Publisher contract is {participation.contract_status}. Delivery changes are disabled until the publisher
            resumes the offer.
          </p>
        )}

        {status === "paused" && (
          <p className="mb-3 rounded-lg border border-gray-100 bg-gray-50 px-3 py-2 text-sm text-gray-600">
            Your participation is paused. You are not receiving new leads until you resume.
          </p>
        )}

        <ContractFormTabs
          showCheckmarks={false}
          form={{ contract_type: "sell", buyer_id: participation.buyer_id, name: participation.contract_name ?? "", lead_type: participation.lead_type ?? "" }}
          compensations={[emptyCompensationDraft()]}
          delivery={emptyContractDelivery()}
          leadCriteria={emptyCriteria}
          panels={{
            details: (
              <div className="flex flex-col gap-3 text-sm">
                <div>
                  <div className="text-xs font-semibold uppercase tracking-wide text-gray-400">Type</div>
                  <div className="mt-1 text-gray-700">Buy</div>
                </div>
                <div>
                  <div className="text-xs font-semibold uppercase tracking-wide text-gray-400">Publisher</div>
                  <div className="mt-1 text-gray-700">{participation.publisher_name ?? "—"}</div>
                </div>
                <div>
                  <div className="text-xs font-semibold uppercase tracking-wide text-gray-400">Lead type</div>
                  <div className="mt-1 text-gray-700">{formatContractLeadType(participation.lead_type) || "—"}</div>
                </div>
                <div>
                  <div className="text-xs font-semibold uppercase tracking-wide text-gray-400">Cap limits</div>
                  <div className="mt-1 text-gray-700">{formatContractCap(participation)}</div>
                </div>
                <div>
                  <div className="text-xs font-semibold uppercase tracking-wide text-gray-400">Description</div>
                  <div className="mt-1 whitespace-pre-wrap text-gray-700">
                    {participation.contract_description || "—"}
                  </div>
                </div>
                <div>
                  <div className="text-xs font-semibold uppercase tracking-wide text-gray-400">Your status</div>
                  <div className="mt-1 text-gray-700">{formatParticipationStatus(status)}</div>
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
                          participationId: participation.id,
                          buyerStageId,
                          buyerPipelineId: pipelineId || undefined,
                        },
                        { onError: (e) => toast.error(errorMessage(e)) }
                      )
                    }
                    onUpdate={(ruleId, buyerStageId) =>
                      updateRoute.mutate(
                        {
                          participationId: participation.id,
                          ruleId,
                          buyerStageId,
                          buyerPipelineId: pipelineId || undefined,
                        },
                        { onError: (e) => toast.error(errorMessage(e)) }
                      )
                    }
                    onDelete={(ruleId) =>
                      removeRoute.mutate(
                        { participationId: participation.id, ruleId },
                        { onError: (e) => toast.error(errorMessage(e)) }
                      )
                    }
                  />
                )}
              </div>
            ) : (
              <p className="text-sm text-gray-500">Return routes apply when delivery mode is Pipeline.</p>
            ),
            fieldmap: <BuyerContractFieldMapSection participationId={participation.id} />,
            triggers: (
              <BuyerTriggerStageFields
                participationId={participation.id}
                buyerPipelineId={pipelineId || participation.buyer_pipeline_id}
              />
            ),
          }}
          extraTabs={[
            { id: "fieldmap", label: "Field mapping" },
            { id: "triggers", label: "Triggers", optional: true },
          ]}
        />
      </DrawerBody>

      {editable && (
        <DrawerFooter className="flex justify-end gap-2">
          {status === "active" && (
            <>
              <Button
                variant="secondary"
                className="min-w-[10.5rem]"
                disabled={updateStatus.isPending}
                onClick={() => setParticipationStatus("paused", "Participation paused")}
              >
                Pause contract
              </Button>
              <Button
                variant="secondary"
                className="min-w-[10.5rem]"
                disabled={updateStatus.isPending}
                onClick={withdrawContract}
              >
                Cancel contract
              </Button>
            </>
          )}
          {status === "paused" && (
            <>
              <Button
                variant="secondary"
                className="min-w-[10.5rem]"
                disabled={updateStatus.isPending}
                onClick={() => setParticipationStatus("active", "Participation resumed")}
              >
                Resume contract
              </Button>
              <Button
                variant="secondary"
                className="min-w-[10.5rem]"
                disabled={updateStatus.isPending}
                onClick={withdrawContract}
              >
                Cancel contract
              </Button>
            </>
          )}
        </DrawerFooter>
      )}
    </div>
  );
}
