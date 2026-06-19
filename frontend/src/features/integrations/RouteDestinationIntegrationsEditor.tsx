import { useState } from "react";
import { useIntegrationConnections } from "@/features/integrations/hooks";
import { Button } from "@/components/ui/button";
import { Label, Select } from "@/components/ui/input";
import type { IntegrationConnection } from "@/types";

export type RouteDestinationIntegrationSelection = {
  connection_id: number;
  connection_name?: string;
  provider_slug?: string;
  route_integration_id?: number;
  delivery_config?: Record<string, unknown>;
};

export function buildIntegrationDeliveryConfig(
  conn: IntegrationConnection | undefined
): Record<string, unknown> {
  const delivery_config: Record<string, unknown> = {};
  if (conn?.provider_slug === "ghl" && conn.config?.location_id) {
    delivery_config.location_id = conn.config.location_id;
  }
  return delivery_config;
}

export function RouteDestinationIntegrationsEditor({
  selected,
  onChange,
  disabled,
}: {
  selected: RouteDestinationIntegrationSelection[];
  onChange: (next: RouteDestinationIntegrationSelection[]) => void;
  disabled?: boolean;
}) {
  const { data: connections } = useIntegrationConnections();
  const [connectionId, setConnectionId] = useState(0);

  const available = (connections ?? []).filter(
    (c) =>
      c.status === "active" &&
      c.provider_slug !== "google_maps" &&
      !selected.some((s) => s.connection_id === c.id)
  );

  function addConnection() {
    if (!connectionId) return;
    const conn = connections?.find((c) => c.id === connectionId);
    onChange([
      ...selected,
      {
        connection_id: connectionId,
        connection_name: conn?.name,
        provider_slug: conn?.provider_slug,
        delivery_config: buildIntegrationDeliveryConfig(conn),
      },
    ]);
    setConnectionId(0);
  }

  function removeConnection(connectionIdToRemove: number) {
    onChange(selected.filter((s) => s.connection_id !== connectionIdToRemove));
  }

  return (
    <div className="space-y-3">
      <p className="text-sm text-muted-foreground">
        Select integrations to deliver leads to when this route runs.
      </p>
      {selected.length === 0 ? (
        <p className="text-sm text-muted-foreground">No integrations selected.</p>
      ) : (
        <ul className="space-y-2 text-sm">
          {selected.map((s) => (
            <li key={s.connection_id} className="flex items-center justify-between gap-2">
              <span>
                {s.connection_name ?? `Connection #${s.connection_id}`}{" "}
                {s.provider_slug && (
                  <span className="text-muted-foreground">({s.provider_slug})</span>
                )}
              </span>
              <Button
                variant="secondary"
                size="sm"
                disabled={disabled}
                onClick={() => removeConnection(s.connection_id)}
              >
                Remove
              </Button>
            </li>
          ))}
        </ul>
      )}
      {available.length > 0 && (
        <div className="space-y-2">
          <Label>Integration</Label>
          <Select
            value={connectionId}
            disabled={disabled}
            onChange={(e) => setConnectionId(Number(e.target.value))}
          >
            <option value={0}>Select…</option>
            {available.map((c) => (
              <option key={c.id} value={c.id}>
                {c.name} ({c.provider_slug})
              </option>
            ))}
          </Select>
          <Button size="sm" disabled={disabled || !connectionId} onClick={addConnection}>
            Add integration
          </Button>
        </div>
      )}
    </div>
  );
}
