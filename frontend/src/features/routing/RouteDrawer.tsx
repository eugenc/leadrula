import { useEffect, useState } from "react";
import {
  useSources,
  useContracts,
  useBuyerContracts,
} from "@/features/admin/hooks";
import { usePipelines, useStages } from "@/features/leads/hooks";
import { useWebhooks } from "@/features/webhooks/hooks";
import { useIntegrationConnections } from "@/features/integrations/hooks";
import { Button } from "@/components/ui/button";
import { Input, Label, Select } from "@/components/ui/input";
import { FormDrawer } from "@/components/ui/dialog";
import { toast } from "@/store/toastStore";
import { errorMessage } from "@/lib/api";
import type { Route } from "@/types";
import {
  BUYER_DESTINATIONS,
  BUYER_ORIGINS,
  DESTINATION_LABELS,
  ORIGIN_LABELS,
  PUBLISHER_DESTINATIONS,
  PUBLISHER_ORIGINS,
} from "./routeFormatters";

type Origin = Route["origin"];
type Destination = Route["destination"];

export function RouteDrawer({
  accountType,
  route,
  open,
  onClose,
  onCreate,
  onUpdate,
}: {
  accountType: "publisher" | "buyer";
  route: Route | null;
  open: boolean;
  onClose: () => void;
  onCreate: (body: Record<string, unknown>) => Promise<void>;
  onUpdate: (id: number, body: Record<string, unknown>) => Promise<void>;
}) {
  const editing = route !== null;
  const origins = accountType === "publisher" ? PUBLISHER_ORIGINS : BUYER_ORIGINS;
  const destinations = accountType === "publisher" ? PUBLISHER_DESTINATIONS : BUYER_DESTINATIONS;

  const { data: sources } = useSources();
  const { data: publisherContracts } = useContracts(accountType === "publisher");
  const { data: buyerContracts } = useBuyerContracts();
  const contracts = accountType === "publisher" ? publisherContracts : buyerContracts;
  const { data: pipelines } = usePipelines();
  const { data: webhooks } = useWebhooks();
  const { data: connections } = useIntegrationConnections();

  const [name, setName] = useState(route?.name ?? "");
  const [origin, setOrigin] = useState<Origin>(route?.origin ?? origins[0]);
  const [sourceId, setSourceId] = useState(route?.source_id ?? 0);
  const [originPipelineId, setOriginPipelineId] = useState(route?.origin_pipeline_id ?? 0);
  const [originStageId, setOriginStageId] = useState(route?.origin_stage_id ?? 0);
  const [originWebhookId, setOriginWebhookId] = useState(route?.origin_webhook_id ?? 0);
  const [originConnectionId, setOriginConnectionId] = useState(route?.origin_connection_id ?? 0);
  const [destination, setDestination] = useState<Destination>(route?.destination ?? (accountType === "publisher" ? "contract" : "pipeline"));
  const [contractId, setContractId] = useState(route?.contract_id ?? 0);
  const [delivery, setDelivery] = useState<"leads" | "leads_pipeline">(route?.delivery ?? "leads_pipeline");
  const [targetPipelineId, setTargetPipelineId] = useState(route?.target_pipeline_id ?? 0);
  const [targetStageId, setTargetStageId] = useState(route?.target_stage_id ?? 0);
  const [destWebhookId, setDestWebhookId] = useState(route?.dest_webhook_id ?? 0);
  const [saving, setSaving] = useState(false);

  useEffect(() => {
    setName(route?.name ?? "");
    setOrigin(route?.origin ?? origins[0]);
    setSourceId(route?.source_id ?? 0);
    setOriginPipelineId(route?.origin_pipeline_id ?? 0);
    setOriginStageId(route?.origin_stage_id ?? 0);
    setOriginWebhookId(route?.origin_webhook_id ?? 0);
    setOriginConnectionId(route?.origin_connection_id ?? 0);
    setDestination(route?.destination ?? (accountType === "publisher" ? "contract" : "pipeline"));
    setContractId(route?.contract_id ?? 0);
    setDelivery(route?.delivery ?? "leads_pipeline");
    setTargetPipelineId(route?.target_pipeline_id ?? 0);
    setTargetStageId(route?.target_stage_id ?? 0);
    setDestWebhookId(route?.dest_webhook_id ?? 0);
  }, [route, accountType, origins]);

  const { data: originStages } = useStages(origin === "pipeline" ? originPipelineId || undefined : undefined);
  const { data: targetStages } = useStages(
    destination === "pipeline" && delivery === "leads_pipeline" ? targetPipelineId || undefined : undefined
  );

  const selectedContract = (contracts ?? []).find((c) => c.id === contractId);
  const { data: contractBuyerStages } = useStages(
    destination === "contract" && delivery === "leads_pipeline"
      ? selectedContract?.buyer_pipeline_id ?? undefined
      : undefined
  );

  function buildBody(): Record<string, unknown> {
    const body: Record<string, unknown> = { name, origin, destination };
    if (destination === "contract" || destination === "pipeline") {
      body.delivery = delivery;
    }
    switch (origin) {
      case "source":
        body.source_id = sourceId;
        break;
      case "pipeline":
        body.origin_pipeline_id = originPipelineId;
        body.origin_stage_id = originStageId;
        break;
      case "webhook":
        body.origin_webhook_id = originWebhookId;
        break;
      case "integration":
        body.origin_connection_id = originConnectionId;
        break;
    }
    switch (destination) {
      case "contract":
        body.contract_id = contractId;
        if (delivery === "leads_pipeline") body.target_stage_id = targetStageId || null;
        break;
      case "pipeline":
        if (delivery === "leads_pipeline") {
          body.target_pipeline_id = targetPipelineId;
          body.target_stage_id = targetStageId;
        }
        break;
      case "webhook":
        body.dest_webhook_id = destWebhookId;
        break;
    }
    return body;
  }

  const originValid =
    (origin === "source" && sourceId) ||
    (origin === "pipeline" && originPipelineId && originStageId) ||
    (origin === "webhook" && originWebhookId) ||
    (origin === "integration" && originConnectionId);

  const destValid =
    (destination === "contract" && contractId) ||
    (destination === "pipeline" && (delivery === "leads" || (targetPipelineId && targetStageId))) ||
    (destination === "webhook" && destWebhookId) ||
    destination === "integration";

  const valid = name && originValid && destValid;

  async function submit() {
    setSaving(true);
    try {
      const body = buildBody();
      if (editing) {
        await onUpdate(route.id, body);
        toast.success("Route updated");
      } else {
        await onCreate(body);
        toast.success("Route created");
      }
      onClose();
    } catch (e) {
      toast.error(errorMessage(e));
    } finally {
      setSaving(false);
    }
  }

  return (
    <FormDrawer
      open={open}
      onClose={onClose}
      title={editing ? route.name : "New Route"}
      subtitle={editing ? "Edit route" : "Create a routing rule"}
      width={560}
      footer={
        <>
          <Button variant="secondary" onClick={onClose}>
            Cancel
          </Button>
          <Button disabled={!valid || saving} onClick={submit}>
            {editing ? "Save" : "Create"}
          </Button>
        </>
      }
    >
      <div className="space-y-3">
        <div>
          <Label>Name</Label>
          <Input value={name} onChange={(e) => setName(e.target.value)} placeholder="Qualified → CRM" />
        </div>
        <div>
          <Label>Origin type</Label>
          <Select value={origin} onChange={(e) => setOrigin(e.target.value as Origin)}>
            {origins.map((o) => (
              <option key={o} value={o}>
                {ORIGIN_LABELS[o]}
              </option>
            ))}
          </Select>
        </div>
        {origin === "source" && (
          <div>
            <Label>Source</Label>
            <Select value={sourceId} onChange={(e) => setSourceId(Number(e.target.value))}>
              <option value={0}>Select…</option>
              {(sources ?? []).map((s) => (
                <option key={s.id} value={s.id}>
                  {s.name} ({s.slug})
                </option>
              ))}
            </Select>
          </div>
        )}
        {origin === "pipeline" && (
          <div className="grid grid-cols-2 gap-3">
            <div>
              <Label>Pipeline</Label>
              <Select value={originPipelineId} onChange={(e) => setOriginPipelineId(Number(e.target.value))}>
                <option value={0}>Select…</option>
                {(pipelines ?? []).map((p) => (
                  <option key={p.id} value={p.id}>
                    {p.name}
                  </option>
                ))}
              </Select>
            </div>
            <div>
              <Label>Trigger stage</Label>
              <Select value={originStageId} onChange={(e) => setOriginStageId(Number(e.target.value))}>
                <option value={0}>Select…</option>
                {(originStages ?? []).map((s) => (
                  <option key={s.id} value={s.id}>
                    {s.name}
                  </option>
                ))}
              </Select>
            </div>
          </div>
        )}
        {origin === "webhook" && (
          <div>
            <Label>Webhook</Label>
            <Select value={originWebhookId} onChange={(e) => setOriginWebhookId(Number(e.target.value))}>
              <option value={0}>Select…</option>
              {(webhooks ?? []).map((w) => (
                <option key={w.id} value={w.id}>
                  {w.name}
                </option>
              ))}
            </Select>
          </div>
        )}
        {origin === "integration" && (
          <div>
            <Label>Integration</Label>
            <Select value={originConnectionId} onChange={(e) => setOriginConnectionId(Number(e.target.value))}>
              <option value={0}>Select…</option>
              {(connections ?? []).filter((c) => c.status === "active").map((c) => (
                <option key={c.id} value={c.id}>
                  {c.name} ({c.provider_slug})
                </option>
              ))}
            </Select>
          </div>
        )}
        <div>
          <Label>Destination type</Label>
          <Select value={destination} onChange={(e) => setDestination(e.target.value as Destination)}>
            {destinations.map((d) => (
              <option key={d} value={d}>
                {DESTINATION_LABELS[d]}
              </option>
            ))}
          </Select>
        </div>
        {destination === "contract" && (
          <>
            <div>
              <Label>Contract</Label>
              <Select value={contractId} onChange={(e) => setContractId(Number(e.target.value))}>
                <option value={0}>Select…</option>
                {(contracts ?? []).map((c) => (
                  <option key={c.id} value={c.id}>
                    {c.buyer_name ?? c.publisher_name ?? c.name}
                  </option>
                ))}
              </Select>
            </div>
            <div>
              <Label>Delivery</Label>
              <Select value={delivery} onChange={(e) => setDelivery(e.target.value as "leads" | "leads_pipeline")}>
                <option value="leads">Lead</option>
                <option value="leads_pipeline">Pipeline</option>
              </Select>
            </div>
            {delivery === "leads_pipeline" && (
              <div>
                <Label>Target stage</Label>
                <Select value={targetStageId} onChange={(e) => setTargetStageId(Number(e.target.value))}>
                  <option value={0}>First stage (default)</option>
                  {(contractBuyerStages ?? []).map((s) => (
                    <option key={s.id} value={s.id}>
                      {s.name}
                    </option>
                  ))}
                </Select>
              </div>
            )}
          </>
        )}
        {destination === "pipeline" && (
          <>
            <div>
              <Label>Delivery</Label>
              <Select value={delivery} onChange={(e) => setDelivery(e.target.value as "leads" | "leads_pipeline")}>
                <option value="leads">Lead</option>
                <option value="leads_pipeline">Pipeline</option>
              </Select>
            </div>
            {delivery === "leads_pipeline" && (
              <div className="grid grid-cols-2 gap-3">
                <div>
                  <Label>Pipeline</Label>
                  <Select value={targetPipelineId} onChange={(e) => setTargetPipelineId(Number(e.target.value))}>
                    <option value={0}>Select…</option>
                    {(pipelines ?? []).map((p) => (
                      <option key={p.id} value={p.id}>
                        {p.name}
                      </option>
                    ))}
                  </Select>
                </div>
                <div>
                  <Label>Target stage</Label>
                  <Select value={targetStageId} onChange={(e) => setTargetStageId(Number(e.target.value))}>
                    <option value={0}>Select…</option>
                    {(targetStages ?? []).map((s) => (
                      <option key={s.id} value={s.id}>
                        {s.name}
                      </option>
                    ))}
                  </Select>
                </div>
              </div>
            )}
          </>
        )}
        {destination === "webhook" && (
          <div>
            <Label>Webhook</Label>
            <Select value={destWebhookId} onChange={(e) => setDestWebhookId(Number(e.target.value))}>
              <option value={0}>Select…</option>
              {(webhooks ?? []).filter((w) => w.outbound_enabled).map((w) => (
                <option key={w.id} value={w.id}>
                  {w.name}
                </option>
              ))}
            </Select>
          </div>
        )}
        {destination === "integration" && (
          <p className="text-sm text-muted-foreground">
            Attach webhooks and integrations to this route after saving.
          </p>
        )}
      </div>
    </FormDrawer>
  );
}
