import { useEffect, useState } from "react";
import { useQueryClient } from "@tanstack/react-query";
import {
  useSources,
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
  useContracts,
} from "@/features/admin/hooks";
import { usePipelines, useStages } from "@/features/leads/hooks";
import { CreateCustomFieldDrawer } from "@/features/admin/CreateCustomFieldDrawer";
import { BuiltinCustomFieldSelect } from "@/features/admin/BuiltinCustomFieldSelect";
import { slugFieldKey } from "@/features/admin/customFieldConstants";
import { PageHeader } from "@/components/layout/PageHeader";
import { PageBody } from "@/components/layout/PageBody";
import { IconButton } from "@/components/layout/IconButton";
import { Table, THead, TH, TBody, TR, TD } from "@/components/ui/table";
import { Button } from "@/components/ui/button";
import { Input, Label, Select } from "@/components/ui/input";
import { Switch, Spinner, EmptyState, Badge } from "@/components/ui/misc";
import { FormDrawer } from "@/components/ui/dialog";
import { ArrowRightLeft, Plus, Trash2 } from "lucide-react";
import { toast } from "@/store/toastStore";
import { apiError } from "@/lib/api";
import type { CustomField, Route, RouteFieldMapEntry } from "@/types";

const BUILTINS = ["first_name", "last_name", "phone", "email", "address", "city", "state", "zip"];

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
  if (r.destination === "publisher" && r.delivery === "leads") return "Publisher";
  return pipelineStage(r.target_pipeline_name, r.target_stage_name);
}

function deliveryCell(r: Route) {
  if (r.delivery === "leads") return "Lead";
  return <Badge variant="distributed">Pipeline</Badge>;
}

export function RoutingPage() {
  const [drawerRoute, setDrawerRoute] = useState<Route | null | undefined>(undefined);
  const [routeMapFor, setRouteMapFor] = useState<Route | null>(null);

  const { data: routes, isLoading: routesLoading } = useRoutes();
  const updateRoute = useUpdateRoute();
  const removeRoute = useDeleteRoute();

  const drawerOpen = drawerRoute !== undefined;

  function openRouteDrawer(route: Route | null) {
    setRouteMapFor(null);
    setDrawerRoute(route);
  }

  function openFieldMapDrawer(route: Route) {
    setDrawerRoute(undefined);
    setRouteMapFor(route);
  }

  return (
    <>
      <PageHeader
        action={
          <Button onClick={() => openRouteDrawer(null)}>
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
                <TH>Target</TH>
                <TH>Delivery</TH>
                <TH>Active</TH>
                <TH />
              </tr>
            </THead>
            <TBody>
              {(routes ?? []).map((r) => (
                <TR key={r.id} onClick={() => openRouteDrawer(r)}>
                  <TD className="font-semibold">{r.name}</TD>
                  <TD>{r.buyer_name ?? "—"}</TD>
                  <TD>{formatOrigin(r)}</TD>
                  <TD>{formatTarget(r)}</TD>
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
                      {r.destination === "buyer" && (
                        <IconButton aria-label="Field mapping" onClick={() => openFieldMapDrawer(r)}>
                          <ArrowRightLeft className="h-4 w-4" />
                        </IconButton>
                      )}
                      <IconButton
                        variant="danger"
                        onClick={() => removeRoute.mutate(r.id, { onError: (e) => toast.error(apiError(e).message) })}
                      >
                        <Trash2 className="h-4 w-4" />
                      </IconButton>
                    </div>
                  </TD>
                </TR>
              ))}
            </TBody>
          </Table>
        )}
      </PageBody>

      <RouteDrawer route={drawerRoute ?? null} open={drawerOpen} onClose={() => setDrawerRoute(undefined)} />
      <RouteFieldMapDrawer route={routeMapFor} open={!!routeMapFor} onClose={() => setRouteMapFor(null)} />
    </>
  );
}

function RouteDrawer({ route, open, onClose }: { route: Route | null; open: boolean; onClose: () => void }) {
  return <RouteDrawerContent route={route} open={open} onClose={onClose} />;
}

