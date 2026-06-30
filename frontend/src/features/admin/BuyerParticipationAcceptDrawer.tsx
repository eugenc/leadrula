import { useState } from "react";
import { Check } from "lucide-react";
import { Sheet, DrawerHeader, DrawerBody, DrawerFooter } from "@/components/ui/dialog";
import { Button } from "@/components/ui/button";
import { Label, Select } from "@/components/ui/input";
import { SectionLabel } from "@/components/layout/SectionLabel";
import { toast } from "@/store/toastStore";
import { errorMessage } from "@/lib/api";
import { cn } from "@/lib/utils";
import {
  useAcceptParticipation,
  useDeclineParticipation,
  useCounterParticipation,
  useParticipationReturnRoutes,
  useAddParticipationReturnRoute,
  useUpdateParticipationReturnRoute,
  useDeleteParticipationReturnRoute,
} from "@/features/admin/hooks";
import { Input } from "@/components/ui/input";
import { useIntegrationConnections } from "@/features/integrations/hooks";
import { useStages } from "@/features/leads/hooks";
import { PUBLISHER_DELIVERY_MODES, formatParticipationStatus } from "@/features/admin/contractOffer";
import { BuyerContractFieldMapSection } from "@/features/admin/BuyerContractFieldMapSection";
import { BuyerTriggerStageFields } from "@/features/admin/BuyerTriggerStageFields";
import { ContractReturnRulesEditor } from "@/features/admin/ContractReturnRulesEditor";
import {
  BuyerParticipationDeliveryFields,
  participationDeliveryValid,
} from "@/features/admin/BuyerParticipationDeliveryFields";
import type { ContractParticipation } from "@/types";

const STEPS = ["Review", "Delivery", "Return routes", "Field mapping", "Integrations"] as const;

