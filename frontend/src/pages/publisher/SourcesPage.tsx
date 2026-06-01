import { useEffect, useMemo, useState } from "react";
import { useNavigate } from "react-router-dom";
import { useQueries } from "@tanstack/react-query";
import {
  useSources,
  useCreateSource,
  useUpdateSource,
  useDeleteSource,
  useSourceFieldMap,
  useSourceSamplePayload,
  useAddSourceFieldMap,
  useDeleteSourceFieldMap,
  useCreateField,
  useRoutes,
} from "@/features/admin/hooks";
import { get } from "@/lib/api";
import { useCustomFields } from "@/features/leads/hooks";
import { CreateCustomFieldDrawer } from "@/features/admin/CreateCustomFieldDrawer";
import { BuiltinCustomFieldSelect } from "@/features/admin/BuiltinCustomFieldSelect";
import { slugFieldKey } from "@/features/admin/customFieldConstants";
import { PageHeader } from "@/components/layout/PageHeader";
import { PageBody } from "@/components/layout/PageBody";
import { IconButton } from "@/components/layout/IconButton";
import { Table, THead, TH, TBody, TR, TD } from "@/components/ui/table";
import { Button } from "@/components/ui/button";
import { Input, Label } from "@/components/ui/input";
import { Switch, Spinner, EmptyState, Badge } from "@/components/ui/misc";
import { FormDrawer } from "@/components/ui/dialog";
import { format } from "date-fns";
import { ArrowRightLeft, Plus, RefreshCw, Trash2 } from "lucide-react";
import { toast } from "@/store/toastStore";
import { errorMessage } from "@/lib/api";
import type { Route, RouteFieldMapEntry, Source } from "@/types";

const BUILTINS = ["first_name", "last_name", "phone", "email", "address", "city", "state", "zip"];
const API_URL = import.meta.env.VITE_API_URL ?? "http://localhost:8080";

function slugify(name: string) {
  return name
    .toLowerCase()
    .replace(/[^a-z0-9]+/g, "-")
    .replace(/^-|-$/g, "");
}

function mappedSourceKeys(entries: { source_key: string }[]): Set<string> {
  return new Set(entries.map((e) => e.source_key));
}

