import { useEffect, useRef, useState } from "react";
import {
  useSources,
} from "@/features/admin/hooks";
import { usePipelines, useStages } from "@/features/leads/hooks";
import { useWebhooks } from "@/features/webhooks/hooks";
import {
  useIntegrationConnections,
  useRouteIntegrations,
  useAttachRouteIntegration,
  useDetachRouteIntegration,
} from "@/features/integrations/hooks";
import { type RouteDestinationIntegrationSelection } from "@/features/integrations/RouteDestinationIntegrationsEditor";
import { formatIntegrationConnectionLabel } from "@/features/integrations/constants";
import { Button } from "@/components/ui/button";
import { Input, Label, Select } from "@/components/ui/input";
import { FormDrawer } from "@/components/ui/dialog";
import { toast } from "@/store/toastStore";
import { errorMessage } from "@/lib/api";
import type { Route, RouteBranch } from "@/types";
import {
  BUYER_ORIGINS,
  ORIGIN_LABELS,
  PUBLISHER_ORIGINS,
} from "./routeFormatters";
import { RouteBranchesEditor } from "./RouteBranchesEditor";
import { blankBranch, branchDestinationValid, reindexBranches } from "./routeBranchUtils";

type Origin = Route["origin"];

function mapAttachedIntegrations(
  attached: {
    id: number;
    branch_position: number;
    connection_id: number;
    connection_name: string;
    provider_slug: string;
    delivery_config: Record<string, unknown>;
  }[]
): Record<number, RouteDestinationIntegrationSelection[]> {
  const out: Record<number, RouteDestinationIntegrationSelection[]> = {};
  for (const a of attached) {
    const pos = a.branch_position ?? 0;
    if (!out[pos]) out[pos] = [];
    out[pos].push({
      connection_id: a.connection_id,
      connection_name: a.connection_name,
      provider_slug: a.provider_slug,
      route_integration_id: a.id,
      delivery_config: a.delivery_config,
    });
  }
  return out;
}

async function syncBranchIntegrations(
  routeId: number,
  branches: RouteBranch[],
  initial: Record<number, RouteDestinationIntegrationSelection[]>,
  current: Record<number, RouteDestinationIntegrationSelection[]>,
  attach: ReturnType<typeof useAttachRouteIntegration>,
  detach: ReturnType<typeof useDetachRouteIntegration>
) {
  for (const branch of branches) {
    if (branch.destination !== "integration") continue;
    const pos = branch.position;
    const init = initial[pos] ?? [];
    const curr = current[pos] ?? [];
    const removed = init.filter((s) => !curr.some((n) => n.connection_id === s.connection_id));
    const added = curr.filter((n) => !init.some((s) => s.connection_id === n.connection_id));

    for (const r of removed) {
      if (r.route_integration_id) {
        await detach.mutateAsync({ id: r.route_integration_id, routeId });
      }
    }
    for (const a of added) {
      await attach.mutateAsync({
        routeId,
        branch_position: pos,
        connection_id: a.connection_id,
        delivery_config: a.delivery_config,
      });
    }
  }
}

