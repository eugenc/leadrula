import { Label, Select } from "@/components/ui/input";
import { SectionLabel } from "@/components/layout/SectionLabel";
import { useIntegrationConnections } from "@/features/integrations/hooks";
import { usePipelines, useStages } from "@/features/leads/hooks";
import { PUBLISHER_DELIVERY_MODES } from "@/features/admin/contractOffer";

export function BuyerParticipationDeliveryFields({
  allowedModes,
  delivery,
  onDeliveryChange,
  pipelineId,
  onPipelineIdChange,
  stageId,
  onStageIdChange,
  webhookId,
  onWebhookIdChange,
  integrationId,
  onIntegrationIdChange,
  showIntegration = false,
}: {
  allowedModes: string[];
  delivery: string;
  onDeliveryChange: (value: string) => void;
  pipelineId: number;
  onPipelineIdChange: (value: number) => void;
  stageId: number;
  onStageIdChange: (value: number) => void;
  webhookId: number;
  onWebhookIdChange: (value: number) => void;
  integrationId: number;
  onIntegrationIdChange: (value: number) => void;
  showIntegration?: boolean;
}) {
  const modeOptions = PUBLISHER_DELIVERY_MODES.filter((m) => allowedModes.includes(m.value));
  const pipelineDelivery = delivery === "leads_pipeline";
  const { data: pipelines } = usePipelines();
  const { data: stages } = useStages(pipelineId || undefined);
  const { data: connections } = useIntegrationConnections();

  return (
    <div className="space-y-3">
      <SectionLabel>Your delivery</SectionLabel>
      <div>
        <Label>Delivery mode</Label>
        <Select value={delivery} onChange={(e) => onDeliveryChange(e.target.value)}>
          {modeOptions.map((m) => (
            <option key={m.value} value={m.value}>
              {m.label}
            </option>
          ))}
        </Select>
      </div>
      {pipelineDelivery && (
        <>
          <div>
            <Label>Destination pipeline</Label>
            <Select
              value={pipelineId}
              onChange={(e) => {
                onPipelineIdChange(Number(e.target.value));
                onStageIdChange(0);
              }}
            >
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
            <Select value={stageId} onChange={(e) => onStageIdChange(Number(e.target.value))}>
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
          <Select value={webhookId} onChange={(e) => onWebhookIdChange(Number(e.target.value))}>
            <option value={0}>Select…</option>
          </Select>
          <p className="mt-1 text-xs text-gray-400">
            Configure an outbound webhook under Integrations → Webhooks first.
          </p>
        </div>
      )}
      {showIntegration && (
        <div className="pt-2">
          <SectionLabel>CRM forward (optional)</SectionLabel>
          <p className="mb-2 text-xs text-gray-400">
            Leads still land in inbox or pipeline first, then forward to your CRM.
          </p>
          <div>
            <Label>Integration</Label>
            <Select value={integrationId} onChange={(e) => onIntegrationIdChange(Number(e.target.value))}>
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
    </div>
  );
}

export function participationDeliveryValid(
  delivery: string,
  pipelineId: number,
  stageId: number,
  webhookId: number
): boolean {
  if (delivery === "leads_pipeline") return pipelineId > 0 && stageId > 0;
  if (delivery === "webhook") return webhookId > 0;
  return true;
}