export function SourcesPage() {
  const [drawerSource, setDrawerSource] = useState<Source | null | undefined>(undefined);
  const [mapFor, setMapFor] = useState<{ id: number; slug: string } | null>(null);

  const { data: sources, isLoading } = useSources();
  const update = useUpdateSource();
  const remove = useDeleteSource();

  const drawerOpen = drawerSource !== undefined;

  function openEditDrawer(source: Source | null) {
    setMapFor(null);
    setDrawerSource(source);
  }

  function openMapDrawer(id: number, slug: string) {
    setDrawerSource(undefined);
    setMapFor({ id, slug });
  }

  return (
    <>
      <PageHeader
        action={
          <Button onClick={() => openEditDrawer(null)}>
            <Plus className="h-4 w-4" /> New Source
          </Button>
        }
      />
      <PageBody>
        {isLoading ? (
          <Spinner className="h-6 w-6" />
        ) : (sources ?? []).length === 0 ? (
          <EmptyState title="No sources yet." />
        ) : (
          <Table>
            <THead>
              <tr>
                <TH>Name</TH>
                <TH>Slug</TH>
                <TH>Webhook</TH>
                <TH>Active</TH>
                <TH />
              </tr>
            </THead>
            <TBody>
              {(sources ?? []).map((s) => (
                <TR key={s.id} onClick={() => openEditDrawer(s)}>
                  <TD className="font-semibold">{s.name}</TD>
                  <TD className="font-mono">{s.slug}</TD>
                  <TD className="font-mono text-xs text-gray-500">
                    POST {API_URL}/api/v1/sources/{s.slug}
                  </TD>
                  <TD>
                    <div onClick={(e) => e.stopPropagation()}>
                      <Switch
                        checked={s.is_active}
                        onChange={(v) => update.mutate({ id: s.id, body: { is_active: v } })}
                      />
                    </div>
                  </TD>
                  <TD>
                    <div className="flex justify-end gap-2" onClick={(e) => e.stopPropagation()}>
                      <IconButton
                        aria-label="Payload mapping"
                        onClick={() => openMapDrawer(s.id, s.slug)}
                      >
                        <ArrowRightLeft className="h-4 w-4" />
                      </IconButton>
                      <IconButton
                        variant="danger"
                        onClick={() => remove.mutate(s.id, { onError: (e) => toast.error(errorMessage(e)) })}
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

      <SourceDrawer
        source={drawerSource ?? null}
        open={drawerOpen}
        onClose={() => setDrawerSource(undefined)}
        onCreated={(src) => {
          toast.success("Source created");
          openMapDrawer(src.id, src.slug);
        }}
      />
      <SourceFieldMapDrawer
        sourceId={mapFor?.id ?? null}
        slug={mapFor?.slug ?? ""}
        open={!!mapFor}
        onClose={() => setMapFor(null)}
      />
    </>
  );
}

function SourceDrawer({
  source,
  open,
  onClose,
  onCreated,
}: {
  source: Source | null;
  open: boolean;
  onClose: () => void;
  onCreated?: (src: Source) => void;
}) {
  if (!open) return null;
  return <SourceDrawerContent source={source} onClose={onClose} onCreated={onCreated} />;
}

function SourceDrawerContent({
  source,
  onClose,
  onCreated,
}: {
  source: Source | null;
  onClose: () => void;
  onCreated?: (src: Source) => void;
}) {
  const editing = source !== null;
  const create = useCreateSource();
  const update = useUpdateSource();

  const [name, setName] = useState(source?.name ?? "");
  const [slug, setSlug] = useState(source?.slug ?? "");
  const [slugTouched, setSlugTouched] = useState(false);
  const [isActive, setIsActive] = useState(source?.is_active ?? true);

  useEffect(() => {
    setName(source?.name ?? "");
    setSlug(source?.slug ?? "");
    setSlugTouched(false);
    setIsActive(source?.is_active ?? true);
  }, [source]);

  function submit() {
    if (editing) {
      update.mutate(
        { id: source.id, body: { name, slug, is_active: isActive } },
        {
          onSuccess: () => {
            toast.success("Source updated");
            onClose();
          },
          onError: (e) => toast.error(errorMessage(e)),
        }
      );
    } else {
      create.mutate(
        { name, slug },
        {
          onSuccess: (src) => {
            onCreated?.(src);
            onClose();
          },
          onError: (e) => toast.error(errorMessage(e)),
        }
      );
    }
  }

  const valid = !!name && !!slug;
  const saving = create.isPending || update.isPending;

  return (
    <FormDrawer
      open
      onClose={onClose}
      title={editing ? source.name : "New Source"}
      subtitle={editing ? "Edit source" : "Create a webhook source"}
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
          <Input
            value={name}
            onChange={(e) => {
              setName(e.target.value);
              if (!editing && !slugTouched) setSlug(slugify(e.target.value));
            }}
            placeholder="Roofing GTA"
          />
        </div>
        <div>
          <Label>Slug (URL path)</Label>
          <Input
            value={slug}
            onChange={(e) => {
              setSlugTouched(true);
              setSlug(e.target.value);
            }}
            placeholder="roofing-gta"
          />
        </div>
        {slug && (
          <p className="text-xs font-mono text-gray-500">
            POST {API_URL}/api/v1/sources/{slug}
          </p>
        )}
        {editing && (
          <div className="flex items-center justify-between">
            <Label>Active</Label>
            <Switch checked={isActive} onChange={setIsActive} />
          </div>
        )}
      </div>
    </FormDrawer>
  );
}

function mappablePayloadKeys(payload: Record<string, unknown>): string[] {
  const keys: string[] = [];
  for (const k of Object.keys(payload)) {
    if (k !== "custom") keys.push(k);
  }
  const custom = payload.custom;
  if (custom && typeof custom === "object" && !Array.isArray(custom)) {
    for (const k of Object.keys(custom as Record<string, unknown>)) {
      keys.push(k);
    }
  }
  return keys;
}

function SourceFieldMapDrawer({
  sourceId,
  slug,
  open,
  onClose,
}: {
  sourceId: number | null;
  slug: string;
  open: boolean;
  onClose: () => void;
}) {
  if (!open || sourceId === null) return null;
  return <SourceFieldMapContent sourceId={sourceId} slug={slug} onClose={onClose} />;
}

function SourceFieldMapContent({
  sourceId,
  slug,
  onClose,
}: {
  sourceId: number;
  slug: string;
  onClose: () => void;
}) {
  const navigate = useNavigate();
  const { data: entries } = useSourceFieldMap(sourceId);
  const { data: sample, isLoading: sampleLoading, refetch } = useSourceSamplePayload(sourceId, true);
  const { data: customFields } = useCustomFields();
  const { data: routes } = useRoutes();
  const add = useAddSourceFieldMap();
  const remove = useDeleteSourceFieldMap();
  const [sourceKey, setSourceKey] = useState("");
  const [target, setTarget] = useState("first_name");
  const [createFieldOpen, setCreateFieldOpen] = useState(false);

  const createField = useCreateField();

  const payload = sample?.payload ?? null;
  const mappableKeys = payload ? mappablePayloadKeys(payload) : [];
  const mappedKeys = mappedSourceKeys(entries ?? []);
  const unmappedKeys = mappableKeys.filter((k) => !mappedKeys.has(k));

  const buyerRoutes = useMemo(
    () =>
      (routes ?? []).filter(
        (r) =>
          r.origin === "source" &&
          r.source_id === sourceId &&
          r.destination === "buyer" &&
          r.is_active
      ),
    [routes, sourceId]
  );

  const routeMapQueries = useQueries({
    queries: buyerRoutes.map((r) => ({
      queryKey: ["route-field-map", r.id],
      queryFn: () => get<RouteFieldMapEntry[]>(`/publisher/routes/${r.id}/field-map`),
    })),
  });

  const publisherCustomIdsInSource = useMemo(
    () =>
      new Set(
        (entries ?? [])
          .filter((e) => e.target_type === "custom" && e.custom_field_id != null)
          .map((e) => e.custom_field_id as number)
      ),
    [entries]
  );

  const routeBridge = useMemo(() => {
    return buyerRoutes
      .map((route, i) => {
        const routeEntries = routeMapQueries[i]?.data ?? [];
        const mappedOnRoute = new Set(
          routeEntries
            .filter((e) => e.src_type === "custom" && e.src_custom_field_id != null)
            .map((e) => e.src_custom_field_id as number)
        );
        const unmappedCount = [...publisherCustomIdsInSource].filter((id) => !mappedOnRoute.has(id)).length;
        return { route, unmappedCount };
      })
      .filter((x) => x.unmappedCount > 0);
  }, [buyerRoutes, routeMapQueries, publisherCustomIdsInSource]);

  function customFieldName(id: number | null): string | null {
    if (!id) return null;
    return (customFields ?? []).find((f) => f.id === id)?.name ?? null;
  }

  function addMapping(key: string, targetVal: string) {
    const isCustom = targetVal.startsWith("cf:");
    const body: Record<string, unknown> = isCustom
      ? { source_key: key, target_type: "custom", custom_field_id: Number(targetVal.slice(3)) }
      : { source_key: key, target_type: "builtin", builtin_field: targetVal };
    add.mutate(
      { sourceId, body },
      {
        onSuccess: () => setSourceKey(""),
        onError: (e) => toast.error(errorMessage(e)),
      }
    );
  }

  function submit() {
    if (!sourceKey) return;
    addMapping(sourceKey, target);
  }

  function openCreateForKey(key: string) {
    setSourceKey(key);
    setCreateFieldOpen(true);
  }

  function openBuyerRouteMap(route: Route) {
    onClose();
    navigate("/p/routing", { state: { openRouteFieldMapId: route.id } });
  }

  return (
    <FormDrawer
      open
      onClose={onClose}
      title="Payload Field Mapping"
      subtitle={slug}
      width={560}
    >
      <div className="space-y-4">
        <div className="space-y-2">
          <div className="flex items-center justify-between">
            <Label>Latest webhook payload</Label>
            <Button size="sm" variant="secondary" onClick={() => refetch()}>
              <RefreshCw className="h-3.5 w-3.5" /> Refresh
            </Button>
          </div>
          {sampleLoading ? (
            <Spinner className="h-5 w-5" />
          ) : !payload ? (
            <div className="rounded-md border border-gray-100 bg-gray-50 px-3 py-2 text-sm text-gray-500">
              <p>No webhook received yet.</p>
              <p className="mt-1 font-mono text-xs">
                POST {API_URL}/api/v1/sources/{slug}
              </p>
            </div>
          ) : (
            <>
              {sample?.received_at && (
                <p className="text-xs text-gray-400">
                  Received {format(new Date(sample.received_at), "MMM d, yyyy h:mma")}
                </p>
              )}
              <pre className="max-h-48 overflow-auto rounded-md border border-gray-100 bg-gray-50 p-3 font-mono text-xs text-gray-800">
                {JSON.stringify(payload, null, 2)}
              </pre>
              {mappableKeys.length > 0 && (
                <div className="space-y-1.5">
                  <p className="text-xs text-gray-500">Payload keys — click to select</p>
                  <div className="flex flex-wrap gap-1.5">
                    {mappableKeys.map((k) => {
                      const mapped = mappedKeys.has(k);
                      return (
                        <button
                          key={k}
                          type="button"
                          onClick={() => setSourceKey(k)}
                          className={
                            mapped
                              ? "rounded-full border border-gray-200 bg-gray-50 px-2 py-0.5 font-mono text-xs text-gray-400"
                              : "rounded-full border border-amber-200 bg-amber-50 px-2 py-0.5 font-mono text-xs text-amber-900 hover:border-teal-300 hover:bg-teal-50"
                          }
                        >
                          {k}
                          {mapped ? " ✓" : ""}
                        </button>
                      );
                    })}
                  </div>
                </div>
              )}
              {unmappedKeys.length > 0 && (
                <div className="rounded-md border border-amber-100 bg-amber-50/50 p-3 space-y-2">
                  <Label className="text-amber-900">Unmapped keys</Label>
                  <p className="text-xs text-amber-800/80">
                    These payload keys are not mapped yet. Create a custom field or map to a built-in.
                  </p>
                  <div className="flex flex-col gap-2">
                    {unmappedKeys.map((k) => (
                      <div key={k} className="flex flex-wrap items-center gap-2">
                        <span className="font-mono text-xs text-gray-800">{k}</span>
                        <Button
                          size="sm"
                          variant="secondary"
                          onClick={() => {
                            setSourceKey(k);
                            setTarget("first_name");
                          }}
                        >
                          Map…
                        </Button>
                        <Button size="sm" onClick={() => openCreateForKey(k)}>
                          Create custom field
                        </Button>
                      </div>
                    ))}
                  </div>
                </div>
              )}
            </>
          )}
        </div>

        <div className="space-y-1">
          <Label>Mappings</Label>
          {(entries ?? []).length === 0 && <p className="text-sm text-gray-400">No mappings yet.</p>}
          {(entries ?? []).map((e) => (
            <div
              key={e.id}
              className="flex items-center justify-between rounded-md border border-gray-100 px-3 py-2 text-sm"
            >
              <span>
                <span className="font-mono">{e.source_key}</span> →{" "}
                {e.target_type === "builtin" ? (
                  <Badge variant="review">{e.builtin_field}</Badge>
                ) : (
                  <Badge variant="distributed">
                    {customFieldName(e.custom_field_id) ?? `custom #${e.custom_field_id}`}
                  </Badge>
                )}
              </span>
              <IconButton variant="danger" onClick={() => remove.mutate(e.id)}>
                <Trash2 className="h-4 w-4" />
              </IconButton>
            </div>
          ))}
        </div>

        <div className="grid grid-cols-[1fr_1fr_auto] items-end gap-2">
          <div>
            <Label>Payload key</Label>
            <Input value={sourceKey} onChange={(e) => setSourceKey(e.target.value)} placeholder="phone_number" />
          </div>
          <BuiltinCustomFieldSelect
            value={target}
            onChange={setTarget}
            customFields={customFields ?? []}
            builtins={BUILTINS}
            label="Lead field"
            onAddCustomField={() => setCreateFieldOpen(true)}
          />
          <Button onClick={submit} disabled={!sourceKey}>
            <Plus className="h-4 w-4" />
          </Button>
        </div>

        {routeBridge.length > 0 && (
          <div className="rounded-md border border-gray-100 bg-gray-50 p-3 space-y-2">
            <Label>Buyer route mapping</Label>
            <p className="text-xs text-gray-500">
              Publisher custom fields mapped above still need a publisher → buyer field map on each route.
            </p>
            {routeBridge.map(({ route, unmappedCount }) => (
              <div key={route.id} className="flex items-center justify-between gap-2 text-sm">
                <span>
                  {unmappedCount} field{unmappedCount === 1 ? "" : "s"} not mapped to{" "}
                  <span className="font-medium">{route.buyer_name ?? "buyer"}</span>
                </span>
                <Button size="sm" variant="secondary" onClick={() => openBuyerRouteMap(route)}>
                  Open route map
                </Button>
              </div>
            ))}
          </div>
        )}
      </div>
      <CreateCustomFieldDrawer
        open={createFieldOpen}
        onClose={() => setCreateFieldOpen(false)}
        defaultName={sourceKey.replace(/_/g, " ")}
        defaultFieldKey={sourceKey ? slugFieldKey(sourceKey) : ""}
        subtitle={sourceKey ? `Payload key: ${sourceKey}` : undefined}
        isPending={createField.isPending}
        onSubmit={(body) =>
          createField.mutateAsync(body).then((field) => {
            const val = `cf:${field.id}`;
            setTarget(val);
            if (sourceKey) addMapping(sourceKey, val);
            return field;
          })
        }
      />
    </FormDrawer>
  );
}
