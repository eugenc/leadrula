import { useCallback, useEffect, useLayoutEffect, useMemo, useRef, useState } from "react";
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
import { formatContractStatus } from "@/features/admin/contractStatus";
import { COMPENSATION_KINDS, formatCompTrigger } from "@/features/admin/contractCompensation";
import { ContractReturnRulesEditor } from "@/features/admin/ContractReturnRulesEditor";
import { BuyerContractFieldMapSection } from "@/features/admin/BuyerContractFieldMapSection";
import { BuyerCallTargetSection } from "@/features/calls/BuyerCallTargetSection";
import { BuyerTriggerStageFields } from "@/features/admin/BuyerTriggerStageFields";
import {
  BuyerParticipationDeliveryFields,
  participationDeliveryValid,
  deliverySaveBlockReason,
} from "@/features/admin/BuyerParticipationDeliveryFields";
import {
  buyerDeliveryBody,
  buyerDeliveryDirty,
  buyerDeliveryDraftFrom,
  type BuyerDeliveryDraft,
} from "@/features/admin/buyerDeliveryDirty";
import type { ContractParticipation } from "@/types";

export function BuyerParticipationDetailDrawer({
  participation,
  onClose,
  registerFlushHandler,
}: {
  participation: ContractParticipation | null;
  onClose: () => void;
  registerFlushHandler?: (fn: (() => Promise<boolean>) | null) => void;
}) {
  const closeRef = useRef(onClose);
  closeRef.current = onClose;

  return (
    <Sheet open={!!participation} onClose={() => closeRef.current()} width={640}>
      {participation && (
        <DrawerContent
          key={participation.id}
          participation={participation}
          onClose={onClose}
          registerClose={(fn) => {
            closeRef.current = fn ?? onClose;
          }}
          registerFlushHandler={registerFlushHandler}
        />
      )}
    </Sheet>
  );
}

