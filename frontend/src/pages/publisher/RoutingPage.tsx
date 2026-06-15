import { useEffect, useState } from "react";
import { useLocation, useNavigate } from "react-router-dom";
import { useQueryClient } from "@tanstack/react-query";
import {
  useRoutes,
  useCreateRoute,
  useUpdateRoute,
  useDeleteRoute,
  useRouteFieldMap,
  useRouteFieldMapOptions,
  useAddRouteFieldMap,
  useDeleteRouteFieldMap,
  useCreateBuyerRouteField,
  useCreateField,
} from "@/features/admin/hooks";
import { RouteDrawer } from "@/features/routing/RouteDrawer";
import { formatRouteOrigin, formatRouteTargetsSummary, formatRouteBranchesSummary } from "@/features/routing/routeFormatters";
import { PageHeader } from "@/components/layout/PageHeader";
import { PageBody } from "@/components/layout/PageBody";
import { IconButton } from "@/components/layout/IconButton";
import { Table, THead, TH, TBody, TR, TD } from "@/components/ui/table";
import { Button } from "@/components/ui/button";
import { Switch, Spinner, EmptyState, Badge, TextWithOverflowTooltip } from "@/components/ui/misc";
import { FormDrawer } from "@/components/ui/dialog";
import { ArrowRightLeft, Plus, Trash2 } from "lucide-react";
import { toast } from "@/store/toastStore";
import { errorMessage } from "@/lib/api";
import { RouteIntegrationsPanel } from "@/features/integrations/RouteIntegrationsPanel";
import { CreateCustomFieldDrawer } from "@/features/admin/CreateCustomFieldDrawer";
import { BuiltinCustomFieldSelect } from "@/features/admin/BuiltinCustomFieldSelect";
import { slugFieldKey } from "@/features/admin/customFieldConstants";
import { MAP_BUILTIN_FIELDS } from "@/features/leads/csvMapping";
import type { CustomField, Route, RouteFieldMapEntry } from "@/types";

const BUILTINS = MAP_BUILTIN_FIELDS;

function deliveryCell(r: Route) {
  if (r.destination === "integration" || r.destination === "webhook") return "—";
  if (r.delivery === "leads") return "Lead";
  return <Badge variant="distributed">Pipeline</Badge>;
}

export function RoutingPage() {
  const location = useLocation();
  const navigate = useNavigate();
  const [drawerRoute, setDrawerRoute] = useState<Route | null | undefined>(undefined);
  const [routeMapFor, setRouteMapFor] = useState<Route | null>(null);

  const { data: routes, isLoading: routesLoading } = useRoutes();
  const createRoute = useCreateRoute();
  const updateRoute = useUpdateRoute();
  const removeRoute = useDeleteRoute();

  useEffect(() => {
    const openId = (location.state as { openRouteFieldMapId?: number } | null)?.openRouteFieldMapId;
    if (!openId || !routes?.length) return;
    const route = routes.find((r) => r.id === openId);
    if (route) {
      setDrawerRoute(undefined);
      setRouteMapFor(route);
    }
    navigate(location.pathname, { replace: true, state: {} });
  }, [location.state, location.pathname, routes, navigate]);

  const drawerOpen = drawerRoute !== undefined;

  return (
    <>
      <PageHeader
        action={
          <Button onClick={() => setDrawerRoute(null)}>
            <Plus className="h-4 w-4" /> New Route
          </Button>
        }
      />
      <PageBody>
        {routesLoading ? (
          <Spinner className="h-6 w-6" />
        ) : (routes ?? []).length === 0 ? (
          <EmptyState title="No routes yet." />
        ) : (
          <Table>
            <THead>
              <tr>
                <TH>Name</TH>
                <TH>Buyer</TH>
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
                const targetsSummary = formatRouteTargetsSummary(r);
                return (
                <TR key={r.id} onClick={() => setDrawerRoute(r)}>
                  <TD className="font-semibold">{r.name}</TD>
                  <TD>{r.buyer_name ?? "—"}</TD>
                  <TD>{formatRouteOrigin(r)}</TD>
                  <TD className="text-sm text-muted-foreground">{formatRouteBranchesSummary(r)}</TD>
                  <TD className="max-w-xs">
                    <TextWithOverflowTooltip>{targetsSummary}</TextWithOverflowTooltip>
                  </TD>
                  <TD>{deliveryCell(r)}</TD>
                  <TD>
                    <div onClick={(e) => e.stopPropagation()}>
                      <Switch
                        checked={r.is_active}
                        onChange={(v) => updateRoute.mutate({ id: r.id, body: { is_active: v } })}
                      />
                    </div>
                  </TD>
                  <TD>
                    <div className="flex justify-end gap-2" onClick={(e) => e.stopPropagation()}>
                      {r.destination === "contract" && (
                        <IconButton aria-label="Field mapping" onClick={() => setRouteMapFor(r)}>
                          <ArrowRightLeft className="h-4 w-4" />
                        </IconButton>
                      )}
                      <IconButton
                        variant="danger"
                        onClick={() => removeRoute.mutate(r.id, { onError: (e) => toast.error(errorMessage(e)) })}
                      >
                        <Trash2 className="h-4 w-4" />
                      </IconButton>
                    </div>
                  </TD>
                </TR>
              );
              })}
            </TBody>
          </Table>
        )}
      </PageBody>

      <RouteDrawer
        accountType="publisher"
        route={drawerRoute ?? null}
        open={drawerOpen}
        onClose={() => setDrawerRoute(undefined)}
        onCreate={(body) => createRoute.mutateAsync(body)}
        onUpdate={(id, body) => updateRoute.mutateAsync({ id, body })}
      />
      <RouteFieldMapDrawer route={routeMapFor} open={!!routeMapFor} onClose={() => setRouteMapFor(null)} />
    </>
  );
}

