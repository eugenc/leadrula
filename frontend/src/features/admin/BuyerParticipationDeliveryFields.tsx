import { Label, Select } from "@/components/ui/input";
import { SectionLabel } from "@/components/layout/SectionLabel";
import { useIntegrationConnections } from "@/features/integrations/hooks";
import { usePipelines, useStages } from "@/features/leads/hooks";
import { PUBLISHER_DELIVERY_MODES } from "@/features/admin/contractOffer";
import { formatIntegrationConnectionLabel, INTEGRATION_CATEGORY } from "@/features/integrations/constants";
import type { IntegrationConnection } from "@/types";

export function activeCrmConnections(connections: IntegrationConnection[] | undefined) {
  return (connections ?? []).filter(
    (c) => c.status === "active" && INTEGRATION_CATEGORY[c.provider_slug] === "crm"
  );
}

export function CrmForwardWarning({
  integrationId,
  connections,
}: {
  integrationId: number;
  connections: IntegrationConnection[] | undefined;
}) {
  const crmConnections = activeCrmConnections(connections);
  if (integrationId > 0 || crmConnections.length === 0) return null;
  const names = crmConnections.map((c) => c.name).join(", ");
  return (
    <p className="rounded-lg border border-amber-100 bg-amber-50 px-3 py-2 text-xs text-amber-800">
      You have CRM integration{crmConnections.length > 1 ? "s" : ""} configured ({names}) but none selected
      for CRM forward. Leads will land in inbox or pipeline but will <span className="font-medium">not</span> sync
      to your CRM, and nothing will appear under Logs → Integrations until you link one here.
    </p>
  );
}

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
      <SectionLabel>Your distribution</SectionLabel>
      <div>
        <Label>Distribution Type</Label>
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
            <Label>Distribute to Pipeline</Label>
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
            <Label>Distribute to Stage</Label>
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
            Leads still land in inbox or pipeline first, then forward to your CRM. Delivery attempts appear under{" "}
            <span className="font-medium text-gray-500">Logs → Integrations</span>, not Webhooks.
          </p>
          <div>
            <Label>Integration</Label>
            <Select value={integrationId} onChange={(e) => onIntegrationIdChange(Number(e.target.value))}>
              <option value={0}>None</option>
              {(connections ?? [])
                .filter((c) => c.status === "active")
                .map((c) => (
                  <option key={c.id} value={c.id}>
                    {formatIntegrationConnectionLabel(c)}
                  </option>
                ))}
            </Select>
          </div>
          <CrmForwardWarning integrationId={integrationId} connections={connections} />
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

export function deliverySaveBlockReason(
  delivery: string,
  pipelineId: number,
  stageId: number,
  webhookId: number
): string | null {
  if (delivery === "leads_pipeline" && (pipelineId <= 0 || stageId <= 0)) {
    return "Select Distribute to Pipeline and Distribute to Stage.";
  }
  if (delivery === "webhook" && webhookId <= 0) {
    return "Select an outbound webhook.";
  }
  return null;
}
