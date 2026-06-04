import { useState } from "react";
import { Label, Select } from "@/components/ui/input";
import { Table, THead, TH, TBody, TR, TD } from "@/components/ui/table";
import { EmptyState } from "@/components/ui/misc";
import { useIntegrationDeliveries } from "@/features/integrations/hooks";

export function IntegrationsDeliveriesTab() {
  const [deliveryStatus, setDeliveryStatus] = useState("");
  const { data: deliveries } = useIntegrationDeliveries(deliveryStatus);

  return (
    <>
      <div className="mb-3 max-w-xs">
        <Label>Status filter</Label>
        <Select value={deliveryStatus} onChange={(e) => setDeliveryStatus(e.target.value)}>
          <option value="">All</option>
          <option value="pending">Pending</option>
          <option value="success">Success</option>
          <option value="failed">Failed</option>
          <option value="dead">Dead</option>
        </Select>
      </div>
      {(deliveries ?? []).length === 0 ? (
        <EmptyState title="No deliveries yet." />
      ) : (
        <Table>
          <THead>
            <tr>
              <TH>Lead</TH>
              <TH>Provider</TH>
              <TH>Status</TH>
              <TH>Attempts</TH>
              <TH>Error</TH>
            </tr>
          </THead>
          <TBody>
            {(deliveries ?? []).map((d) => (
              <TR key={d.id}>
                <TD>#{d.lead_id}</TD>
                <TD>{d.provider}</TD>
                <TD>{d.status}</TD>
                <TD>{d.attempts}</TD>
                <TD className="text-muted-foreground text-xs">{d.last_error ?? "—"}</TD>
              </TR>
            ))}
          </TBody>
        </Table>
      )}
    </>
  );
}