function RouteFieldMapDrawer({
  route,
  open,
  onClose,
}: {
  route: Route | null;
  open: boolean;
  onClose: () => void;
}) {
  if (!open || !route) return null;
  return <RouteFieldMapContent route={route} onClose={onClose} />;
}

function RouteFieldMapContent({ route, onClose }: { route: Route; onClose: () => void }) {
  const qc = useQueryClient();
  const { data: entries } = useRouteFieldMap(route.id);
  const { data: options, isLoading: optionsLoading } = useRouteFieldMapOptions(route.id);
  const add = useAddRouteFieldMap();
  const remove = useDeleteRouteFieldMap();
  const createField = useCreateField();
  const createBuyerField = useCreateBuyerRouteField(route.id);
  const [src, setSrc] = useState("first_name");
  const [dst, setDst] = useState("first_name");
  const [createFieldSide, setCreateFieldSide] = useState<"src" | "dst" | null>(null);

  const publisherFields = options?.publisher_fields ?? [];
  const buyerFields = options?.buyer_fields ?? [];
  const buyerName = options?.buyer_name ?? route.buyer_name ?? "Buyer";

  function fieldBody(prefix: string, val: string) {
    if (val.startsWith("cf:")) {
      return { [`${prefix}_type`]: "custom", [`${prefix}_custom_field_id`]: Number(val.slice(3)) };
    }
    return { [`${prefix}_type`]: "builtin", [`${prefix}_builtin`]: val };
  }

  function submit() {
    add.mutate(
      { routeId: route.id, body: { ...fieldBody("src", src), ...fieldBody("dst", dst) } },
      {
        onSuccess: () => toast.success("Mapping added"),
        onError: (e) => toast.error(errorMessage(e)),
      }
    );
  }

  function defaultNameForSide(side: "src" | "dst"): string {
    const val = side === "src" ? src : dst;
    if (val.startsWith("cf:")) {
      const id = Number(val.slice(3));
      const fields = side === "src" ? publisherFields : buyerFields;
      return fields.find((f) => f.id === id)?.name ?? "";
    }
    return val.replace(/_/g, " ");
  }

  function addRouteMapping(srcVal: string, dstVal: string) {
    add.mutate(
      { routeId: route.id, body: { ...fieldBody("src", srcVal), ...fieldBody("dst", dstVal) } },
      {
        onSuccess: () => toast.success("Mapping added"),
        onError: (e) => toast.error(errorMessage(e)),
      }
    );
  }

  function onFieldCreated(field: CustomField) {
    const val = `cf:${field.id}`;
    const side = createFieldSide;
    const srcVal = side === "src" ? val : src;
    const dstVal = side === "dst" ? val : dst;
    if (side === "src") setSrc(val);
    else if (side === "dst") setDst(val);
    setCreateFieldSide(null);
    qc.invalidateQueries({ queryKey: ["route-field-map-options", route.id] });
    if (side) addRouteMapping(srcVal, dstVal);
    return field;
  }

  function fieldLabel(
    type: "builtin" | "custom",
    builtin: string | null,
    customId: number | null,
    label: string | null | undefined,
    fields: CustomField[]
  ) {
    if (label) return label;
    if (type === "builtin" && builtin) return builtin;
    if (type === "custom" && customId) {
      return fields.find((f) => f.id === customId)?.name ?? `custom #${customId}`;
    }
    return "—";
  }

  function renderEntry(e: RouteFieldMapEntry) {
    const srcName = fieldLabel(e.src_type, e.src_builtin, e.src_custom_field_id, e.src_label, publisherFields);
    const dstName = fieldLabel(e.dst_type, e.dst_builtin, e.dst_custom_field_id, e.dst_label, buyerFields);
    return (
      <>
        <span className="text-gray-500">Publisher:</span> {srcName}
        {" → "}
        <span className="text-gray-500">Buyer:</span> {dstName}
      </>
    );
  }

  return (
    <FormDrawer
      open
      onClose={onClose}
      title={`Field Map — ${buyerName}`}
      subtitle="Publisher → buyer field mapping"
      width={560}
    >
      {optionsLoading ? (
        <Spinner className="h-6 w-6" />
      ) : (
        <div className="space-y-4">
          <FieldMapList
            entries={entries ?? []}
            onDelete={(id) => remove.mutate(id, { onError: (e) => toast.error(errorMessage(e)) })}
            renderEntry={renderEntry}
          />
          {buyerFields.length === 0 && (
            <p className="text-sm text-gray-400">
              Buyer has no custom fields yet — use + Add custom field… or map built-in fields.
            </p>
          )}
          <div className="grid grid-cols-[1fr_1fr_auto] items-end gap-2">
            <BuiltinCustomFieldSelect
              value={src}
              onChange={setSrc}
              customFields={publisherFields}
              builtins={BUILTINS}
              label="Publisher field"
              onAddCustomField={() => setCreateFieldSide("src")}
            />
            <BuiltinCustomFieldSelect
              value={dst}
              onChange={setDst}
              customFields={buyerFields}
              builtins={BUILTINS}
              label="Buyer field"
              onAddCustomField={() => setCreateFieldSide("dst")}
            />
            <Button onClick={submit} disabled={add.isPending}>
              <Plus className="h-4 w-4" />
            </Button>
          </div>
          <RouteIntegrationsPanel routeId={route.id} />
        </div>
      )}
      <CreateCustomFieldDrawer
        open={createFieldSide !== null}
        onClose={() => setCreateFieldSide(null)}
        defaultName={createFieldSide ? defaultNameForSide(createFieldSide) : ""}
        defaultFieldKey={createFieldSide ? slugFieldKey(defaultNameForSide(createFieldSide)) : ""}
        subtitle={
          createFieldSide === "src"
            ? "Publisher field"
            : createFieldSide === "dst"
              ? `Buyer field (${buyerName})`
              : undefined
        }
        isPending={createField.isPending || createBuyerField.isPending}
        onSubmit={(body) =>
          createFieldSide === "dst"
            ? createBuyerField.mutateAsync(body).then(onFieldCreated)
            : createField.mutateAsync(body).then(onFieldCreated)
        }
      />
    </FormDrawer>
  );
}

function FieldMapList<T extends { id: number }>({
  entries,
  onDelete,
  renderEntry,
}: {
  entries: T[];
  onDelete: (id: number) => void;
  renderEntry: (e: T) => React.ReactNode;
}) {
  return (
    <div className="space-y-1">
      {entries.length === 0 && <p className="text-sm text-gray-400">No mappings yet.</p>}
      {entries.map((e) => (
        <div
          key={e.id}
          className="flex items-center justify-between rounded-md border border-gray-100 px-3 py-2 text-sm"
        >
          <span>{renderEntry(e)}</span>
          <IconButton variant="danger" onClick={() => onDelete(e.id)}>
            <Trash2 className="h-4 w-4" />
          </IconButton>
        </div>
      ))}
    </div>
  );
}