function RouteDrawerContent({
  route,
  open,
  onClose,
}: {
  route: Route | null;
  open: boolean;
  onClose: () => void;
}) {
  const editing = route !== null;
  const { data: sources } = useSources();
  const { data: contracts } = useContracts();
  const { data: pipelines } = usePipelines();
  const create = useCreateRoute();
  const update = useUpdateRoute();

  const [name, setName] = useState(route?.name ?? "");
  const [origin, setOrigin] = useState<"source" | "pipeline">(route?.origin ?? "source");
  const [sourceId, setSourceId] = useState(route?.source_id ?? 0);
  const [originPipelineId, setOriginPipelineId] = useState(route?.origin_pipeline_id ?? 0);
  const [originStageId, setOriginStageId] = useState(route?.origin_stage_id ?? 0);
  const [destination, setDestination] = useState<"publisher" | "buyer">(route?.destination ?? "buyer");
  const [contractId, setContractId] = useState(route?.contract_id ?? 0);
  const [delivery, setDelivery] = useState<"leads" | "leads_pipeline">(route?.delivery ?? "leads_pipeline");
  const [targetPipelineId, setTargetPipelineId] = useState(route?.target_pipeline_id ?? 0);
  const [targetStageId, setTargetStageId] = useState(route?.target_stage_id ?? 0);
  const [buyerStageId, setBuyerStageId] = useState(route?.target_stage_id ?? 0);

  useEffect(() => {
    setName(route?.name ?? "");
    setOrigin(route?.origin ?? "source");
    setSourceId(route?.source_id ?? 0);
    setOriginPipelineId(route?.origin_pipeline_id ?? 0);
    setOriginStageId(route?.origin_stage_id ?? 0);
    setDestination(route?.destination ?? "buyer");
    setContractId(route?.contract_id ?? 0);
    setDelivery(route?.delivery ?? "leads_pipeline");
    setTargetPipelineId(route?.target_pipeline_id ?? 0);
    setTargetStageId(route?.target_stage_id ?? 0);
    setBuyerStageId(route?.target_stage_id ?? 0);
  }, [route]);

  const { data: originStages } = useStages(origin === "pipeline" ? originPipelineId || undefined : undefined);
  const { data: pubStages } = useStages(
    destination === "publisher" && delivery === "leads_pipeline" ? targetPipelineId || undefined : undefined
  );

  const selectedContract = (contracts ?? []).find((c) => c.id === contractId);
  const { data: buyerStages } = useStages(
    destination === "buyer" && delivery === "leads_pipeline" ? selectedContract?.buyer_pipeline_id : undefined
  );

  const publisherDestAllowed = origin === "source";

  function buildBody(): Record<string, unknown> {
    const body: Record<string, unknown> = { name, origin, destination, delivery };
    if (origin === "source") body.source_id = sourceId;
    else {
      body.origin_pipeline_id = originPipelineId;
      body.origin_stage_id = originStageId;
    }
    if (destination === "buyer") {
      body.contract_id = contractId;
      if (delivery === "leads_pipeline") body.target_stage_id = buyerStageId || null;
    } else if (delivery === "leads_pipeline") {
      body.target_pipeline_id = targetPipelineId;
      body.target_stage_id = targetStageId;
    }
    return body;
  }

  function submit() {
    const body = buildBody();
    if (editing) {
      update.mutate(
        { id: route.id, body },
        {
          onSuccess: () => {
            toast.success("Route updated");
            onClose();
          },
          onError: (e) => toast.error(apiError(e).message),
        }
      );
    } else {
      create.mutate(body, {
        onSuccess: () => {
          toast.success("Route created");
          onClose();
        },
        onError: (e) => toast.error(apiError(e).message),
      });
    }
  }

  const valid =
    name &&
    ((origin === "source" && sourceId) || (origin === "pipeline" && originPipelineId && originStageId)) &&
    (destination === "buyer" ? contractId : delivery === "leads" || (targetPipelineId && targetStageId));

  const saving = create.isPending || update.isPending;

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
            <Input value={name} onChange={(e) => setName(e.target.value)} placeholder="Roofing → Buyer" />
          </div>
          <div>
            <Label>Origin type</Label>
            <Select value={origin} onChange={(e) => setOrigin(e.target.value as "source" | "pipeline")}>
              <option value="source">Webhook source</option>
              <option value="pipeline">Publisher pipeline stage</option>
            </Select>
          </div>
          {origin === "source" ? (
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
          ) : (
            <div className="grid grid-cols-2 gap-3">
              <div>
                <Label>Publisher pipeline</Label>
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
          <div>
            <Label>Destination</Label>
            <Select value={destination} onChange={(e) => setDestination(e.target.value as "publisher" | "buyer")}>
              {publisherDestAllowed && <option value="publisher">Publisher (in-house)</option>}
              <option value="buyer">Buyer (via contract)</option>
            </Select>
          </div>
          {destination === "buyer" && (
            <div>
              <Label>Contract</Label>
              <Select value={contractId} onChange={(e) => setContractId(Number(e.target.value))}>
                <option value={0}>Select…</option>
                {(contracts ?? []).map((c) => (
                  <option key={c.id} value={c.id}>
                    {c.buyer_name ?? `Buyer #${c.buyer_id}`}
                  </option>
                ))}
              </Select>
            </div>
          )}
          <div>
            <Label>Delivery</Label>
            <Select value={delivery} onChange={(e) => setDelivery(e.target.value as "leads" | "leads_pipeline")}>
              <option value="leads">Lead</option>
              <option value="leads_pipeline">Pipeline</option>
            </Select>
          </div>
          {destination === "publisher" && delivery === "leads_pipeline" && (
            <div className="grid grid-cols-2 gap-3">
              <div>
                <Label>Publisher pipeline</Label>
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
                  {(pubStages ?? []).map((s) => (
                    <option key={s.id} value={s.id}>
                      {s.name}
                    </option>
                  ))}
                </Select>
              </div>
            </div>
          )}
          {destination === "buyer" && delivery === "leads_pipeline" && (
            <div>
              <Label>Buyer delivery stage</Label>
              <Select value={buyerStageId} onChange={(e) => setBuyerStageId(Number(e.target.value))}>
                <option value={0}>First stage (default)</option>
                {(buyerStages ?? []).map((s) => (
                  <option key={s.id} value={s.id}>
                    {s.name}
                  </option>
                ))}
              </Select>
            </div>
          )}
      </div>
    </FormDrawer>
  );
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
        onError: (e) => toast.error(apiError(e).message),
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

  function onFieldCreated(field: CustomField) {
    const val = `cf:${field.id}`;
    if (createFieldSide === "src") setSrc(val);
    else if (createFieldSide === "dst") setDst(val);
    setCreateFieldSide(null);
    qc.invalidateQueries({ queryKey: ["route-field-map-options", route.id] });
    return field;
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
            onDelete={(id) => remove.mutate(id, { onError: (e) => toast.error(apiError(e).message) })}
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
