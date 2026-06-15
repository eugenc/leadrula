import { useMemo } from "react";
import {
  useRouteIntegrations,
  useAttachRouteIntegration,
  useDetachRouteIntegration,
} from "@/features/integrations/hooks";
import { Spinner } from "@/components/ui/misc";
import { toast } from "@/store/toastStore";
import { errorMessage } from "@/lib/api";
import {
  RouteDestinationIntegrationsEditor,
  type RouteDestinationIntegrationSelection,
} from "./RouteDestinationIntegrationsEditor";

function toSelections(attached: ReturnType<typeof useRouteIntegrations>["data"]): RouteDestinationIntegrationSelection[] {
  return (attached ?? []).map((a) => ({
    connection_id: a.connection_id,
    connection_name: a.connection_name,
    provider_slug: a.provider_slug,
    route_integration_id: a.id,
    delivery_config: a.delivery_config,
  }));
}

export function RouteIntegrationsPanel({ routeId }: { routeId: number }) {
  const { data: attached, isLoading } = useRouteIntegrations(routeId);
  const attach = useAttachRouteIntegration();
  const detach = useDetachRouteIntegration();
  const selected = useMemo(() => toSelections(attached), [attached]);
  const busy = attach.isPending || detach.isPending;

  async function handleChange(next: RouteDestinationIntegrationSelection[]) {
    const removed = selected.filter((s) => !next.some((n) => n.connection_id === s.connection_id));
    const added = next.filter((n) => !selected.some((s) => s.connection_id === n.connection_id));

    try {
      for (const r of removed) {
        if (r.route_integration_id) {
          await detach.mutateAsync({ id: r.route_integration_id, routeId });
        }
      }
      for (const a of added) {
        await attach.mutateAsync({
          routeId,
          connection_id: a.connection_id,
          delivery_config: a.delivery_config,
        });
      }
      if (added.length > 0) toast.success("Integration attached");
    } catch (e) {
      toast.error(errorMessage(e));
    }
  }

  if (isLoading) return <Spinner className="h-5 w-5" />;

  return (
    <div className="space-y-4 border-t border-border pt-4 mt-4">
      <h3 className="text-sm font-semibold">Integrations</h3>
      <RouteDestinationIntegrationsEditor
        selected={selected}
        onChange={handleChange}
        disabled={busy}
      />
    </div>
  );
}
