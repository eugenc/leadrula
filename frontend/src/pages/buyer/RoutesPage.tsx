import { useState } from "react";
import {
  useBuyerRoutes,
  useCreateBuyerRoute,
  useUpdateBuyerRoute,
  useDeleteBuyerRoute,
} from "@/features/admin/hooks";
import { RouteDrawer } from "@/features/routing/RouteDrawer";
import { formatRouteOrigin, formatRouteTargetsSummary, formatRouteBranchesSummary, routeEditableByBuyer } from "@/features/routing/routeFormatters";
import { PageHeader } from "@/components/layout/PageHeader";
import { PageBody } from "@/components/layout/PageBody";
import { IconButton } from "@/components/layout/IconButton";
import { Table, THead, TH, TBody, TR, TD } from "@/components/ui/table";
import { Spinner, EmptyState, TextWithOverflowTooltip } from "@/components/ui/misc";
import { Button } from "@/components/ui/button";
import { Switch } from "@/components/ui/misc";
import { useAuthStore } from "@/store/authStore";
import { Plus, Trash2 } from "lucide-react";
import { toast } from "@/store/toastStore";
import { errorMessage } from "@/lib/api";
import type { Route } from "@/types";

function deliveryCell(r: Route) {
  if (r.destination === "integration" || r.destination === "webhook") return "—";
  if (r.delivery === "leads") return "Lead";
  return "Pipeline";
}

export function RoutesPage() {
  const isAdmin = useAuthStore((s) => s.user?.role === "admin");
  const { data: routes, isLoading } = useBuyerRoutes();
  const createRoute = useCreateBuyerRoute();
  const updateRoute = useUpdateBuyerRoute();
  const removeRoute = useDeleteBuyerRoute();

  const [drawerRoute, setDrawerRoute] = useState<Route | null | undefined>(undefined);

  const drawerOpen = drawerRoute !== undefined;

  return (
    <>
      <PageHeader
        action={
          isAdmin ? (
            <Button onClick={() => setDrawerRoute(null)}>
              <Plus className="h-4 w-4" /> New Route
            </Button>
          ) : undefined
        }
      />
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
                <TH>Routes</TH>
                <TH className="max-w-xs">Targets</TH>
                <TH>Delivery</TH>
                <TH>Active</TH>
                <TH className="min-w-0 w-12" />
              </tr>
            </THead>
            <TBody>
              {(routes ?? []).map((r) => {
                const editable = routeEditableByBuyer(r);
                const targetsSummary = formatRouteTargetsSummary(r);
                return (
                  <TR
                    key={r.id}
                    onClick={() => {
                      if (editable && isAdmin) setDrawerRoute(r);
                    }}
                  >
                    <TD className="font-semibold">{r.name}</TD>
                    <TD>{formatRouteOrigin(r)}</TD>
                    <TD className="text-sm text-muted-foreground">{formatRouteBranchesSummary(r)}</TD>
                    <TD className="max-w-xs">
                      <TextWithOverflowTooltip>{targetsSummary}</TextWithOverflowTooltip>
                    </TD>
                    <TD>{deliveryCell(r)}</TD>
                    <TD>
                      {editable && isAdmin ? (
                        <div onClick={(e) => e.stopPropagation()}>
                          <Switch
                            checked={r.is_active}
                            onChange={(v) =>
                              updateRoute.mutate(
                                { id: r.id, body: { is_active: v } },
                                { onError: (e) => toast.error(errorMessage(e)) }
                              )
                            }
                          />
                        </div>
                      ) : (
                        (r.is_active ? "Yes" : "No")
                      )}
                    </TD>
                    <TD>
                      {editable && isAdmin && (
                        <div className="flex justify-end" onClick={(e) => e.stopPropagation()}>
                          <IconButton
                            variant="danger"
                            onClick={() =>
                              removeRoute.mutate(r.id, { onError: (e) => toast.error(errorMessage(e)) })
                            }
                          >
                            <Trash2 className="h-4 w-4" />
                          </IconButton>
                        </div>
                      )}
                    </TD>
                  </TR>
                );
              })}
            </TBody>
          </Table>
        )}
      </PageBody>

      {isAdmin && (
        <RouteDrawer
          accountType="buyer"
          route={drawerRoute ?? null}
          open={drawerOpen}
          onClose={() => setDrawerRoute(undefined)}
          onCreate={(body) => createRoute.mutateAsync(body)}
          onUpdate={(id, body) => updateRoute.mutateAsync({ id, body })}
        />
      )}
    </>
  );
}