export function BuyerParticipationAcceptDrawer({
  participation,
  onClose,
}: {
  participation: ContractParticipation | null;
  onClose: () => void;
}) {
  if (!participation) return null;
  if (participation.status !== "pending" && participation.status !== "counter_pending") return null;
  return (
    <Sheet open onClose={onClose} width={640}>
      <DrawerContent participation={participation} onClose={onClose} />
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
  const accept = useAcceptParticipation();
  const decline = useDeclineParticipation();
  const counter = useCounterParticipation();
  const [step, setStep] = useState(0);
  const [mappingComplete, setMappingComplete] = useState(false);
  const [showCounterOffer, setShowCounterOffer] = useState(false);
  const [counterRate, setCounterRate] = useState("");
  const allowed = participation.allowed_delivery_modes ?? ["leads", "leads_pipeline"];
  const modeOptions = PUBLISHER_DELIVERY_MODES.filter((m) => allowed.includes(m.value));
  const [delivery, setDelivery] = useState<string>(modeOptions[0]?.value ?? "leads");
  const [pipelineId, setPipelineId] = useState(participation.buyer_pipeline_id ?? 0);
  const [stageId, setStageId] = useState(participation.buyer_target_stage_id ?? 0);
  const [webhookId, setWebhookId] = useState(0);
  const [integrationId, setIntegrationId] = useState(0);

  const pipelineDelivery = delivery === "leads_pipeline";
  const { data: stages, isLoading: buyerStagesLoading } = useStages(pipelineId || undefined);
  const { data: returnRoutes, isLoading: routesLoading } = useParticipationReturnRoutes(
    pipelineDelivery ? participation.id : null
  );
  const returnRoutesLoading = routesLoading || buyerStagesLoading;
  const addRoute = useAddParticipationReturnRoute();
  const updateRoute = useUpdateParticipationReturnRoute();
  const removeRoute = useDeleteParticipationReturnRoute();
  const { data: connections } = useIntegrationConnections();

  const deliveryValid = participationDeliveryValid(delivery, pipelineId, stageId, webhookId);
  const returnRoutesValid = !pipelineDelivery || (returnRoutes?.length ?? 0) > 0;
  const publisherPipelineConfigured = (participation.source_pipeline_id ?? 0) > 0;
  const buyerPipelineSelected = pipelineId > 0;

  function stepComplete(i: number): boolean {
    if (i === 0) return step > 0;
    if (i === 1) return step > 1;
    if (i === 2) return pipelineDelivery ? returnRoutesValid && step > 2 : step > 1;
    if (i === 3) return mappingComplete;
    if (i === 4) return step === 4;
    return false;
  }

  function canGoToStep(target: number): boolean {
    if (target <= step) return true;
    if (target >= 1 && !deliveryValid && target > 1) return false;
    if (pipelineDelivery && target >= 3 && !returnRoutesValid) return false;
    return true;
  }

  function goToStep(target: number) {
    if (!canGoToStep(target)) return;
    setStep(target);
  }

  function nextStep() {
    if (step === 1 && !pipelineDelivery) {
      setStep(3);
      return;
    }
    setStep((s) => s + 1);
  }

  function prevStep() {
    if (step === 3 && !pipelineDelivery) {
      setStep(1);
      return;
    }
    setStep((s) => s - 1);
  }

  function submitAccept() {
    const body: Record<string, unknown> = { delivery };
    if (pipelineDelivery) {
      body.buyer_pipeline_id = pipelineId;
      body.buyer_target_stage_id = stageId;
    }
    if (delivery === "webhook" && webhookId) body.outbound_webhook_id = webhookId;
    if (integrationId) body.integration_connection_id = integrationId;
    accept.mutate(
      { id: participation.id, body },
      {
        onSuccess: () => {
          toast.success("Contract accepted");
          onClose();
        },
        onError: (e) => toast.error(errorMessage(e)),
      }
    );
  }

  return (
    <>
      <DrawerHeader
        title={participation.contract_name ?? "Contract"}
        subtitle={`${participation.publisher_name ?? "Publisher"} · ${formatParticipationStatus(participation.status)}`}
        onClose={onClose}
      />
      <DrawerBody>
        <div className="mb-4 flex overflow-x-auto border-b border-gray-100">
          {STEPS.filter((s) => !(s === "Return routes" && !pipelineDelivery)).map((s) => {
            const stepIndex = STEPS.indexOf(s);
            const done = stepComplete(stepIndex);
            return (
              <button
                key={s}
                type="button"
                onClick={() => goToStep(stepIndex)}
                disabled={!canGoToStep(stepIndex)}
                className={cn(
                  "-mb-px flex shrink-0 items-center gap-1.5 border-b-2 px-3 py-2 text-sm font-semibold transition-colors",
                  step === stepIndex ? "border-jade-500 text-jade-700" : "border-transparent text-gray-400",
                  !canGoToStep(stepIndex) && "cursor-not-allowed opacity-50"
                )}
              >
                {done && <Check className="h-3.5 w-3.5 text-jade-600" />}
                <span>{s}</span>
              </button>
            );
          })}
        </div>

        {step === 0 && (
          <div className="space-y-2 text-sm text-gray-700">
            <p>Publisher: {participation.publisher_name}</p>
            <p>Lead type: {participation.lead_type || "—"}</p>
            <p>Allowed delivery: {allowed.join(", ")}</p>
            {participation.status === "counter_pending" && (
              <p className="text-xs text-amber-700">You submitted a counter-offer awaiting publisher response.</p>
            )}
            {participation.status === "pending" && showCounterOffer && (
              <div className="mt-3 space-y-2 rounded-lg border border-gray-100 p-3">
                <p className="text-xs font-semibold text-gray-500">Counter-offer (optional)</p>
                <Label>Proposed flat rate per lead ($)</Label>
                <Input
                  type="number"
                  min={0}
                  step="0.01"
                  value={counterRate}
                  onChange={(e) => setCounterRate(e.target.value)}
                  placeholder="e.g. 25.00"
                />
                <Button
                  variant="secondary"
                  disabled={!counterRate || counter.isPending}
                  onClick={() =>
                    counter.mutate(
                      {
                        id: participation.id,
                        body: {
                          compensations: [
                            {
                              kind: "flat_rate",
                              flat_amount: Number(counterRate),
                              trigger: "per_lead",
                              delivery: "leads",
                              cap_period: "one_time",
                            },
                          ],
                        },
                      },
                      {
                        onSuccess: () => {
                          toast.success("Counter-offer submitted");
                          onClose();
                        },
                        onError: (e) => toast.error(errorMessage(e)),
                      }
                    )
                  }
                >
                  Submit counter-offer
                </Button>
              </div>
            )}
          </div>
        )}

        {step === 1 && (
          <div className="space-y-3">
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
            />
            <BuyerTriggerStageFields
              participationId={participation.id}
              buyerPipelineId={pipelineId || participation.buyer_pipeline_id}
              persistChanges={false}
            />
          </div>
        )}

        {step === 2 && pipelineDelivery && (
          <div className="space-y-3">
            <SectionLabel>Return routes</SectionLabel>
            {!returnRoutesLoading && !buyerPipelineSelected && (
              <p className="text-sm text-gray-500">
                Go back to Delivery and select your destination pipeline and stage.
              </p>
            )}
            {!returnRoutesLoading && buyerPipelineSelected && !publisherPipelineConfigured && (
              <p className="text-sm text-gray-500">
                The publisher must finish pipeline delivery setup before you can add return routes.
              </p>
            )}
            {(returnRoutesLoading || (buyerPipelineSelected && publisherPipelineConfigured)) && (
              <ContractReturnRulesEditor
                side="buyer"
                buyerStages={stages ?? []}
                publisherStages={[]}
                rules={returnRoutes ?? []}
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
        )}

        {step === 3 && (
          <BuyerContractFieldMapSection
            participationId={participation.id}
            onCompleteChange={setMappingComplete}
          />
        )}

        {step === 4 && (
          <div className="space-y-3">
            <SectionLabel>CRM forward (optional)</SectionLabel>
            <p className="text-xs text-gray-400">Leads still land in inbox or pipeline first, then forward to your CRM.</p>
            <div>
              <Label>Integration</Label>
              <Select value={integrationId} onChange={(e) => setIntegrationId(Number(e.target.value))}>
                <option value={0}>None</option>
                {(connections ?? [])
                  .filter((c) => c.status === "active")
                  .map((c) => (
                    <option key={c.id} value={c.id}>
                      {c.name || c.provider_name}
                    </option>
                  ))}
              </Select>
            </div>
          </div>
        )}
      </DrawerBody>

      <DrawerFooter className="flex justify-end gap-2">
        <Button
          variant="secondary"
          disabled={decline.isPending}
          onClick={() =>
            decline.mutate(participation.id, {
              onSuccess: () => {
                toast.success("Declined");
                onClose();
              },
              onError: (e) => toast.error(errorMessage(e)),
            })
          }
        >
          Decline
        </Button>
        {participation.status === "pending" && (
          <Button
            variant="secondary"
            onClick={() => {
              setShowCounterOffer((v) => !v);
              setStep(0);
            }}
          >
            Counteroffer
          </Button>
        )}
        {step > 0 && (
          <Button variant="secondary" onClick={prevStep}>
            Back
          </Button>
        )}
        {step < 4 ? (
          <Button
            disabled={
              (step === 1 && !deliveryValid) ||
              (step === 2 && pipelineDelivery && !returnRoutesValid) ||
              (step === 3 && !mappingComplete)
            }
            onClick={nextStep}
          >
            Next
          </Button>
        ) : (
          <Button
            disabled={
              accept.isPending ||
              !mappingComplete ||
              !deliveryValid ||
              (pipelineDelivery && !returnRoutesValid)
            }
            onClick={submitAccept}
          >
            Accept contract
          </Button>
        )}
      </DrawerFooter>
    </>
  );
}
