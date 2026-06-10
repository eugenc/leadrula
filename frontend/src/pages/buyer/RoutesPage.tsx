import { useState } from "react";
import { useBuyerRoutes } from "@/features/admin/hooks";
import { PageBody } from "@/components/layout/PageBody";
import { Table, THead, TH, TBody, TR, TD } from "@/components/ui/table";
import { Badge, Spinner, EmptyState } from "@/components/ui/misc";
import { Button } from "@/components/ui/button";
import { FormDrawer } from "@/components/ui/dialog";
import { RouteIntegrationsPanel } from "@/features/integrations/RouteIntegrationsPanel";
import type { Route } from "@/types";

function pipelineStage(pipeline?: string | null, stage?: string | null) {
  if (!pipeline) return "—";
  if (!stage) return `${pipeline} > First stage`;
  return `${pipeline} > ${stage}`;
}

function formatOrigin(r: Route) {
  if (r.origin === "source") return `Source: ${r.source_name ?? `#${r.source_id}`}`;
  return pipelineStage(r.origin_pipeline_name, r.origin_stage_name);
}

function formatTarget(r: Route) {
  return pipelineStage(r.target_pipeline_name, r.target_stage_name);
}

function deliveryCell(r: Route) {
  if (r.delivery === "leads") return "Lead";
  return <Badge variant="distributed">Pipeline</Badge>;
}

export function RoutesPage() {
  const { data: routes, isLoading } = useBuyerRoutes();
  const [integrationsRoute, setIntegrationsRoute] = useState<Route | null>(null);

  return (
    <PageBody>
      {isLoading ? (
        <Spinner className="h-6 w-6" />
      ) : (routes ?? []).length === 0 ? (
        <EmptyState title="No inbound routes yet." />
      ) : (
        <Table>
          <THead>
            <tr>
              <TH>Name</TH>
              <TH>Origin</TH>
              <TH>Target</TH>
              <TH>Delivery</TH>
              <TH>Active</TH>
              <TH className="min-w-0 w-12" />
            </tr>
          </THead>
          <TBody>
            {(routes ?? []).map((r) => (
              <TR key={r.id}>
                <TD className="font-semibold">{r.name}</TD>
                <TD>{formatOrigin(r)}</TD>
                <TD>{formatTarget(r)}</TD>
                <TD>{deliveryCell(r)}</TD>
                <TD>{r.is_active ? "Yes" : "No"}</TD>
                <TD>
                  <Button size="sm" variant="secondary" onClick={() => setIntegrationsRoute(r)}>
                    Integrations
                  </Button>
                </TD>
              </TR>
            ))}
          </TBody>
        </Table>
      )}

      <FormDrawer
        open={!!integrationsRoute}
        onClose={() => setIntegrationsRoute(null)}
        title={integrationsRoute ? `Integrations — ${integrationsRoute.name}` : "Integrations"}
        width={520}
      >
        {integrationsRoute && <RouteIntegrationsPanel routeId={integrationsRoute.id} />}
      </FormDrawer>
    </PageBody>
  );
}