function DrawerContent({
  participation,
  onClose,
  registerClose,
  registerFlushHandler,
}: {
  participation: ContractParticipation;
  onClose: () => void;
  registerClose: (fn: (() => void) | null) => void;
  registerFlushHandler?: (fn: (() => Promise<boolean>) | null) => void;
}) {
  const allowed = useMemo(
    () => participation.allowed_delivery_modes ?? ["leads", "leads_pipeline"],
    [participation.allowed_delivery_modes]
  );
  const touchedRef = useRef(false);
  const closingRef = useRef(false);

  const [delivery, setDeliveryState] = useState(
    () => buyerDeliveryDraftFrom(participation, allowed).delivery
  );
  const [pipelineId, setPipelineIdState] = useState(participation.buyer_pipeline_id ?? 0);
  const [stageId, setStageIdState] = useState(participation.buyer_target_stage_id ?? 0);
  const [webhookId, setWebhookIdState] = useState(participation.outbound_webhook_id ?? 0);
  const [integrationId, setIntegrationIdState] = useState(participation.integration_connection_id ?? 0);
  const [status, setStatusState] = useState(participation.status);

  function touch() {
    touchedRef.current = true;
  }
  const setDelivery = (v: string) => {
    touch();
    setDeliveryState(v);
  };
  const setPipelineId = (v: number) => {
    touch();
    setPipelineIdState(v);
  };
  const setStageId = (v: number) => {
    touch();
    setStageIdState(v);
  };
  const setWebhookId = (v: number) => {
    touch();
    setWebhookIdState(v);
  };
  const setIntegrationId = (v: number) => {
    touch();
    setIntegrationIdState(v);
  };

  useEffect(() => {
    touchedRef.current = false;
    const draft = buyerDeliveryDraftFrom(participation, allowed);
    setDeliveryState(draft.delivery);
    setPipelineIdState(draft.pipelineId);
    setStageIdState(draft.stageId);
    setWebhookIdState(draft.webhookId);
    setIntegrationIdState(draft.integrationId);
    setStatusState(participation.status);
  }, [
    participation.id,
    participation.delivery,
    participation.buyer_pipeline_id,
    participation.buyer_target_stage_id,
    participation.outbound_webhook_id,
    participation.integration_connection_id,
    participation.status,
    allowed,
  ]);

  const localDraft: BuyerDeliveryDraft = {
    delivery,
    pipelineId,
    stageId,
    webhookId,
    integrationId,
  };

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

  const publisherPipelineConfigured = (participation.source_pipeline_id ?? 0) > 0;
  const buyerPipelineSelected = pipelineId > 0;
  const deliveryValid = participationDeliveryValid(delivery, pipelineId, stageId, webhookId);
  const returnRoutesValid = !pipelineDelivery || (returnRoutes?.length ?? 0) > 0;
  const deliverySaveBlock = deliverySaveBlockReason(delivery, pipelineId, stageId, webhookId);

  const flushRef = useRef({
    participation,
    allowed,
    localDraft,
    canEditDelivery: false,
  });
  flushRef.current = {
    participation,
    allowed,
    localDraft,
    canEditDelivery,
  };

  const flushDelivery = useCallback(async (): Promise<boolean> => {
    const { participation: p, allowed: modes, localDraft: draft, canEditDelivery: editable } =
      flushRef.current;
    if (!editable || !buyerDeliveryDirty(draft, p, modes)) return true;
    if (!participationDeliveryValid(draft.delivery, draft.pipelineId, draft.stageId, draft.webhookId)) {
      toast.error("Complete distribution settings before saving.");
      return false;
    }
    const block = deliverySaveBlockReason(
      draft.delivery,
      draft.pipelineId,
      draft.stageId,
      draft.webhookId
    );
    if (block) {
      toast.error(block);
      return false;
    }

    const toastId = toast.progress("Saving…");
    try {
      await saveDelivery.mutateAsync({ id: p.id, body: buyerDeliveryBody(draft) });
      toast.update(toastId, "Saved");
      setTimeout(() => toast.dismiss(toastId), 1500);
      touchedRef.current = false;
      return true;
    } catch (e) {
      toast.dismiss(toastId);
      toast.error(errorMessage(e));
      return false;
    }
  }, [saveDelivery]);

  const handleClose = useCallback(async () => {
    if (closingRef.current) return;
    closingRef.current = true;
    try {
      const ok = await flushDelivery();
      if (ok) onClose();
    } finally {
      closingRef.current = false;
    }
  }, [flushDelivery, onClose]);

  useLayoutEffect(() => {
    registerClose(() => {
      void handleClose();
    });
    registerFlushHandler?.(flushDelivery);
    return () => {
      registerClose(null);
      registerFlushHandler?.(null);
    };
  }, [handleClose, flushDelivery, registerClose, registerFlushHandler]);

  const primaryRate =
    participation.rate_per_lead ??
    compensations?.find((c) => c.kind === "flat_rate")?.flat_amount ??
    compensations?.find((c) => c.trigger === "per_lead")?.flat_amount ??
    0;

  const hasTriggerComps = (compensations ?? []).some(
    (c) => (c.kind === "rev_share" || c.kind === "profit_share") && c.trigger === "buyer_stage"
  );

  const isCall = participation.lead_type === "Call";

  function saveDeliverySettings() {
    saveDelivery.mutate(
      { id: participation.id, body: buyerDeliveryBody(localDraft) },
      {
        onSuccess: () => {
          toast.success("Distribution saved");
          touchedRef.current = false;
        },
        onError: (e) => toast.error(errorMessage(e)),
      }
    );
  }

  function setParticipationStatus(next: string, successMessage: string) {
    updateStatus.mutate(
      { id: participation.id, status: next },
      {
        onSuccess: () => {
          setStatusState(next);
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
        subtitle={`${participation.publisher_name ?? "Publisher"} · ${formatParticipationStatus(status)} · ${formatMoney(primaryRate)}/lead · ${participation.lead_count ?? 0} received`}
        onClose={() => void handleClose()}
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
            Publisher contract is {formatContractStatus(participation.contract_status)}. Distribution changes are disabled until the publisher
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
                <div className="mt-4 border-t border-gray-100 pt-4">
                  <SectionLabel>Return Routes</SectionLabel>
                  {pipelineDelivery ? (
                    <div className="space-y-3">
                      {!returnRoutesLoading && !buyerPipelineSelected && (
                        <p className="text-sm text-gray-500">Select Distribute to Pipeline and Distribute to Stage under Distribution first.</p>
                      )}
                      {!returnRoutesLoading && buyerPipelineSelected && !publisherPipelineConfigured && (
                        <p className="text-sm text-gray-500">
                          The publisher must finish pipeline delivery setup before you can add return routes.
                        </p>
                      )}
                      {(returnRoutesLoading || (buyerPipelineSelected && publisherPipelineConfigured)) && (
                        <ContractReturnRulesEditor
                          side="buyer"
                          buyerStages={buyerStages ?? []}
                          publisherStages={[]}
                          rules={returnRoutes ?? []}
                          loading={returnRoutesLoading}
                          onAdd={(buyerStageId, _returnStageId, schedule, label) =>
                            addRoute.mutate(
                              {
                                participationId: participation.id,
                                buyerStageId,
                                buyerPipelineId: pipelineId || undefined,
                                schedule,
                                label,
                              },
                              { onError: (e) => toast.error(errorMessage(e)) }
                            )
                          }
                          onUpdate={(ruleId, buyerStageId, _returnStageId, schedule, label) =>
                            updateRoute.mutate(
                              {
                                participationId: participation.id,
                                ruleId,
                                buyerStageId,
                                buyerPipelineId: pipelineId || undefined,
                                schedule,
                                label,
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
                  )}
                </div>
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
                    {saveDelivery.isPending ? "Saving…" : "Save distribution"}
                  </Button>
                )}
                {canEditDelivery && deliverySaveBlock && (
                  <p className="mt-2 text-xs text-gray-500">{deliverySaveBlock}</p>
                )}
                {canEditDelivery && deliveryValid && pipelineDelivery && !returnRoutesValid && (
                  <p className="mt-2 text-xs text-amber-700">
                    Add return routes below so leads can be returned to the publisher.
                  </p>
                )}
              </>
            ),
            fieldmap: <BuyerContractFieldMapSection participationId={participation.id} />,
            ...(isCall ? { calltarget: <BuyerCallTargetSection participationId={participation.id} /> } : {}),
            ...(hasTriggerComps
              ? {
                  triggers: (
                    <BuyerTriggerStageFields
                      participationId={participation.id}
                      buyerPipelineId={pipelineId || participation.buyer_pipeline_id}
                    />
                  ),
                }
              : {}),
          }}
          extraTabs={[
            { id: "fieldmap", label: "Field Mapping" },
            ...(isCall ? [{ id: "calltarget" as const, label: "Call Target" }] : []),
            ...(hasTriggerComps ? [{ id: "triggers" as const, label: "Rev / Profit Share" }] : []),
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
