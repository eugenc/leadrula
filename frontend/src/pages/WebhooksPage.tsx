import { useEffect, useState } from "react";
import {
  useWebhooks,
  useCreateWebhook,
  useUpdateWebhook,
  useDeleteWebhook,
  useRotateWebhookSecret,
  useRotateWebhookOutboundSecret,
  useWebhookEvents,
  useCreateWebhookEvent,
  useDeleteWebhookEvent,
  useWebhookFieldMap,
  useWebhookSamplePayload,
  useAddWebhookFieldMap,
  useDeleteWebhookFieldMap,
  useWebhookDeliveries,
  useWebhookOutboundTriggers,
  useCreateWebhookOutboundTrigger,
  useUpdateWebhookOutboundTrigger,
  useDeleteWebhookOutboundTrigger,
} from "@/features/webhooks/hooks";
import { useCustomFields, usePipelines, useStages } from "@/features/leads/hooks";
import { CreateCustomFieldDrawer } from "@/features/admin/CreateCustomFieldDrawer";
import { BuiltinCustomFieldSelect } from "@/features/admin/BuiltinCustomFieldSelect";
import { useCreateField } from "@/features/admin/hooks";
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
import { ArrowRightLeft, Copy, KeyRound, Plus, RefreshCw, Trash2, Zap } from "lucide-react";
import { toast } from "@/store/toastStore";
import { errorMessage } from "@/lib/api";
import type { Webhook, WebhookEvent, WebhookOutboundTrigger, OutboundTriggerEvent, ResponseMapEntry } from "@/types";

const API_URL = import.meta.env.VITE_API_URL ?? "http://localhost:8080";
const BUILTINS = [
  "first_name", "last_name", "phone", "email", "address", "city", "state", "zip",
  "source", "external_id", "action_at", "disqualification_reason_id",
];

function slugify(name: string) {
  return name.toLowerCase().replace(/[^a-z0-9]+/g, "-").replace(/^-|-$/g, "");
}

function mappedKeys(entries: { source_key: string }[]): Set<string> {
  return new Set(entries.map((e) => e.source_key));
}

function mappablePayloadKeys(payload: Record<string, unknown>): string[] {
  const keys: string[] = [];
  for (const k of Object.keys(payload)) {
    if (k !== "custom") keys.push(k);
  }
  const custom = payload.custom;
  if (custom && typeof custom === "object" && !Array.isArray(custom)) {
    for (const k of Object.keys(custom as Record<string, unknown>)) keys.push(k);
  }
  return keys;
}

