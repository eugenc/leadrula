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
import { useIntegrationConnections } from "@/features/integrations/hooks";
import { useCustomFields } from "@/features/leads/hooks";
import { CreateCustomFieldDrawer } from "@/features/admin/CreateCustomFieldDrawer";
import { BuiltinCustomFieldSelect } from "@/features/admin/BuiltinCustomFieldSelect";
import { slugFieldKey } from "@/features/admin/customFieldConstants";
import { buildPayloadSuggestions, builtinFieldLabel, PAYLOAD_MAP_BUILTIN_FIELDS } from "@/features/leads/csvMapping";
import { payloadValuePreview } from "@/features/intake/payloadKeys";
import { PageHeader } from "@/components/layout/PageHeader";
import { PageBody } from "@/components/layout/PageBody";
import { IconButton } from "@/components/layout/IconButton";
import { Table, THead, TH, TBody, TR, TD } from "@/components/ui/table";
import { Button } from "@/components/ui/button";
import { Input, Label, Select } from "@/components/ui/input";
import { Switch, Spinner, EmptyState, Badge } from "@/components/ui/misc";
import { FormDrawer } from "@/components/ui/dialog";
import { format } from "date-fns";
import { ArrowRightLeft, Copy, Plus, RefreshCw, Trash2 } from "lucide-react";
import { IntakeLogSection } from "@/features/intake/IntakeLogTable";
import { toast } from "@/store/toastStore";
import { errorMessage, apiBaseURL } from "@/lib/api";
import type { Route, RouteFieldMapEntry, Source, SourceType } from "@/types";

const BUILTINS = PAYLOAD_MAP_BUILTIN_FIELDS;

function slugify(name: string) {
  return name
    .toLowerCase()
    .replace(/[^a-z0-9]+/g, "-")
    .replace(/^-|-$/g, "");
}

