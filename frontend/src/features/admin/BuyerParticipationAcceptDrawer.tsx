import { useState } from "react";
import { Sheet, DrawerHeader, DrawerBody, DrawerFooter } from "@/components/ui/dialog";
import { Button } from "@/components/ui/button";
import { Label, Select } from "@/components/ui/input";
import { SectionLabel } from "@/components/layout/SectionLabel";
import { toast } from "@/store/toastStore";
import { errorMessage } from "@/lib/api";
import {
  useAcceptParticipation,
  useDeclineParticipation,
  useCounterParticipation,
  useBuyerPipelines,
} from "@/features/admin/hooks";
import { Input } from "@/components/ui/input";
import { useIntegrationConnections } from "@/features/integrations/hooks";
import { useStages } from "@/features/leads/hooks";
import { PUBLISHER_DELIVERY_MODES } from "@/features/admin/contractOffer";
import { formatParticipationStatus } from "@/features/admin/contractOffer";
import type { ContractParticipation } from "@/types";

const STEPS = ["Review", "Delivery", "Integrations"] as const;

export function BuyerParticipationAcceptDrawer({
  participation,
  onClose,
}: {
  participation: ContractParticipation | null;
  onClose: () => void;
}) {
  if (!participation) return null;
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
  const [counterRate, setCounterRate] = useState("");
  const allowed = participation.allowed_delivery_modes ?? ["leads", "leads_pipeline"];
  const modeOptions = PUBLISHER_DELIVERY_MODES.filter((m) => allowed.includes(m.value));
  const [delivery, setDelivery] = useState<string>(modeOptions[0]?.value ?? "leads");
  const [pipelineId, setPipelineId] = useState(0);
  const [stageId, setStageId] = useState(0);
  const [webhookId, setWebhookId] = useState(0);
  const [integrationId, setIntegrationId] = useState(0);

  const { data: pipelines } = useBuyerPipelines(participation.buyer_id);
  const { data: stages } = useStages(pipelineId || undefined);
  const { data: connections } = useIntegrationConnections();

  const actionable = participation.status === "pending" || participation.status === "counter_pending";

  function submitAccept() {
    const body: Record<string, unknown> = { delivery };
    if (delivery === "leads_pipeline") {
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
        <div className="mb-4 flex gap-2 text-xs font-semibold text-gray-400">
          {STEPS.map((s, i) => (
            <span key={s} className={i === step ? "text-jade-700" : ""}>
              {i + 1}. {s}
            </span>
          ))}
        </div>

        {step === 0 && (
          <div className="space-y-2 text-sm text-gray-700">
            <p>Publisher: {participation.publisher_name}</p>
            <p>Lead type: {participation.lead_type || "—"}</p>
            <p>Allowed delivery: {allowed.join(", ")}</p>
            {participation.status === "counter_pending" && (
              <p className="text-xs text-amber-700">You submitted a counter-offer awaiting publisher response.</p>
            )}
            {participation.status === "pending" && (
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
            <SectionLabel>Publisher delivery</SectionLabel>
            <div>
              <Label>Delivery mode</Label>
              <Select value={delivery} onChange={(e) => setDelivery(e.target.value)}>
                {modeOptions.map((m) => (
                  <option key={m.value} value={m.value}>
                    {m.label}
                  </option>
                ))}
              </Select>
            </div>
            {delivery === "leads_pipeline" && (
              <>
                <div>
                  <Label>Pipeline</Label>
                  <Select value={pipelineId} onChange={(e) => setPipelineId(Number(e.target.value))}>
                    <option value={0}>Select…</option>
                    {(pipelines ?? []).map((p) => (
                      <option key={p.id} value={p.id}>
                        {p.name}
                      </option>
                    ))}
                  </Select>
                </div>
                <div>
                  <Label>Destination stage</Label>
                  <Select value={stageId} onChange={(e) => setStageId(Number(e.target.value))}>
                    <option value={0}>Select…</option>
                    {(stages ?? [])
                      .filter((s) => s.stage_type === "standard" || s.stage_type === "action")
                      .map((s) => (
                        <option key={s.id} value={s.id}>
                          {s.name}
                        </option>
                      ))}
                  </Select>
                </div>
              </>
            )}
            {delivery === "webhook" && (
              <div>
                <Label>Outbound webhook ID</Label>
                <Select value={webhookId} onChange={(e) => setWebhookId(Number(e.target.value))}>
                  <option value={0}>Select…</option>
                  {/* Webhook picker — user configures webhooks separately; ID entry via select placeholder */}
                </Select>
                <p className="mt-1 text-xs text-gray-400">Configure an outbound webhook under Integrations → Webhooks first.</p>
              </div>
            )}
          </div>
        )}

        {step === 2 && (
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
        {actionable && (
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
        )}
        {step > 0 && (
          <Button variant="secondary" onClick={() => setStep((s) => s - 1)}>
            Back
          </Button>
        )}
        {step < STEPS.length - 1 ? (
          <Button onClick={() => setStep((s) => s + 1)}>Next</Button>
        ) : actionable ? (
          <Button disabled={accept.isPending} onClick={submitAccept}>
            Accept contract
          </Button>
        ) : (
          <Button variant="secondary" onClick={onClose}>
            Close
          </Button>
        )}
      </DrawerFooter>
    </>
  );
}
