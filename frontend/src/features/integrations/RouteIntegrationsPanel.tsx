import { useState } from "react";
import {
  useIntegrationConnections,
  useRouteIntegrations,
  useAttachRouteIntegration,
  useDetachRouteIntegration,
} from "@/features/integrations/hooks";
import { Button } from "@/components/ui/button";
import { Label, Select, Input } from "@/components/ui/input";
import { Spinner } from "@/components/ui/misc";
import { toast } from "@/store/toastStore";
import { errorMessage } from "@/lib/api";

export function RouteIntegrationsPanel({ routeId }: { routeId: number }) {
  const { data: connections } = useIntegrationConnections();
  const { data: attached, isLoading } = useRouteIntegrations(routeId);
  const attach = useAttachRouteIntegration();
  const detach = useDetachRouteIntegration();
  const [connectionId, setConnectionId] = useState(0);
  const [pipelineId, setPipelineId] = useState("");
  const [stageId, setStageId] = useState("");

  const available = (connections ?? []).filter(
    (c) => c.status === "active" && !(attached ?? []).some((a) => a.connection_id === c.id)
  );

  function doAttach() {
    if (!connectionId) return;
    const conn = connections?.find((c) => c.id === connectionId);
    const delivery_config: Record<string, unknown> = {};
    if (conn?.provider_slug === "ghl" && conn.config?.location_id) {
      delivery_config.location_id = conn.config.location_id;
    }
    if (pipelineId) delivery_config.pipeline_id = pipelineId;
    if (stageId) delivery_config.stage_id = stageId;
    attach.mutate(
      { routeId, connection_id: connectionId, delivery_config },
      {
        onSuccess: () => {
          toast.success("Integration attached");
          setConnectionId(0);
          setPipelineId("");
          setStageId("");
        },
        onError: (e) => toast.error(errorMessage(e)),
      }
    );
  }

  if (isLoading) return <Spinner className="h-5 w-5" />;

  return (
    <div className="space-y-4 border-t border-border pt-4 mt-4">
      <h3 className="text-sm font-semibold">Integrations</h3>
      {(attached ?? []).length === 0 ? (
        <p className="text-sm text-muted-foreground">No integrations on this route.</p>
      ) : (
        <ul className="space-y-2 text-sm">
          {(attached ?? []).map((a) => (
            <li key={a.id} className="flex items-center justify-between gap-2">
              <span>
                {a.connection_name} <span className="text-muted-foreground">({a.provider_slug})</span>
              </span>
              <Button
                variant="secondary"
                size="sm"
                onClick={() =>
                  detach.mutate(
                    { id: a.id, routeId },
                    { onError: (e) => toast.error(errorMessage(e)) }
                  )
                }
              >
                Remove
              </Button>
            </li>
          ))}
        </ul>
      )}
      {available.length > 0 && (
        <div className="space-y-2">
          <Label>Attach connection</Label>
          <Select value={connectionId} onChange={(e) => setConnectionId(Number(e.target.value))}>
            <option value={0}>Select…</option>
            {available.map((c) => (
              <option key={c.id} value={c.id}>
                {c.name} ({c.provider_slug})
              </option>
            ))}
          </Select>
          <div className="grid grid-cols-2 gap-2">
            <Input
              placeholder="Pipeline ID (optional)"
              value={pipelineId}
              onChange={(e) => setPipelineId(e.target.value)}
            />
            <Input
              placeholder="Stage ID (optional)"
              value={stageId}
              onChange={(e) => setStageId(e.target.value)}
            />
          </div>
          <Button size="sm" disabled={!connectionId || attach.isPending} onClick={doAttach}>
            Attach
          </Button>
        </div>
      )}
    </div>
  );
}