function sourceFieldKeySets(entries: { source_key: string; target_type: string }[]) {
  const ignoredKeys = new Set(
    entries.filter((e) => e.target_type === "ignore").map((e) => e.source_key)
  );
  const mappedKeys = new Set(
    entries.filter((e) => e.target_type !== "ignore").map((e) => e.source_key)
  );
  return { ignoredKeys, mappedKeys };
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
                <TH>Endpoint</TH>
                <TH>Active</TH>
                <TH className="min-w-0 w-12" />
              </tr>
            </THead>
            <TBody>
              {(sources ?? []).map((s) => (
                <TR key={s.id} onClick={() => openEditDrawer(s)}>
                  <TD className="font-semibold">{s.name}</TD>
                  <TD className="font-mono">{s.slug}</TD>
                  <TD className="font-mono text-xs text-gray-500">
                    <div>POST {apiBaseURL}/api/v1/sources/{s.slug}</div>
                    <Badge className="mt-1">{s.api_key_required ? "Auth required" : "Open"}</Badge>
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
  const { data: connections } = useIntegrationConnections();
  const twilioConnections = (connections ?? []).filter((c) => c.provider_slug === "twilio");

  const [type, setType] = useState<SourceType>(source?.type ?? "webhook");
  const [name, setName] = useState(source?.name ?? "");
  const [slug, setSlug] = useState(source?.slug ?? "");
  const [slugTouched, setSlugTouched] = useState(false);
  const [isActive, setIsActive] = useState(source?.is_active ?? true);
  const [apiKeyRequired, setApiKeyRequired] = useState(source?.api_key_required ?? true);
  const [trackingNumber, setTrackingNumber] = useState(source?.tracking_number ?? "");
  const [twilioConnId, setTwilioConnId] = useState(source?.integration_connection_id ?? 0);
  const [payloadEnabled, setPayloadEnabled] = useState(source?.payload_enabled ?? false);
  const [requirePreload, setRequirePreload] = useState(source?.require_preload ?? false);

  useEffect(() => {
    setType(source?.type ?? "webhook");
    setName(source?.name ?? "");
    setSlug(source?.slug ?? "");
    setSlugTouched(false);
    setIsActive(source?.is_active ?? true);
    setApiKeyRequired(source?.api_key_required ?? true);
    setTrackingNumber(source?.tracking_number ?? "");
    setTwilioConnId(source?.integration_connection_id ?? 0);
    setPayloadEnabled(source?.payload_enabled ?? false);
    setRequirePreload(source?.require_preload ?? false);
  }, [source]);

  const isCall = type === "call";

  function callBody() {
    return {
      tracking_number: trackingNumber.trim(),
      integration_connection_id: twilioConnId || null,
      payload_enabled: payloadEnabled,
      require_preload: payloadEnabled ? requirePreload : false,
    };
  }

  function submit() {
    if (editing) {
      const body: Record<string, unknown> = {
        name,
        slug,
        is_active: isActive,
        api_key_required: apiKeyRequired,
      };
      if (source.type === "call") body.call = callBody();
      update.mutate(
        { id: source.id, body },
        {
          onSuccess: () => {
            toast.success("Source updated");
            onClose();
          },
          onError: (e) => toast.error(errorMessage(e)),
        }
      );
    } else {
      const body: Record<string, unknown> = { name, slug, type, api_key_required: apiKeyRequired };
      if (isCall) body.call = callBody();
      create.mutate(body, {
        onSuccess: (src) => {
          onCreated?.(src);
          onClose();
        },
        onError: (e) => toast.error(errorMessage(e)),
      });
    }
  }

  const valid = !!name && !!slug && !!type && (!isCall || !!trackingNumber.trim());
  const saving = create.isPending || update.isPending;

  return (
    <FormDrawer
      open
      onClose={onClose}
      title={editing ? source.name : "New Source"}
      subtitle={editing ? "Edit source" : "Create a lead source"}
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
        {editing ? (
          <div>
            <Label>Type</Label>
            <p className="text-sm text-gray-700">
              <Badge>{source.type === "webhook" ? "Webhook" : source.type === "call" ? "Call" : source.type}</Badge>
            </p>
          </div>
        ) : (
          <div>
            <Label>Type</Label>
            <Select value={type} onChange={(e) => setType(e.target.value as SourceType)}>
              <option value="webhook">Webhook</option>
              <option value="call">Call (inbound phone)</option>
            </Select>
          </div>
        )}
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
        {slug && !isCall && (
          <p className="text-xs font-mono text-gray-500">
            POST {apiBaseURL}/api/v1/sources/{slug}
          </p>
        )}
        {isCall && (
          <>
            <div>
              <Label>Twilio account</Label>
              <Select value={twilioConnId} onChange={(e) => setTwilioConnId(Number(e.target.value))}>
                <option value={0}>Select Twilio connection…</option>
                {twilioConnections.map((c) => (
                  <option key={c.id} value={c.id}>
                    {c.name}
                  </option>
                ))}
              </Select>
              {twilioConnections.length === 0 && (
                <p className="mt-1 text-xs text-amber-700">
                  Connect Twilio on the Integrations page first.
                </p>
              )}
            </div>
            <div>
              <Label>Tracking number (E.164)</Label>
              <Input
                value={trackingNumber}
                onChange={(e) => setTrackingNumber(e.target.value)}
                placeholder="+14155550123"
              />
              <p className="mt-1 text-xs text-gray-500">
                Point this Twilio number's Voice webhook at:
              </p>
              <p className="text-xs font-mono text-gray-500">
                POST {apiBaseURL}/webhooks/twilio/voice
              </p>
            </div>
            <div className="flex items-center justify-between">
              <div>
                <Label>Accept inbound payload</Label>
                <p className="text-xs text-gray-500">Preload caller/lead data via the calls API before the call.</p>
              </div>
              <Switch checked={payloadEnabled} onChange={setPayloadEnabled} />
            </div>
            {payloadEnabled && (
              <div className="flex items-center justify-between">
                <div>
                  <Label>Require preload</Label>
                  <p className="text-xs text-gray-500">Only route calls that have a matching preload payload.</p>
                </div>
                <Switch checked={requirePreload} onChange={setRequirePreload} />
              </div>
            )}
          </>
        )}
        {!isCall && (
          <>
            <div className="flex items-center justify-between">
              <div>
                <Label>Require API key</Label>
                <p className="text-xs text-gray-500">Publisher API key Bearer token on inbound POST requests</p>
              </div>
              <Switch checked={apiKeyRequired} onChange={setApiKeyRequired} />
            </div>
            {!apiKeyRequired && (
              <p className="text-xs text-amber-700">Anyone with the source URL can POST payloads.</p>
            )}
          </>
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
  const [rowTargets, setRowTargets] = useState<Record<string, string>>({});
  const [createFieldOpen, setCreateFieldOpen] = useState(false);

  const createField = useCreateField();

  const payload = sample?.payload ?? null;
  const mappableKeys = payload ? mappablePayloadKeys(payload) : [];
  const { ignoredKeys, mappedKeys } = useMemo(
    () => sourceFieldKeySets(entries ?? []),
    [entries]
  );
  const unmappedKeys = mappableKeys.filter((k) => !mappedKeys.has(k) && !ignoredKeys.has(k));

  const suggestions = useMemo(
    () => buildPayloadSuggestions(unmappedKeys, customFields ?? []),
    [unmappedKeys, customFields]
  );

  useEffect(() => {
    setRowTargets((prev) => {
      let changed = false;
      const next = { ...prev };
      for (const [key, target] of Object.entries(suggestions)) {
        if (!(key in prev)) {
          next[key] = target;
          changed = true;
        }
      }
      return changed ? next : prev;
    });
  }, [suggestions]);

  const buyerRoutes = useMemo(
    () =>
      (routes ?? []).filter(
        (r) =>
          r.origin === "source" &&
          r.source_id === sourceId &&
          r.destination === "contract" &&
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

  function rowTarget(key: string) {
    return rowTargets[key] ?? suggestions[key] ?? "first_name";
  }

  function isSuggested(key: string) {
    return !!suggestions[key] && rowTarget(key) === suggestions[key];
  }

  function ignoreMapping(key: string) {
    add.mutate(
      { sourceId, body: { source_key: key, target_type: "ignore" } },
      { onError: (e) => toast.error(errorMessage(e)) }
    );
  }

  function prefillMapping(key: string) {
    setSourceKey(key);
    setTarget(rowTargets[key] ?? suggestions[key] ?? "first_name");
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
                POST {apiBaseURL}/api/v1/sources/{slug}
              </p>
            </div>
          ) : (
            <>
              {sample?.received_at && (
                <p className="text-xs text-gray-400">
                  Received {format(new Date(sample.received_at), "MMM d, yyyy h:mma")}
                </p>
              )}
              <pre className="max-h-48 overflow-auto rounded-md border border-gray-100 bg-gray-50 p-3 font-mono text-xs">
                {JSON.stringify(payload, null, 2)}
              </pre>
              {mappableKeys.length > 0 && (
                <div className="space-y-1.5">
                  <p className="text-xs text-gray-500">Payload keys — click to select</p>
                  <div className="flex flex-wrap gap-1.5">
                    {mappableKeys.map((k) => {
                      const mapped = mappedKeys.has(k);
                      const ignored = ignoredKeys.has(k);
                      return (
                        <button
                          key={k}
                          type="button"
                          onClick={() => prefillMapping(k)}
                          className={
                            mapped || ignored
                              ? "rounded-full border border-gray-200 bg-gray-50 px-2 py-0.5 font-mono text-xs text-gray-400"
                              : "rounded-full border border-jade-200 bg-jade-50 px-2 py-0.5 font-mono text-xs text-gray-800 hover:border-jade-300 hover:bg-jade-100"
                          }
                        >
                          {k}
                          {mapped ? " ✓" : ignored ? " —" : ""}
                        </button>
                      );
                    })}
                  </div>
                </div>
              )}
              {unmappedKeys.length > 0 && (
                <div className="space-y-3">
                  <div>
                    <Label>Unmapped keys</Label>
                    <p className="text-xs text-gray-500">
                      These payload keys are not mapped yet. Create a custom field or map to a built-in.
                    </p>
                  </div>
                  {unmappedKeys.map((k) => (
                    <div key={k} className="rounded-md border border-gray-100 p-3">
                      <div className="mb-2">
                        <span className="font-mono text-sm font-medium">{k}</span>
                        <p className="mt-0.5 truncate text-xs text-gray-400">
                          {payloadValuePreview(payload, k)}
                        </p>
                      </div>
                      <div className="flex flex-wrap items-end gap-2">
                        <div className="min-w-[11rem] flex-1">
                          <BuiltinCustomFieldSelect
                            value={rowTarget(k)}
                            onChange={(v) => setRowTargets((t) => ({ ...t, [k]: v }))}
                            customFields={customFields ?? []}
                            builtins={BUILTINS}
                            label="Lead field"
                            onAddCustomField={() => openCreateForKey(k)}
                          />
                          {isSuggested(k) && (
                            <p className="mt-0.5 text-xs text-gray-400">Suggested</p>
                          )}
                        </div>
                        <Button size="sm" onClick={() => addMapping(k, rowTarget(k))}>
                          Map
                        </Button>
                        <Button size="sm" variant="secondary" onClick={() => openCreateForKey(k)}>
                          Create custom field
                        </Button>
                        <Button size="sm" variant="secondary" onClick={() => ignoreMapping(k)}>
                          Ignore
                        </Button>
                      </div>
                    </div>
                  ))}
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
                {e.target_type === "ignore" ? (
                  <Badge variant="default">Ignored</Badge>
                ) : e.target_type === "builtin" ? (
                  <Badge variant="review">{builtinFieldLabel(e.builtin_field ?? "")}</Badge>
                ) : (
                  <Badge variant="distributed">
                    {customFieldName(e.custom_field_id) ?? `custom #${e.custom_field_id}`}
                  </Badge>
                )}
              </span>
              <div className="flex items-center gap-2">
                {e.target_type !== "ignore" && (
                  <IconButton
                    aria-label="Map to another field"
                    title="Map to another field"
                    onClick={() => prefillMapping(e.source_key)}
                  >
                    <Copy className="h-4 w-4" />
                  </IconButton>
                )}
                {e.target_type === "ignore" && (
                  <Button size="sm" variant="secondary" onClick={() => prefillMapping(e.source_key)}>
                    Map
                  </Button>
                )}
                <IconButton variant="danger" onClick={() => remove.mutate(e.id)}>
                  <Trash2 className="h-4 w-4" />
                </IconButton>
              </div>
            </div>
          ))}
        </div>

        <div className="grid grid-cols-[1fr_1fr_auto] items-end gap-2">
          <div className="min-w-0">
            <Label>Payload key</Label>
            <Input value={sourceKey} onChange={(e) => setSourceKey(e.target.value)} placeholder="phone_number" />
          </div>
          <div className="min-w-0">
            <BuiltinCustomFieldSelect
              value={target}
              onChange={setTarget}
              customFields={customFields ?? []}
              builtins={BUILTINS}
              label="Lead field"
              onAddCustomField={() => setCreateFieldOpen(true)}
            />
          </div>
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

        <div className="space-y-2 border-t border-gray-100 pt-4">
          <div className="flex items-center justify-between">
            <Label>Recent intake</Label>
            <Button
              size="sm"
              variant="secondary"
              onClick={() => navigate(`/p/log?source=${encodeURIComponent(slug)}`)}
            >
              View all in Log
            </Button>
          </div>
          <IntakeLogSection
            source="publisher"
            readOnly={false}
            emptyTitle="No intake yet for this source."
            sourceSlug={slug}
            compact
          />
        </div>
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