export function WebhooksPage() {
  const [drawerWebhook, setDrawerWebhook] = useState<Webhook | null | undefined>(undefined);
  const [detailFor, setDetailFor] = useState<Webhook | null>(null);

  const { data: webhooks, isLoading } = useWebhooks();
  const update = useUpdateWebhook();
  const remove = useDeleteWebhook();

  const drawerOpen = drawerWebhook !== undefined;

  return (
    <>
      <PageHeader
        action={
          <Button onClick={() => setDrawerWebhook(null)}>
            <Plus className="h-4 w-4" /> New Webhook
          </Button>
        }
      />
      <PageBody>
        {isLoading ? (
          <Spinner className="h-6 w-6" />
        ) : (webhooks ?? []).length === 0 ? (
          <EmptyState title="No webhooks yet." />
        ) : (
          <Table>
            <THead>
              <tr>
                <TH>Name</TH>
                <TH>Slug</TH>
                <TH>Direction</TH>
                <TH>Endpoint</TH>
                <TH>Active</TH>
                <TH />
              </tr>
            </THead>
            <TBody>
              {(webhooks ?? []).map((w) => (
                <TR key={w.id} onClick={() => setDetailFor(w)}>
                  <TD className="font-semibold">{w.name}</TD>
                  <TD className="font-mono text-xs">{w.slug}</TD>
                  <TD>
                    <div className="flex gap-1">
                      {w.inbound_enabled && <Badge>Inbound</Badge>}
                      {w.outbound_enabled && <Badge>Outbound</Badge>}
                    </div>
                  </TD>
                  <TD className="font-mono text-xs text-gray-500">
                    {w.inbound_enabled ? `POST ${API_URL}/api/v1/webhooks/${w.slug}` : "—"}
                  </TD>
                  <TD>
                    <div onClick={(e) => e.stopPropagation()}>
                      <Switch
                        checked={w.is_active}
                        onChange={(v) => update.mutate({ id: w.id, body: { is_active: v } })}
                      />
                    </div>
                  </TD>
                  <TD>
                    <div className="flex justify-end gap-2" onClick={(e) => e.stopPropagation()}>
                      <IconButton aria-label="Configure" onClick={() => setDetailFor(w)}>
                        <ArrowRightLeft className="h-4 w-4" />
                      </IconButton>
                      <IconButton
                        variant="danger"
                        onClick={() => remove.mutate(w.id, { onError: (e) => toast.error(errorMessage(e)) })}
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

      <WebhookDrawer
        webhook={drawerWebhook ?? null}
        open={drawerOpen}
        onClose={() => setDrawerWebhook(undefined)}
        onCreated={(wb, secret) => {
          toast.success("Webhook created");
          navigator.clipboard.writeText(secret);
          toast.success("Secret copied to clipboard");
          setDetailFor(wb);
        }}
      />
      <WebhookDetailDrawer webhook={detailFor} open={!!detailFor} onClose={() => setDetailFor(null)} />
    </>
  );
}

function WebhookDrawer({
  webhook,
  open,
  onClose,
  onCreated,
}: {
  webhook: Webhook | null;
  open: boolean;
  onClose: () => void;
  onCreated?: (wb: Webhook, secret: string) => void;
}) {
  if (!open) return null;
  const editing = webhook !== null;
  const create = useCreateWebhook();
  const update = useUpdateWebhook();
  const rotate = useRotateWebhookSecret();

  const [name, setName] = useState(webhook?.name ?? "");
  const [slug, setSlug] = useState(webhook?.slug ?? "");
  const [slugTouched, setSlugTouched] = useState(false);
  const [isActive, setIsActive] = useState(webhook?.is_active ?? true);
  const [inboundEnabled, setInboundEnabled] = useState(webhook?.inbound_enabled ?? true);
  const [outboundEnabled, setOutboundEnabled] = useState(webhook?.outbound_enabled ?? false);
  const [outboundURL, setOutboundURL] = useState(webhook?.outbound_url ?? "");
  const [newSecret, setNewSecret] = useState<string | null>(null);
  const rotateOutbound = useRotateWebhookOutboundSecret();

  useEffect(() => {
    setName(webhook?.name ?? "");
    setSlug(webhook?.slug ?? "");
    setSlugTouched(false);
    setIsActive(webhook?.is_active ?? true);
    setInboundEnabled(webhook?.inbound_enabled ?? true);
    setOutboundEnabled(webhook?.outbound_enabled ?? false);
    setOutboundURL(webhook?.outbound_url ?? "");
    setNewSecret(null);
  }, [webhook]);

  function submit() {
    if (editing && webhook) {
      update.mutate(
        {
          id: webhook.id,
          body: {
            name, slug, is_active: isActive,
            inbound_enabled: inboundEnabled,
            outbound_enabled: outboundEnabled,
            outbound_url: outboundURL || null,
          },
        },
        { onSuccess: () => { toast.success("Webhook updated"); onClose(); }, onError: (e) => toast.error(errorMessage(e)) }
      );
    } else {
      create.mutate(
        {
          name,
          slug,
          inbound_enabled: inboundEnabled,
          outbound_enabled: outboundEnabled,
          outbound_url: outboundURL || null,
        },
        {
          onSuccess: (res) => {
            onCreated?.(res.webhook, res.secret);
            onClose();
          },
          onError: (e) => toast.error(errorMessage(e)),
        }
      );
    }
  }

  function rotateSecret() {
    if (!webhook) return;
    rotate.mutate(webhook.id, {
      onSuccess: (res) => {
        setNewSecret(res.secret);
        navigator.clipboard.writeText(res.secret);
        toast.success("New secret copied to clipboard");
      },
      onError: (e) => toast.error(errorMessage(e)),
    });
  }

  const valid = !!name && !!slug;
  const saving = create.isPending || update.isPending;

  return (
    <FormDrawer
      open
      onClose={onClose}
      title={editing ? webhook!.name : "New Webhook"}
      subtitle={editing ? "Edit webhook" : "Create inbound webhook endpoint"}
      footer={
        <>
          <Button variant="secondary" onClick={onClose}>Cancel</Button>
          <Button disabled={!valid || saving} onClick={submit}>{editing ? "Save" : "Create"}</Button>
        </>
      }
    >
      <div className="space-y-3">
        <div>
          <Label>Name</Label>
          <Input value={name} onChange={(e) => { setName(e.target.value); if (!editing && !slugTouched) setSlug(slugify(e.target.value)); }} />
        </div>
        <div>
          <Label>Slug</Label>
          <Input value={slug} onChange={(e) => { setSlugTouched(true); setSlug(e.target.value); }} />
        </div>
        <div className="rounded-md border border-gray-100 bg-gray-50 p-3 space-y-3">
          <p className="text-xs font-semibold text-gray-600 uppercase tracking-wide">Direction</p>
          <div className="flex items-center justify-between">
            <div>
              <Label>Inbound</Label>
              <p className="text-xs text-gray-500">Accept POST callbacks from providers</p>
            </div>
            <Switch checked={inboundEnabled} onChange={setInboundEnabled} />
          </div>
          {inboundEnabled && slug && (
            <p className="text-xs font-mono text-gray-400">
              POST {API_URL}/api/v1/webhooks/{slug}
            </p>
          )}
          <div className="flex items-center justify-between">
            <div>
              <Label>Outbound</Label>
              <p className="text-xs text-gray-500">Send HTTP POST on lead/pipeline events</p>
            </div>
            <Switch checked={outboundEnabled} onChange={setOutboundEnabled} />
          </div>
          {outboundEnabled && (
            <div>
              <Label>Outbound URL</Label>
              <Input
                value={outboundURL}
                onChange={(e) => setOutboundURL(e.target.value)}
                placeholder="https://example.com/webhook"
              />
            </div>
          )}
        </div>
        {editing && (
          <>
            <div className="flex items-center justify-between">
              <Label>Active</Label>
              <Switch checked={isActive} onChange={setIsActive} />
            </div>
            {inboundEnabled && (
              <div className="rounded-md border border-amber-100 bg-amber-50 px-3 py-2 text-sm text-amber-800">
                <p className="font-medium">Inbound secret prefix: {webhook!.secret_prefix}…</p>
                <Button size="sm" variant="secondary" className="mt-2" onClick={rotateSecret}>
                  <KeyRound className="h-3.5 w-3.5" /> Rotate inbound secret
                </Button>
                {newSecret && (
                  <div className="mt-2 flex items-center gap-2 font-mono text-xs">
                    <span className="break-all">{newSecret}</span>
                    <IconButton aria-label="Copy" onClick={() => { navigator.clipboard.writeText(newSecret!); toast.success("Copied"); }}>
                      <Copy className="h-3.5 w-3.5" />
                    </IconButton>
                  </div>
                )}
              </div>
            )}
            {outboundEnabled && webhook!.outbound_url && (
              <div className="rounded-md border border-blue-100 bg-blue-50 px-3 py-2 text-sm text-blue-800">
                <p className="font-medium">Outbound HMAC secret</p>
                <p className="text-xs text-blue-600 mt-1">Used to sign outbound requests (X-Leadrula-Signature header)</p>
                <Button
                  size="sm"
                  variant="secondary"
                  className="mt-2"
                  onClick={() => {
                    rotateOutbound.mutate(webhook!.id, {
                      onSuccess: (res) => {
                        navigator.clipboard.writeText(res.secret);
                        toast.success("New outbound secret copied to clipboard");
                      },
                      onError: (e) => toast.error(errorMessage(e)),
                    });
                  }}
                >
                  <KeyRound className="h-3.5 w-3.5" /> Rotate outbound secret
                </Button>
              </div>
            )}
          </>
        )}
      </div>
    </FormDrawer>
  );
}

function WebhookDetailDrawer({
  webhook,
  open,
  onClose,
}: {
  webhook: Webhook | null;
  open: boolean;
  onClose: () => void;
}) {
  const [tab, setTab] = useState<"events" | "triggers" | "log">("events");
  const [eventDrawer, setEventDrawer] = useState<WebhookEvent | null | undefined>(undefined);
  const [mapEvent, setMapEvent] = useState<WebhookEvent | null>(null);
  const [triggerDrawer, setTriggerDrawer] = useState<WebhookOutboundTrigger | null | undefined>(undefined);

  const { data: events } = useWebhookEvents(webhook?.id ?? null);
  const { data: triggers } = useWebhookOutboundTriggers(webhook?.id ?? null);
  const { data: deliveries } = useWebhookDeliveries(webhook?.id ?? null);
  const deleteEvent = useDeleteWebhookEvent();
  const deleteTrigger = useDeleteWebhookOutboundTrigger();

  if (!open || !webhook) return null;

  return (
    <>
      <FormDrawer open onClose={onClose} title={webhook.name} subtitle={`Webhook · ${webhook.slug}`} width={720}>
        <div className="space-y-4">
          {webhook.inbound_enabled && (
            <p className="font-mono text-xs text-gray-500">
              Authorization: Bearer {"{secret}"} · POST {API_URL}/api/v1/webhooks/{webhook.slug}
            </p>
          )}
          {webhook.outbound_enabled && webhook.outbound_url && (
            <p className="text-xs text-gray-500">
              <span className="font-semibold text-gray-700">Outbound →</span>{" "}
              <span className="font-mono">{webhook.outbound_url}</span>
            </p>
          )}
          <div className="flex gap-2 border-b border-gray-100 pb-2">
            {webhook.inbound_enabled && (
              <Button size="sm" variant={tab === "events" ? "primary" : "secondary"} onClick={() => setTab("events")}>
                Inbound events
              </Button>
            )}
            {webhook.outbound_enabled && (
              <Button size="sm" variant={tab === "triggers" ? "primary" : "secondary"} onClick={() => setTab("triggers")}>
                Outbound triggers
              </Button>
            )}
            <Button size="sm" variant={tab === "log" ? "primary" : "secondary"} onClick={() => setTab("log")}>Delivery log</Button>
          </div>

          {tab === "events" && (
            <div className="space-y-3">
              <div className="flex justify-end">
                <Button size="sm" onClick={() => setEventDrawer(null)}><Plus className="h-3.5 w-3.5" /> Add event</Button>
              </div>
              {(events ?? []).length === 0 ? (
                <EmptyState title="No events configured." />
              ) : (
                <Table>
                  <THead>
                    <tr>
                      <TH>Event key</TH>
                      <TH>Action</TH>
                      <TH>Config</TH>
                      <TH />
                    </tr>
                  </THead>
                  <TBody>
                    {(events ?? []).map((e) => (
                      <TR key={e.id}>
                        <TD className="font-mono">{e.event_key}</TD>
                        <TD><Badge>{e.action}</Badge></TD>
                        <TD className="text-xs text-gray-500">
                          {e.action === "create" && e.duplicate_mode && `on duplicate: ${e.duplicate_mode}`}
                          {e.action !== "create" && e.lookup_by && `lookup: ${e.lookup_by}`}
                          {e.action === "move_stage" && e.target_stage_id && ` → stage ${e.target_stage_id}`}
                          {e.action === "create" && e.target_pipeline_id && ` → pipeline ${e.target_pipeline_id}`}
                        </TD>
                        <TD>
                          <div className="flex justify-end gap-1">
                            <IconButton aria-label="Field map" onClick={() => setMapEvent(e)}><ArrowRightLeft className="h-4 w-4" /></IconButton>
                            <IconButton variant="danger" onClick={() => deleteEvent.mutate({ webhookId: webhook.id, eventId: e.id }, { onError: (err) => toast.error(errorMessage(err)) })}>
                              <Trash2 className="h-4 w-4" />
                            </IconButton>
                          </div>
                        </TD>
                      </TR>
                    ))}
                  </TBody>
                </Table>
              )}
            </div>
          )}

          {tab === "triggers" && (
            <div className="space-y-3">
              <div className="flex justify-end">
                <Button size="sm" onClick={() => setTriggerDrawer(null)}>
                  <Plus className="h-3.5 w-3.5" /> Add trigger
                </Button>
              </div>
              {(triggers ?? []).length === 0 ? (
                <EmptyState title="No outbound triggers configured." />
              ) : (
                <Table>
                  <THead>
                    <tr>
                      <TH>Event</TH>
                      <TH>Conditions</TH>
                      <TH>Active</TH>
                      <TH />
                    </tr>
                  </THead>
                  <TBody>
                    {(triggers ?? []).map((t) => (
                      <TR key={t.id} onClick={() => setTriggerDrawer(t)}>
                        <TD><Badge>{t.trigger_event}</Badge></TD>
                        <TD className="text-xs text-gray-500">
                          {Array.isArray(t.conditions) && t.conditions.length > 0
                            ? `${t.conditions.length} condition${t.conditions.length !== 1 ? "s" : ""} (${t.condition_logic})`
                            : "Always"}
                        </TD>
                        <TD>{t.is_active ? "✓" : "—"}</TD>
                        <TD>
                          <div className="flex justify-end gap-1" onClick={(e) => e.stopPropagation()}>
                            <IconButton aria-label="Edit" onClick={() => setTriggerDrawer(t)}>
                              <Zap className="h-4 w-4" />
                            </IconButton>
                            <IconButton
                              variant="danger"
                              onClick={() =>
                                deleteTrigger.mutate(
                                  { webhookId: webhook.id, triggerId: t.id },
                                  { onError: (err) => toast.error(errorMessage(err)) }
                                )
                              }
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
            </div>
          )}

          {tab === "log" && (
            <div className="space-y-2">
              {(deliveries?.items ?? []).length === 0 ? (
                <EmptyState title="No deliveries yet." />
              ) : (
                <Table>
                  <THead>
                    <tr><TH>Time</TH><TH>Status</TH><TH>Lead</TH><TH>Error</TH></tr>
                  </THead>
                  <TBody>
                    {(deliveries?.items ?? []).map((d) => (
                      <TR key={d.id}>
                        <TD className="text-xs">{format(new Date(d.created_at), "MMM d h:mma")}</TD>
                        <TD><Badge>{d.status}</Badge></TD>
                        <TD className="font-mono text-xs">{d.lead_public_id ?? "—"}</TD>
                        <TD className="text-xs text-red-600">{d.error_message ?? ""}</TD>
                      </TR>
                    ))}
                  </TBody>
                </Table>
              )}
            </div>
          )}
        </div>
      </FormDrawer>

      <EventDrawer
        webhookId={webhook.id}
        event={eventDrawer ?? null}
        open={eventDrawer !== undefined}
        onClose={() => setEventDrawer(undefined)}
        onCreated={() => { toast.success("Event created"); setEventDrawer(undefined); }}
      />
      <EventFieldMapDrawer
        webhookId={webhook.id}
        event={mapEvent}
        slug={webhook.slug}
        open={!!mapEvent}
        onClose={() => setMapEvent(null)}
      />
      <OutboundTriggerDrawer
        webhookId={webhook.id}
        trigger={triggerDrawer ?? null}
        open={triggerDrawer !== undefined}
        onClose={() => setTriggerDrawer(undefined)}
      />
    </>
  );
}

function EventDrawer({
  webhookId,
  event,
  open,
  onClose,
  onCreated,
}: {
  webhookId: number;
  event: WebhookEvent | null;
  open: boolean;
  onClose: () => void;
  onCreated?: () => void;
}) {
  if (!open) return null;
  const create = useCreateWebhookEvent();
  const { data: pipelines } = usePipelines();

  const [eventKey, setEventKey] = useState("");
  const [action, setAction] = useState<WebhookEvent["action"]>("update");
  const [duplicateMode, setDuplicateMode] = useState<"update" | "duplicate" | "reject">("reject");
  const [lookupBy, setLookupBy] = useState<"external_id" | "public_id">("external_id");
  const [targetPipelineId, setTargetPipelineId] = useState<number | "">("");
  const [targetStageId, setTargetStageId] = useState<number | "">("");

  const pipelineId = typeof targetPipelineId === "number" ? targetPipelineId : undefined;
  const { data: stages } = useStages(pipelineId);

  function submit() {
    const body: Record<string, unknown> = { event_key: eventKey, action };
    if (action === "create") {
      body.duplicate_mode = duplicateMode;
      if (targetPipelineId) body.target_pipeline_id = targetPipelineId;
    } else {
      body.lookup_by = lookupBy;
    }
    if (action === "move_stage" && targetStageId) body.target_stage_id = targetStageId;

    create.mutate({ webhookId, body }, {
      onSuccess: () => onCreated?.(),
      onError: (e) => toast.error(errorMessage(e)),
    });
  }

  return (
    <FormDrawer open onClose={onClose} title="New event" subtitle="Map payload event value to an action" footer={
      <>
        <Button variant="secondary" onClick={onClose}>Cancel</Button>
        <Button disabled={!eventKey || create.isPending} onClick={submit}>Create</Button>
      </>
    }>
      <div className="space-y-3">
        <div>
          <Label>Event key (matches payload event field value)</Label>
          <Input value={eventKey} onChange={(e) => setEventKey(e.target.value)} placeholder="lead.updated" />
        </div>
        <div>
          <Label>Action</Label>
          <select className="w-full rounded-md border border-gray-200 px-3 py-2 text-sm" value={action} onChange={(e) => setAction(e.target.value as WebhookEvent["action"])}>
            <option value="create">Create lead</option>
            <option value="update">Update lead</option>
            <option value="delete">Delete lead (soft)</option>
            <option value="move_stage">Move to stage</option>
          </select>
        </div>
        {action === "create" && (
          <>
            <div>
              <Label>On duplicate external_id</Label>
              <select className="w-full rounded-md border border-gray-200 px-3 py-2 text-sm" value={duplicateMode} onChange={(e) => setDuplicateMode(e.target.value as typeof duplicateMode)}>
                <option value="reject">Reject</option>
                <option value="update">Update existing</option>
                <option value="duplicate">Create duplicate</option>
              </select>
            </div>
            <div>
              <Label>Initial pipeline (optional)</Label>
              <select className="w-full rounded-md border border-gray-200 px-3 py-2 text-sm" value={targetPipelineId} onChange={(e) => setTargetPipelineId(e.target.value ? Number(e.target.value) : "")}>
                <option value="">None</option>
                {(pipelines ?? []).map((p) => <option key={p.id} value={p.id}>{p.name}</option>)}
              </select>
            </div>
          </>
        )}
        {action !== "create" && (
          <div>
            <Label>Lookup lead by</Label>
            <select className="w-full rounded-md border border-gray-200 px-3 py-2 text-sm" value={lookupBy} onChange={(e) => setLookupBy(e.target.value as typeof lookupBy)}>
              <option value="external_id">external_id</option>
              <option value="public_id">public_id</option>
            </select>
          </div>
        )}
        {action === "move_stage" && (
          <>
            <div>
              <Label>Pipeline</Label>
              <select className="w-full rounded-md border border-gray-200 px-3 py-2 text-sm" value={targetPipelineId} onChange={(e) => { setTargetPipelineId(e.target.value ? Number(e.target.value) : ""); setTargetStageId(""); }}>
                <option value="">Select pipeline</option>
                {(pipelines ?? []).map((p) => <option key={p.id} value={p.id}>{p.name}</option>)}
              </select>
            </div>
            <div>
              <Label>Target stage</Label>
              <select className="w-full rounded-md border border-gray-200 px-3 py-2 text-sm" value={targetStageId} onChange={(e) => setTargetStageId(e.target.value ? Number(e.target.value) : "")}>
                <option value="">Select stage</option>
                {(stages ?? []).map((s) => <option key={s.id} value={s.id}>{s.name}</option>)}
              </select>
            </div>
          </>
        )}
      </div>
    </FormDrawer>
  );
}

function EventFieldMapDrawer({
  webhookId,
  event,
  slug,
  open,
  onClose,
}: {
  webhookId: number;
  event: WebhookEvent | null;
  slug: string;
  open: boolean;
  onClose: () => void;
}) {
  if (!open || !event) return null;

  const { data: entries } = useWebhookFieldMap(webhookId, event.id);
  const { data: sample, isLoading: sampleLoading, refetch } = useWebhookSamplePayload(webhookId, true);
  const { data: customFields } = useCustomFields();
  const add = useAddWebhookFieldMap();
  const remove = useDeleteWebhookFieldMap();
  const createField = useCreateField();

  const [sourceKey, setSourceKey] = useState("");
  const [target, setTarget] = useState("external_id");
  const [createFieldOpen, setCreateFieldOpen] = useState(false);

  const payload = sample?.payload ?? null;
  const mappableKeys = payload ? mappablePayloadKeys(payload) : [];
  const mapped = mappedKeys(entries ?? []);
  const unmappedKeys = mappableKeys.filter((k) => !mapped.has(k));

  function addMapping(key: string, targetVal: string) {
    const isCustom = targetVal.startsWith("cf:");
    const body: Record<string, unknown> = isCustom
      ? { source_key: key, target_type: "custom", custom_field_id: Number(targetVal.slice(3)) }
      : { source_key: key, target_type: "builtin", builtin_field: targetVal };
    add.mutate({ webhookId, eventId: event!.id, body }, {
      onSuccess: () => setSourceKey(""),
      onError: (e) => toast.error(errorMessage(e)),
    });
  }

  return (
    <FormDrawer open onClose={onClose} title="Event field mapping" subtitle={`${event.event_key} · ${slug}`} width={560}>
      <div className="space-y-4">
        <div className="space-y-2">
          <div className="flex items-center justify-between">
            <Label>Latest payload</Label>
            <Button size="sm" variant="secondary" onClick={() => refetch()}><RefreshCw className="h-3.5 w-3.5" /> Refresh</Button>
          </div>
          {sampleLoading ? <Spinner className="h-5 w-5" /> : !payload ? (
            <p className="text-sm text-gray-500">No webhook received yet.</p>
          ) : (
            <pre className="max-h-48 overflow-auto rounded-md border border-gray-100 bg-gray-50 p-3 font-mono text-xs">{JSON.stringify(payload, null, 2)}</pre>
          )}
        </div>

        {unmappedKeys.length > 0 && (
          <div className="flex flex-wrap gap-1.5">
            {unmappedKeys.map((k) => (
              <button key={k} type="button" onClick={() => setSourceKey(k)} className="rounded-full border border-jade-200 bg-jade-50 px-2 py-0.5 font-mono text-xs">{k}</button>
            ))}
          </div>
        )}

        <div className="flex gap-2">
          <Input value={sourceKey} onChange={(e) => setSourceKey(e.target.value)} placeholder="payload key" className="flex-1" />
          <BuiltinCustomFieldSelect
            label="Target field"
            builtins={BUILTINS}
            customFields={customFields ?? []}
            value={target}
            onChange={setTarget}
            onAddCustomField={() => setCreateFieldOpen(true)}
          />
          <Button disabled={!sourceKey} onClick={() => addMapping(sourceKey, target)}>Map</Button>
        </div>

        {(entries ?? []).length > 0 && (
          <Table>
            <THead><tr><TH>Payload key</TH><TH>Target</TH><TH /></tr></THead>
            <TBody>
              {(entries ?? []).map((e) => (
                <TR key={e.id}>
                  <TD className="font-mono text-xs">{e.source_key}</TD>
                  <TD className="text-xs">{e.target_type === "builtin" ? e.builtin_field : `custom #${e.custom_field_id}`}</TD>
                  <TD>
                    <IconButton variant="danger" onClick={() => remove.mutate(e.id)}><Trash2 className="h-4 w-4" /></IconButton>
                  </TD>
                </TR>
              ))}
            </TBody>
          </Table>
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

// ── outbound trigger field suggestions ───────────────────────────────────────
const TRIGGER_EVENT_LABELS: Record<OutboundTriggerEvent, string> = {
  "lead.create": "Lead created",
  "lead.update": "Lead updated",
  "lead.delete": "Lead deleted",
  "pipeline.move_stage": "Stage moved",
  "pipeline.place": "Placed in pipeline",
  "pipeline.stage_rule_applied": "Stage rule applied",
};

const TEMPLATE_FIELDS = [
  "{{event}}", "{{lead.public_id}}", "{{lead.first_name}}", "{{lead.last_name}}",
  "{{lead.phone}}", "{{lead.email}}", "{{lead.address}}", "{{lead.city}}",
  "{{lead.state}}", "{{lead.zip}}", "{{lead.source}}", "{{lead.external_id}}",
  "{{lead.status}}", "{{lead.created_at}}",
  "{{pipeline.pipeline_id}}", "{{pipeline.pipeline_name}}",
  "{{pipeline.stage_id}}", "{{pipeline.stage_name}}",
  "{{pipeline.previous_stage_id}}", "{{pipeline.previous_stage_name}}",
];

const DEFAULT_TEMPLATE = `{
  "event": "{{event}}",
  "lead_id": "{{lead.public_id}}",
  "first_name": "{{lead.first_name}}",
  "last_name": "{{lead.last_name}}",
  "phone": "{{lead.phone}}",
  "stage": "{{pipeline.stage_name}}"
}`;

function OutboundTriggerDrawer({
  webhookId,
  trigger,
  open,
  onClose,
}: {
  webhookId: number;
  trigger: WebhookOutboundTrigger | null;
  open: boolean;
  onClose: () => void;
}) {
  if (!open) return null;

  const editing = trigger !== null;
  const create = useCreateWebhookOutboundTrigger();
  const update = useUpdateWebhookOutboundTrigger();

  const { data: customFields = [] } = useCustomFields();
  const [triggerEvent, setTriggerEvent] = useState<OutboundTriggerEvent>(
    trigger?.trigger_event ?? "lead.create"
  );
  const [conditionLogic, setConditionLogic] = useState<"and" | "or">(
    trigger?.condition_logic ?? "and"
  );
  const [payloadTemplate, setPayloadTemplate] = useState(
    trigger?.payload_template ?? DEFAULT_TEMPLATE
  );
  const [isActive, setIsActive] = useState(trigger?.is_active ?? true);
  const [templateCursor, setTemplateCursor] = useState<HTMLTextAreaElement | null>(null);
  const [responseMap, setResponseMap] = useState<ResponseMapEntry[]>(
    trigger?.response_map ?? []
  );

  useEffect(() => {
    setTriggerEvent(trigger?.trigger_event ?? "lead.create");
    setConditionLogic(trigger?.condition_logic ?? "and");
    setPayloadTemplate(trigger?.payload_template ?? DEFAULT_TEMPLATE);
    setIsActive(trigger?.is_active ?? true);
    setResponseMap(trigger?.response_map ?? []);
  }, [trigger]);

  function addResponseRow() {
    setResponseMap((prev) => [
      ...prev,
      { response_key: "", target_type: "builtin", builtin_field: "external_id" },
    ]);
  }

  function removeResponseRow(idx: number) {
    setResponseMap((prev) => prev.filter((_, i) => i !== idx));
  }

  function updateResponseRow(idx: number, field: string, val: string) {
    setResponseMap((prev) => {
      const next = [...prev];
      const entry = { ...next[idx] };
      if (field === "response_key") {
        entry.response_key = val;
      } else if (field === "target") {
        if (val.startsWith("cf:")) {
          entry.target_type = "custom";
          entry.custom_field_id = parseInt(val.slice(3), 10);
          delete entry.builtin_field;
        } else {
          entry.target_type = "builtin";
          entry.builtin_field = val;
          delete entry.custom_field_id;
        }
      }
      next[idx] = entry;
      return next;
    });
  }

  function targetValue(entry: ResponseMapEntry): string {
    if (entry.target_type === "custom" && entry.custom_field_id !== undefined) {
      return `cf:${entry.custom_field_id}`;
    }
    return entry.builtin_field ?? "external_id";
  }

  function insertField(field: string) {
    if (templateCursor) {
      const start = templateCursor.selectionStart;
      const end = templateCursor.selectionEnd;
      const before = payloadTemplate.slice(0, start);
      const after = payloadTemplate.slice(end);
      setPayloadTemplate(before + field + after);
      setTimeout(() => {
        templateCursor.selectionStart = start + field.length;
        templateCursor.selectionEnd = start + field.length;
        templateCursor.focus();
      }, 0);
    } else {
      setPayloadTemplate((t) => t + field);
    }
  }

  function save() {
    const body = {
      trigger_event: triggerEvent,
      condition_logic: conditionLogic,
      conditions: [],
      payload_template: payloadTemplate,
      response_map: responseMap,
      is_active: isActive,
    };
    if (editing && trigger) {
      update.mutate(
        { webhookId, triggerId: trigger.id, body },
        {
          onSuccess: () => { toast.success("Trigger updated"); onClose(); },
          onError: (e) => toast.error(errorMessage(e)),
        }
      );
    } else {
      create.mutate(
        { webhookId, body },
        {
          onSuccess: () => { toast.success("Trigger created"); onClose(); },
          onError: (e) => toast.error(errorMessage(e)),
        }
      );
    }
  }

  const saving = create.isPending || update.isPending;

  return (
    <FormDrawer
      open
      onClose={onClose}
      title={editing ? "Edit outbound trigger" : "New outbound trigger"}
      subtitle="Configure when and how outbound webhook requests are sent"
      width={600}
      footer={
        <>
          <Button variant="secondary" onClick={onClose}>Cancel</Button>
          <Button disabled={saving} onClick={save}>{editing ? "Save" : "Create"}</Button>
        </>
      }
    >
      <div className="space-y-4">
        <div>
          <Label>Trigger event</Label>
          <select
            className="w-full rounded-md border border-gray-200 bg-white px-3 py-2 text-sm"
            value={triggerEvent}
            onChange={(e) => setTriggerEvent(e.target.value as OutboundTriggerEvent)}
          >
            {(Object.entries(TRIGGER_EVENT_LABELS) as [OutboundTriggerEvent, string][]).map(([k, v]) => (
              <option key={k} value={k}>{v}</option>
            ))}
          </select>
        </div>

        <div className="flex items-center gap-3">
          <Label>Active</Label>
          <Switch checked={isActive} onChange={setIsActive} />
        </div>

        <div>
          <div className="mb-2 flex items-center justify-between">
            <Label>JSON payload template</Label>
            <p className="text-xs text-gray-500">Use {"{{field}}"} placeholders</p>
          </div>
          <div className="mb-2 flex flex-wrap gap-1">
            {TEMPLATE_FIELDS.map((f) => (
              <button
                key={f}
                type="button"
                onClick={() => insertField(f)}
                className="rounded border border-indigo-200 bg-indigo-50 px-1.5 py-0.5 font-mono text-xs text-indigo-700 hover:bg-indigo-100"
              >
                {f}
              </button>
            ))}
          </div>
          <textarea
            ref={(el) => setTemplateCursor(el)}
            rows={12}
            value={payloadTemplate}
            onChange={(e) => setPayloadTemplate(e.target.value)}
            className="w-full rounded-md border border-gray-200 bg-white px-3 py-2 font-mono text-xs focus:outline-none focus:ring-2 focus:ring-jade-500"
            spellCheck={false}
          />
        </div>

        <div className="rounded-md border border-gray-100 bg-gray-50 px-3 py-2 text-xs text-gray-500">
          <p className="font-semibold text-gray-700 mb-1">Conditions</p>
          <p>Condition builder is available for advanced filtering — all triggers fire unconditionally by default. Edit the JSON payload template above to control what data is sent.</p>
        </div>

        <div>
          <div className="mb-2 flex items-center justify-between">
            <Label>Response mapping</Label>
            <button
              type="button"
              onClick={addResponseRow}
              className="flex items-center gap-1 text-xs text-indigo-600 hover:text-indigo-800"
            >
              <Plus className="h-3 w-3" /> Add field
            </button>
          </div>
          {responseMap.length === 0 ? (
            <p className="text-xs text-gray-400">
              Map fields from the external API response back to the lead. Example: extract{" "}
              <code className="font-mono">data.id</code> and write it to{" "}
              <code className="font-mono">external_id</code>.
            </p>
          ) : (
            <div className="space-y-2">
              {responseMap.map((entry, idx) => (
                <div key={idx} className="flex items-center gap-2">
                  <input
                    type="text"
                    placeholder="data.id"
                    value={entry.response_key}
                    onChange={(e) => updateResponseRow(idx, "response_key", e.target.value)}
                    className="w-32 rounded border border-gray-200 px-2 py-1 font-mono text-xs focus:outline-none focus:ring-1 focus:ring-jade-500"
                  />
                  <span className="text-xs text-gray-400">→</span>
                  <select
                    value={targetValue(entry)}
                    onChange={(e) => updateResponseRow(idx, "target", e.target.value)}
                    className="flex-1 rounded border border-gray-200 bg-white px-2 py-1 text-xs focus:outline-none focus:ring-1 focus:ring-jade-500"
                  >
                    <optgroup label="Built-in">
                      {["first_name", "last_name", "phone", "email", "address", "city", "state", "zip", "source", "external_id"].map((b) => (
                        <option key={b} value={b}>{b}</option>
                      ))}
                    </optgroup>
                    {customFields.filter((f) => f.is_active !== false).length > 0 && (
                      <optgroup label="Custom">
                        {customFields
                          .filter((f) => f.is_active !== false)
                          .map((f) => (
                            <option key={f.id} value={`cf:${f.id}`}>{f.name}</option>
                          ))}
                      </optgroup>
                    )}
                  </select>
                  <button
                    type="button"
                    onClick={() => removeResponseRow(idx)}
                    className="text-gray-400 hover:text-red-500"
                  >
                    <Trash2 className="h-3.5 w-3.5" />
                  </button>
                </div>
              ))}
            </div>
          )}
        </div>
      </div>
    </FormDrawer>
  );
}