function branchesFromRoute(route: Route | null, accountType: "publisher" | "buyer"): RouteBranch[] {
  if (route?.branches?.length) return route.branches;
  if (route) {
    return [
      {
        position: 0,
        condition_logic: "and",
        conditions: [],
        destination: route.destination,
        delivery: route.delivery,
        target_pipeline_id: route.target_pipeline_id,
        target_stage_id: route.target_stage_id,
        contract_id: route.contract_id,
        compensation_id: route.compensation_id,
        dest_webhook_id: route.dest_webhook_id ?? null,
      },
    ];
  }
  return [blankBranch(0, accountType)];
}

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
  onCreate: (body: Record<string, unknown>) => Promise<Route>;
  onUpdate: (id: number, body: Record<string, unknown>) => Promise<Route>;
}) {
  const editing = route !== null;
  const origins = accountType === "publisher" ? PUBLISHER_ORIGINS : BUYER_ORIGINS;

  const { data: sources } = useSources();
  const { data: pipelines } = usePipelines();
  const { data: webhooks } = useWebhooks();
  const { data: connections } = useIntegrationConnections();
  const { data: attachedIntegrations, isLoading: attachedLoading } = useRouteIntegrations(
    editing && open ? route.id : null
  );
  const attachIntegration = useAttachRouteIntegration();
  const detachIntegration = useDetachRouteIntegration();

  const [name, setName] = useState(route?.name ?? "");
  const [origin, setOrigin] = useState<Origin>(route?.origin ?? origins[0]);
  const [sourceId, setSourceId] = useState(route?.source_id ?? 0);
  const [originPipelineId, setOriginPipelineId] = useState(route?.origin_pipeline_id ?? 0);
  const [originStageId, setOriginStageId] = useState(route?.origin_stage_id ?? 0);
  const [originWebhookId, setOriginWebhookId] = useState(route?.origin_webhook_id ?? 0);
  const [originConnectionId, setOriginConnectionId] = useState(route?.origin_connection_id ?? 0);
  const [branches, setBranches] = useState<RouteBranch[]>(() => branchesFromRoute(route, accountType));
  const [integrationSelections, setIntegrationSelections] = useState<
    Record<number, RouteDestinationIntegrationSelection[]>
  >({});
  const initialIntegrationsRef = useRef<Record<number, RouteDestinationIntegrationSelection[]>>({});
  const [saving, setSaving] = useState(false);

  useEffect(() => {
    setName(route?.name ?? "");
    setOrigin(route?.origin ?? origins[0]);
    setSourceId(route?.source_id ?? 0);
    setOriginPipelineId(route?.origin_pipeline_id ?? 0);
    setOriginStageId(route?.origin_stage_id ?? 0);
    setOriginWebhookId(route?.origin_webhook_id ?? 0);
    setOriginConnectionId(route?.origin_connection_id ?? 0);
    setBranches(branchesFromRoute(route, accountType));
    if (!route) {
      setIntegrationSelections({});
      initialIntegrationsRef.current = {};
    }
  }, [route, accountType, origins]);

  useEffect(() => {
    if (!editing || !attachedIntegrations) return;
    const mapped = mapAttachedIntegrations(attachedIntegrations);
    setIntegrationSelections(mapped);
    initialIntegrationsRef.current = mapped;
  }, [editing, attachedIntegrations]);

  const { data: originStages } = useStages(origin === "pipeline" ? originPipelineId || undefined : undefined);

  function buildBody(): Record<string, unknown> {
    const normalized = reindexBranches(branches);
    const body: Record<string, unknown> = {
      name,
      origin,
      branches: normalized,
    };
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
    return body;
  }

  const originValid =
    (origin === "source" && sourceId) ||
    (origin === "pipeline" && originPipelineId && originStageId) ||
    (origin === "webhook" && originWebhookId) ||
    (origin === "integration" && originConnectionId);

  const branchesValid = branches.every((b) =>
    branchDestinationValid(b, (integrationSelections[b.position] ?? []).length)
  );

  const integrationsLoading = branches.some((b) => b.destination === "integration") && editing && attachedLoading;
  const showPayloadDomain = origin === "webhook" || origin === "integration" || origin === "source";
  const valid = name && originValid && branchesValid && branches.length > 0 && !integrationsLoading;

  async function submit() {
    setSaving(true);
    try {
      const body = buildBody();
      let saved: Route;
      if (editing) {
        saved = await onUpdate(route.id, body);
      } else {
        saved = await onCreate(body);
      }
      await syncBranchIntegrations(
        saved.id,
        reindexBranches(branches),
        editing ? initialIntegrationsRef.current : {},
        integrationSelections,
        attachIntegration,
        detachIntegration
      );
      toast.success(editing ? "Route updated" : "Route created");
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
                  {formatIntegrationConnectionLabel(c)}
                </option>
              ))}
            </Select>
          </div>
        )}
        <RouteBranchesEditor
          accountType={accountType}
          branches={branches}
          onChange={setBranches}
          integrationSelections={integrationSelections}
          onIntegrationSelectionsChange={setIntegrationSelections}
          integrationsLoading={integrationsLoading}
          showPayloadDomain={showPayloadDomain}
          disabled={saving}
        />
      </div>
    </FormDrawer>
  );
}
