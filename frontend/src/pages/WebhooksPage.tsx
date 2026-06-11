import { Fragment, useEffect, useMemo, useRef, useState } from "react";
import {
  useWebhooks,
  useCreateWebhook,
  useUpdateWebhook,
  useDeleteWebhook,
  useRotateWebhookSecret,
  useRotateWebhookOutboundSecret,
  useWebhookEvents,
  useCreateWebhookEvent,
  useUpdateWebhookEvent,
  useDeleteWebhookEvent,
  useWebhookFieldMap,
  useWebhookSamplePayload,
  useAddWebhookFieldMap,
  useDeleteWebhookFieldMap,
  useWebhookDeliveries,
  useWebhookDelivery,
  useReplayWebhookDelivery,
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
import { buildPayloadSuggestions, MAP_BUILTIN_FIELDS } from "@/features/leads/csvMapping";
import { SUNBASE_URL, sunbaseFieldMap } from "@/features/integrations/sunbaseConstants";
import { PageHeader } from "@/components/layout/PageHeader";
import { PageBody } from "@/components/layout/PageBody";
import { IconButton } from "@/components/layout/IconButton";
import { Table, THead, TH, TBody, TR, TD } from "@/components/ui/table";
import { Button } from "@/components/ui/button";
import { Input, Label } from "@/components/ui/input";
import { Switch, Spinner, EmptyState, Badge } from "@/components/ui/misc";
import { FormDrawer } from "@/components/ui/dialog";
import { Dropdown, DropdownItem, DropdownSearch } from "@/components/ui/dropdown";
import { format } from "date-fns";
import { ArrowRightLeft, Copy, Eye, EyeOff, KeyRound, Pencil, Plus, Trash2, Zap } from "lucide-react";
import { toast } from "@/store/toastStore";
import { errorMessage } from "@/lib/api";
import type { Webhook, WebhookEvent, WebhookOutboundTrigger, OutboundTriggerEvent, OutboundFormat, OutboundMethod, OutboundFieldMapEntry, ResponseMapEntry, InboundCondition, WebhookDelivery } from "@/types";
import { canReplayDelivery, webhookDeliveryStatusLabel } from "@/features/intake/logShared";

type MappingContext = {
  deliveryId?: number;
  payload?: Record<string, unknown>;
  actionId?: number;
} | null;

const INBOUND_CONDITION_OPS: { value: InboundCondition["op"]; label: string }[] = [
  { value: "eq", label: "equals" },
  { value: "neq", label: "not equals" },
  { value: "contains", label: "contains" },
  { value: "empty", label: "is empty" },
  { value: "not_empty", label: "is not empty" },
];

const CONDITION_SUMMARY_OPS: Record<InboundCondition["op"], string> = {
  eq: "EQUAL",
  neq: "NOT EQUAL",
  contains: "CONTAINS",
  empty: "IS EMPTY",
  not_empty: "IS NOT EMPTY",
};

function formatCondition(c: InboundCondition): string {
  const op = CONDITION_SUMMARY_OPS[c.op] ?? c.op.toUpperCase();
  if (c.op === "empty" || c.op === "not_empty") {
    return `Payload: "${c.field}" ${op}`;
  }
  return `Payload: "${c.field}" ${op} "${c.value ?? ""}"`;
}

function conditionSummary(conditions: InboundCondition[], logic: string): string {
  if (!conditions?.length) return "Always";
  const joiner = logic === "or" ? " OR " : " AND ";
  return conditions.map(formatCondition).join(joiner);
}

function InboundConditionRow({
  condition,
  mappableKeys,
  onChange,
  onRemove,
}: {
  condition: InboundCondition;
  mappableKeys: string[];
  onChange: (c: InboundCondition) => void;
  onRemove: () => void;
}) {
  const showValue = condition.op !== "empty" && condition.op !== "not_empty";
  return (
    <div className="flex flex-wrap items-center gap-2 rounded-md border border-gray-100 bg-gray-50 p-2">
      <select
        className="rounded border border-gray-200 px-2 py-1 text-xs"
        value={condition.field}
        onChange={(e) => onChange({ ...condition, field: e.target.value })}
      >
        <option value="">Field</option>
        {mappableKeys.map((k) => <option key={k} value={k}>{k}</option>)}
        {condition.field && !mappableKeys.includes(condition.field) && (
          <option value={condition.field}>{condition.field}</option>
        )}
      </select>
      <select
        className="rounded border border-gray-200 px-2 py-1 text-xs"
        value={condition.op}
        onChange={(e) => {
          const op = e.target.value as InboundCondition["op"];
          onChange({ ...condition, op, value: op === "empty" || op === "not_empty" ? "" : condition.value });
        }}
      >
        {INBOUND_CONDITION_OPS.map((o) => <option key={o.value} value={o.value}>{o.label}</option>)}
      </select>
      {showValue && (
        <Input
          className="w-32 text-xs"
          value={condition.value ?? ""}
          onChange={(e) => onChange({ ...condition, value: e.target.value })}
          placeholder="value"
        />
      )}
      <IconButton variant="danger" aria-label="Remove condition" onClick={onRemove}>
        <Trash2 className="h-3.5 w-3.5" />
      </IconButton>
    </div>
  );
}

const API_URL = import.meta.env.VITE_API_URL ?? "http://localhost:8080";
const BUILTINS = [
  ...MAP_BUILTIN_FIELDS,
  "external_id",
  "action_at",
  "disqualification_reason_id",
];

const OUTBOUND_BUILTINS = [
  "first_name", "last_name", "phone", "email", "address", "city", "state", "zip",
  "source", "external_id", "public_id", "status", "cost", "revenue",
];

const OUTBOUND_META_FIELDS = [
  { value: "event", label: "Event name" },
  { value: "pipeline.stage_name", label: "Stage name" },
  { value: "pipeline.pipeline_name", label: "Pipeline name" },
];

function outboundHelperText(format: OutboundFormat, method: OutboundMethod): string {
  if (format === "json" && method === "POST") return "Send a JSON body on POST";
  if (format === "json" && method === "GET") return "Template keys become query parameters on the URL";
  return "Field map keys become query parameters on the URL. GET and POST send the same query string.";
}

function resolveOutboundFieldMap(map?: OutboundFieldMapEntry[]): OutboundFieldMapEntry[] {
  return map ?? [];
}

function slugify(name: string) {
  return name.toLowerCase().replace(/[^a-z0-9]+/g, "-").replace(/^-|-$/g, "");
}

function copyText(text: string, label: string) {
  navigator.clipboard.writeText(text).then(
    () => toast.success(label),
    () => toast.error("Could not copy to clipboard")
  );
}

function InboundEndpointRows({
  slug,
  secret,
  secretPrefix,
  secretRequired,
  showSecret,
  onToggleSecret,
  onCopySecret,
  onRotate,
  rotatePending,
}: {
  slug: string;
  secret: string | null;
  secretPrefix?: string;
  secretRequired: boolean;
  showSecret: boolean;
  onToggleSecret: () => void;
  onCopySecret: () => void;
  onRotate: () => void;
  rotatePending?: boolean;
}) {
  const endpoint = `${API_URL}/api/v1/webhooks/${slug}`;
  return (
    <div className="space-y-2 text-sm">
      <div className="flex items-center gap-1.5 font-mono text-xs text-gray-700">
        <span className="select-all break-all flex-1">POST {endpoint}</span>
        <IconButton
          aria-label="Copy endpoint"
          onClick={() => copyText(`POST ${endpoint}`, "Endpoint copied")}
        >
          <Copy className="h-3.5 w-3.5" />
        </IconButton>
      </div>
      {!secretRequired ? (
        <p className="text-xs text-amber-700">No secret required. Anyone with this URL can POST payloads.</p>
      ) : secret ? (
        <div className="flex items-center gap-2 text-xs text-gray-700">
          <span className="font-medium">Secret:</span>
          <span className="font-mono select-all">
            {showSecret ? secret : "••••••••••••••••"}
          </span>
          <IconButton
            aria-label={showSecret ? "Hide secret" : "Show secret"}
            onClick={onToggleSecret}
          >
            {showSecret ? <EyeOff className="h-3.5 w-3.5" /> : <Eye className="h-3.5 w-3.5" />}
          </IconButton>
          <IconButton aria-label="Copy secret" onClick={onCopySecret}>
            <Copy className="h-3.5 w-3.5" />
          </IconButton>
          <IconButton aria-label="Rotate secret" disabled={rotatePending} onClick={onRotate}>
            <KeyRound className="h-3.5 w-3.5" />
          </IconButton>
        </div>
      ) : (
        <div className="space-y-1">
          <div className="flex items-center gap-2 text-xs text-gray-700">
            <span className="font-medium">Secret:</span>
            <span className="font-mono">••••••••••••••••</span>
            {secretPrefix && (
              <span className="font-mono text-gray-500">({secretPrefix}…)</span>
            )}
            <IconButton aria-label="Rotate secret" disabled={rotatePending} onClick={onRotate}>
              <KeyRound className="h-3.5 w-3.5" />
            </IconButton>
          </div>
          <p className="text-xs text-gray-500">
            Full secret is only available after creation or rotation.
          </p>
        </div>
      )}
    </div>
  );
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
  const [detailSecret, setDetailSecret] = useState<string | null>(null);
  const [secretByWebhookId, setSecretByWebhookId] = useState<Record<number, string>>({});
  const [mappingContext, setMappingContext] = useState<MappingContext>(null);

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
                <TH className="min-w-0 w-12" />
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
                      <IconButton aria-label="Edit" onClick={() => setDrawerWebhook(w)}>
                        <Pencil className="h-4 w-4" />
                      </IconButton>
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
        mappingContext={mappingContext}
        onMappingContextChange={setMappingContext}
        onClose={() => { setDrawerWebhook(undefined); setMappingContext(null); }}
        onCreated={(wb, secret) => {
          toast.success("Webhook created");
          if (secret) {
            navigator.clipboard.writeText(secret);
            toast.success("Secret copied to clipboard");
            setSecretByWebhookId((prev) => ({ ...prev, [wb.id]: secret }));
            setDetailSecret(secret);
          } else {
            setDetailSecret(null);
          }
          setDetailFor(wb);
        }}
      />
      <WebhookDetailDrawer
        webhook={detailFor}
        open={!!detailFor}
        initialSecret={
          detailFor
            ? detailSecret ?? secretByWebhookId[detailFor.id] ?? null
            : null
        }
        onClose={() => { setDetailFor(null); setDetailSecret(null); }}
        onSecretCached={(webhookId, secret) =>
          setSecretByWebhookId((prev) => ({ ...prev, [webhookId]: secret }))
        }
        onMapFields={(ctx) => {
          if (!detailFor) return;
          setMappingContext(ctx);
          setDrawerWebhook(detailFor);
          setDetailFor(null);
          setDetailSecret(null);
        }}
      />
    </>
  );
}

function WebhookDrawer({
  webhook,
  open,
  mappingContext,
  onMappingContextChange,
  onClose,
  onCreated,
}: {
  webhook: Webhook | null;
  open: boolean;
  mappingContext?: MappingContext;
  onMappingContextChange?: (ctx: MappingContext) => void;
  onClose: () => void;
  onCreated?: (wb: Webhook, secret: string | null) => void;
}) {
  if (!open) return null;
  const editing = webhook !== null;
  const create = useCreateWebhook();
  const update = useUpdateWebhook();
  const [actionDrawer, setActionDrawer] = useState<WebhookEvent | null | undefined>(undefined);
  const [triggerDrawer, setTriggerDrawer] = useState<WebhookOutboundTrigger | null | undefined>(undefined);
  const { data: actions } = useWebhookEvents(editing ? webhook!.id : null);
  const { data: triggers } = useWebhookOutboundTriggers(editing ? webhook!.id : null);
  const deleteAction = useDeleteWebhookEvent();
  const deleteTrigger = useDeleteWebhookOutboundTrigger();

  useEffect(() => {
    if (!mappingContext?.deliveryId || !mappingContext.actionId) return;
    const a = actions?.find((x) => x.id === mappingContext.actionId);
    if (a) setActionDrawer(a);
  }, [mappingContext, actions]);

  const [name, setName] = useState(webhook?.name ?? "");
  const [slug, setSlug] = useState(webhook?.slug ?? "");
  const [slugTouched, setSlugTouched] = useState(false);
  const [isActive, setIsActive] = useState(webhook?.is_active ?? true);
  const [inboundEnabled, setInboundEnabled] = useState(webhook?.inbound_enabled ?? true);
  const [inboundSecretRequired, setInboundSecretRequired] = useState(webhook?.inbound_secret_required ?? true);
  const [outboundEnabled, setOutboundEnabled] = useState(webhook?.outbound_enabled ?? false);
  const [outboundSignEnabled, setOutboundSignEnabled] = useState(webhook?.outbound_sign_enabled ?? true);
  const [outboundURL, setOutboundURL] = useState(webhook?.outbound_url ?? "");
  const [outboundFormat, setOutboundFormat] = useState<OutboundFormat>(webhook?.outbound_format ?? "json");
  const [outboundMethod, setOutboundMethod] = useState<OutboundMethod>(webhook?.outbound_method ?? "POST");
  const [payloadTemplate, setPayloadTemplate] = useState(resolvePayloadTemplate(webhook?.outbound_payload_template));
  const [fieldMap, setFieldMap] = useState<OutboundFieldMapEntry[]>(resolveOutboundFieldMap(webhook?.outbound_field_map));
  const [responseMap, setResponseMap] = useState<ResponseMapEntry[]>(webhook?.outbound_response_map ?? []);
  const rotateOutbound = useRotateWebhookOutboundSecret();

  useEffect(() => {
    setName(webhook?.name ?? "");
    setSlug(webhook?.slug ?? "");
    setSlugTouched(false);
    setIsActive(webhook?.is_active ?? true);
    setInboundEnabled(webhook?.inbound_enabled ?? true);
    setInboundSecretRequired(webhook?.inbound_secret_required ?? true);
    setOutboundEnabled(webhook?.outbound_enabled ?? false);
    setOutboundSignEnabled(webhook?.outbound_sign_enabled ?? true);
    setOutboundURL(webhook?.outbound_url ?? "");
    setOutboundFormat(webhook?.outbound_format ?? "json");
    setOutboundMethod(webhook?.outbound_method ?? "POST");
    setPayloadTemplate(resolvePayloadTemplate(webhook?.outbound_payload_template));
    setFieldMap(resolveOutboundFieldMap(webhook?.outbound_field_map));
    setResponseMap(webhook?.outbound_response_map ?? []);
  }, [webhook]);

  function submit() {
    if (editing && webhook) {
      const body: Record<string, unknown> = {
        name, slug, is_active: isActive,
        inbound_enabled: inboundEnabled,
        inbound_secret_required: inboundSecretRequired,
        outbound_enabled: outboundEnabled,
        outbound_sign_enabled: outboundSignEnabled,
        outbound_url: outboundURL || null,
      };
      if (outboundEnabled) {
        body.outbound_format = outboundFormat;
        body.outbound_method = outboundMethod;
        body.outbound_response_map = responseMap;
        if (outboundFormat === "json") {
          body.outbound_payload_template = payloadTemplate;
        } else {
          body.outbound_field_map = fieldMap;
        }
      }
      update.mutate(
        {
          id: webhook.id,
          body,
        },
        { onSuccess: () => { toast.success("Webhook updated"); onClose(); }, onError: (e) => toast.error(errorMessage(e)) }
      );
    } else {
      create.mutate(
        {
          name,
          slug,
          inbound_enabled: inboundEnabled,
          inbound_secret_required: inboundSecretRequired,
          outbound_enabled: outboundEnabled,
          outbound_sign_enabled: outboundSignEnabled,
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

  const valid = !!name && !!slug;
  const saving = create.isPending || update.isPending;

  return (
    <FormDrawer
      open
      onClose={onClose}
      title={editing ? webhook!.name : "New Webhook"}
      subtitle={editing ? "Edit webhook" : "Create inbound webhook endpoint"}
      width={720}
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
        <div className="space-y-3">
          <p className="text-xs font-semibold text-gray-600 uppercase tracking-wide">Direction</p>
          <div className="flex items-center justify-between">
            <div>
              <Label>Inbound</Label>
              <p className="text-xs text-gray-500">Accept POST callbacks from providers</p>
            </div>
            <Switch checked={inboundEnabled} onChange={setInboundEnabled} />
          </div>
          {inboundEnabled && (
            <div className="flex items-center justify-between pl-2 border-l-2 border-gray-100">
              <div>
                <Label>Require secret</Label>
                <p className="text-xs text-gray-500">Bearer token on inbound POST requests</p>
              </div>
              <Switch checked={inboundSecretRequired} onChange={setInboundSecretRequired} />
            </div>
          )}
          {inboundEnabled && !inboundSecretRequired && (
            <p className="text-xs text-amber-700">Anyone with the webhook URL can POST payloads.</p>
          )}
          <div className="flex items-center justify-between">
            <div>
              <Label>Outbound</Label>
              <p className="text-xs text-gray-500">{outboundEnabled ? outboundHelperText(outboundFormat, outboundMethod) : "Send HTTP GET or POST on lead/pipeline events"}</p>
            </div>
            <Switch checked={outboundEnabled} onChange={setOutboundEnabled} />
          </div>
          {outboundEnabled && (
            <div className="flex items-center justify-between pl-2 border-l-2 border-gray-100">
              <div>
                <Label>Sign requests</Label>
                <p className="text-xs text-gray-500">X-Leadrula-Signature HMAC header</p>
              </div>
              <Switch checked={outboundSignEnabled} onChange={setOutboundSignEnabled} />
            </div>
          )}
          {outboundEnabled && !outboundSignEnabled && (
            <p className="text-xs text-amber-700">Outbound requests are unsigned. Receivers cannot verify payload integrity.</p>
          )}
          {outboundEnabled && (
            <>
              <div>
                <Label>Outbound URL</Label>
                <Input
                  value={outboundURL}
                  onChange={(e) => setOutboundURL(e.target.value)}
                  placeholder="https://example.com/webhook"
                />
              </div>
              <div>
                <Label>Outbound format</Label>
                <div className="mt-1 flex gap-4">
                  <label className="flex items-center gap-1.5 text-sm">
                    <input type="radio" name="outbound-format" checked={outboundFormat === "json"} onChange={() => setOutboundFormat("json")} />
                    JSON body
                  </label>
                  <label className="flex items-center gap-1.5 text-sm">
                    <input type="radio" name="outbound-format" checked={outboundFormat === "url"} onChange={() => setOutboundFormat("url")} />
                    URL parameters
                  </label>
                </div>
              </div>
              <div>
                <Label>HTTP method</Label>
                <div className="mt-1 flex gap-4">
                  <label className="flex items-center gap-1.5 text-sm">
                    <input type="radio" name="outbound-method" checked={outboundMethod === "GET"} onChange={() => setOutboundMethod("GET")} />
                    GET
                  </label>
                  <label className="flex items-center gap-1.5 text-sm">
                    <input type="radio" name="outbound-method" checked={outboundMethod === "POST"} onChange={() => setOutboundMethod("POST")} />
                    POST
                  </label>
                </div>
              </div>
              {outboundFormat === "url" && (
                <Button
                  size="sm"
                  variant="secondary"
                  onClick={() => {
                    setOutboundURL(SUNBASE_URL);
                    setOutboundFormat("url");
                    setOutboundMethod("POST");
                    setFieldMap(sunbaseFieldMap("YOUR_SCHEMA").map((e) => ({ ...e })));
                  }}
                >
                  Apply SunbaseData preset
                </Button>
              )}
            </>
          )}
        </div>
        {editing && (
          <>
            <div className="flex items-center justify-between">
              <Label>Active</Label>
              <Switch checked={isActive} onChange={setIsActive} />
            </div>
            {outboundEnabled && webhook!.outbound_url && outboundSignEnabled && (
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
            {inboundEnabled && (
              <div className="space-y-3 border-t border-gray-100 pt-3">
                <div className="flex items-center justify-between">
                  <p className="text-xs font-semibold uppercase tracking-wide text-gray-600">Inbound actions</p>
                  <Button size="sm" onClick={() => setActionDrawer(null)}><Plus className="h-3.5 w-3.5" /> Add action</Button>
                </div>
                {(actions ?? []).length === 0 ? (
                  <p className="text-sm text-gray-500">No actions configured.</p>
                ) : (
                  <Table>
                    <THead><tr><TH>Conditions</TH><TH>Action</TH><TH className="min-w-0 w-12" /></tr></THead>
                    <TBody>
                      {(actions ?? []).map((a) => (
                        <TR key={a.id}>
                          <TD className="text-xs text-gray-600">{conditionSummary(a.conditions ?? [], a.condition_logic)}</TD>
                          <TD><Badge>{a.action}</Badge></TD>
                          <TD>
                            <div className="flex justify-end gap-1">
                              <IconButton aria-label="Edit action" onClick={() => setActionDrawer(a)}><Pencil className="h-4 w-4" /></IconButton>
                              <IconButton variant="danger" onClick={() => deleteAction.mutate({ webhookId: webhook!.id, eventId: a.id }, { onError: (e) => toast.error(errorMessage(e)) })}>
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
            {outboundEnabled && (
              <div className="space-y-4 border-t border-gray-100 pt-3">
                {outboundFormat === "json" ? (
                  <OutboundPayloadTemplateEditor value={payloadTemplate} onChange={setPayloadTemplate} />
                ) : (
                  <OutboundFieldMapping entries={fieldMap} onChange={setFieldMap} />
                )}
                <OutboundResponseMapping entries={responseMap} onChange={setResponseMap} />
                <div className="space-y-3 border-t border-gray-100 pt-3">
                <div className="flex items-center justify-between">
                  <p className="text-xs font-semibold uppercase tracking-wide text-gray-600">Outbound triggers</p>
                  <Button size="sm" onClick={() => setTriggerDrawer(null)}><Plus className="h-3.5 w-3.5" /> Add trigger</Button>
                </div>
                {(triggers ?? []).length === 0 ? (
                  <p className="text-sm text-gray-500">No outbound triggers configured.</p>
                ) : (
                  <Table>
                    <THead><tr><TH>Event</TH><TH>Active</TH><TH className="min-w-0 w-12" /></tr></THead>
                    <TBody>
                      {(triggers ?? []).map((t) => (
                        <TR key={t.id}>
                          <TD><Badge>{t.trigger_event}</Badge></TD>
                          <TD>{t.is_active ? "✓" : "—"}</TD>
                          <TD>
                            <div className="flex justify-end gap-1">
                              <IconButton aria-label="Edit" onClick={() => setTriggerDrawer(t)}><Zap className="h-4 w-4" /></IconButton>
                              <IconButton variant="danger" onClick={() => deleteTrigger.mutate({ webhookId: webhook!.id, triggerId: t.id }, { onError: (e) => toast.error(errorMessage(e)) })}>
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
              </div>
            )}
          </>
        )}
      </div>
      {editing && (
        <>
          <ActionDrawer
            webhookId={webhook!.id}
            action={actionDrawer ?? null}
            open={actionDrawer !== undefined}
            mappingContext={mappingContext}
            onClose={() => setActionDrawer(undefined)}
            onSaved={(saved) => {
              toast.success("Action saved");
              if (mappingContext?.deliveryId) {
                onMappingContextChange?.({ ...mappingContext, actionId: saved.id });
                setActionDrawer(saved);
              } else {
                setActionDrawer(undefined);
              }
            }}
          />
          <OutboundTriggerDrawer
            webhookId={webhook!.id}
            trigger={triggerDrawer ?? null}
            open={triggerDrawer !== undefined}
            onClose={() => setTriggerDrawer(undefined)}
          />
        </>
      )}
    </FormDrawer>
  );
}

function WebhookDetailDrawer({
  webhook,
  open,
  initialSecret,
  onClose,
  onMapFields,
  onSecretCached,
}: {
  webhook: Webhook | null;
  open: boolean;
  initialSecret?: string | null;
  onClose: () => void;
  onMapFields: (ctx: MappingContext) => void;
  onSecretCached: (webhookId: number, secret: string) => void;
}) {
  const [expandedId, setExpandedId] = useState<number | null>(null);
  const [secret, setSecret] = useState<string | null>(initialSecret ?? null);
  const [showSecret, setShowSecret] = useState(false);
  const rotate = useRotateWebhookSecret();
  const replay = useReplayWebhookDelivery();

  useEffect(() => {
    setSecret(initialSecret ?? null);
    setShowSecret(false);
  }, [webhook, initialSecret]);

  const { data: deliveries, refetch: refetchDeliveries } = useWebhookDeliveries(webhook?.id ?? null);
  const { data: expandedDelivery } = useWebhookDelivery(webhook?.id ?? null, expandedId);

  if (!open || !webhook) return null;

  function handleRotate() {
    rotate.mutate(webhook!.id, {
      onSuccess: (res) => {
        setSecret(res.secret);
        setShowSecret(true);
        onSecretCached(webhook!.id, res.secret);
        toast.success("Secret rotated");
      },
      onError: (e) => toast.error(errorMessage(e)),
    });
  }

  return (
    <FormDrawer open onClose={onClose} title={webhook.name} subtitle={`Webhook · ${webhook.slug}`} width={720}>
      <div className="space-y-4">
        {webhook.inbound_enabled && (
          <InboundEndpointRows
            slug={webhook.slug}
            secret={secret}
            secretPrefix={webhook.secret_prefix}
            secretRequired={webhook.inbound_secret_required}
            showSecret={showSecret}
            onToggleSecret={() => setShowSecret((v) => !v)}
            onCopySecret={() => copyText(secret!, "Secret copied")}
            onRotate={handleRotate}
            rotatePending={rotate.isPending}
          />
        )}

        <div className="space-y-2">
          <p className="text-xs font-semibold uppercase tracking-wide text-gray-600">Delivery log</p>
          {(deliveries?.items ?? []).length === 0 ? (
            <EmptyState title="No deliveries yet." />
          ) : (
            <Table>
              <THead>
                <tr><TH>Time</TH><TH>Status</TH><TH>Lead</TH><TH className="min-w-0 w-12" /></tr>
              </THead>
              <TBody>
                {(deliveries?.items ?? []).map((d) => (
                  <Fragment key={d.id}>
                    <TR>
                      <TD className="text-xs">{format(new Date(d.created_at), "MMM d h:mma")}</TD>
                      <TD><Badge>{webhookDeliveryStatusLabel(d.status)}</Badge></TD>
                      <TD className="font-mono text-xs">{d.lead_public_id ?? "—"}</TD>
                      <TD>
                        <div className="flex justify-end gap-1">
                          <Button size="sm" variant="secondary" onClick={() => setExpandedId(expandedId === d.id ? null : d.id)}>
                            {expandedId === d.id ? "Hide" : "View"}
                          </Button>
                          <Button
                            size="sm"
                            variant="secondary"
                            onClick={() => onMapFields({ deliveryId: d.id })}
                          >
                            Actions
                          </Button>
                          {canReplayDelivery(d) && (
                            <Button
                              size="sm"
                              disabled={replay.isPending}
                              onClick={() =>
                                replay.mutate(
                                  { webhookId: webhook.id, deliveryId: d.id },
                                  {
                                    onSuccess: () => { toast.success("Replayed"); refetchDeliveries(); },
                                    onError: (e) => toast.error(errorMessage(e)),
                                  }
                                )
                              }
                            >
                              Run again
                            </Button>
                          )}
                        </div>
                      </TD>
                    </TR>
                    {expandedId === d.id && (
                      <tr>
                        <td colSpan={4} className="px-4 py-2">
                          <pre className="max-h-48 overflow-auto rounded-md border border-gray-100 bg-gray-50 p-3 font-mono text-xs">
                            {JSON.stringify(expandedDelivery?.request_payload ?? {}, null, 2)}
                          </pre>
                          {d.error_message && <p className="mt-1 text-xs text-red-600">{d.error_message}</p>}
                        </td>
                      </tr>
                    )}
                  </Fragment>
                ))}
              </TBody>
            </Table>
          )}
        </div>
      </div>
    </FormDrawer>
  );
}

function ActionDrawer({
  webhookId,
  action,
  open,
  mappingContext,
  onClose,
  onSaved,
}: {
  webhookId: number;
  action: WebhookEvent | null;
  open: boolean;
  mappingContext?: MappingContext;
  onClose: () => void;
  onSaved?: (action: WebhookEvent) => void;
}) {
  if (!open) return null;
  const editing = action !== null;
  const create = useCreateWebhookEvent();
  const update = useUpdateWebhookEvent();
  const { data: pipelines } = usePipelines();
  const { data: sample } = useWebhookSamplePayload(webhookId, true);
  const { data: deliveryPayload } = useWebhookDelivery(
    webhookId,
    mappingContext?.deliveryId ?? null
  );

  const [actionType, setActionType] = useState<WebhookEvent["action"]>("create");
  const [duplicateMode, setDuplicateMode] = useState<"update" | "duplicate" | "reject">("reject");
  const [lookupBy, setLookupBy] = useState<NonNullable<WebhookEvent["lookup_by"]>>("external_id");
  const [lookupSourceKey, setLookupSourceKey] = useState("");
  const [targetPipelineId, setTargetPipelineId] = useState<number | "">("");
  const [targetStageId, setTargetStageId] = useState<number | "">("");
  const [conditionLogic, setConditionLogic] = useState<"and" | "or">("and");
  const [conditions, setConditions] = useState<InboundCondition[]>([]);
  const [savedActionId, setSavedActionId] = useState<number | null>(action?.id ?? null);

  const pipelineId = typeof targetPipelineId === "number" ? targetPipelineId : undefined;
  const { data: stages } = useStages(pipelineId);

  useEffect(() => {
    if (action) {
      setActionType(action.action);
      setDuplicateMode(action.duplicate_mode ?? "reject");
      setLookupBy(action.lookup_by ?? "external_id");
      setLookupSourceKey(action.lookup_source_key ?? "");
      setTargetPipelineId(action.target_pipeline_id ?? "");
      setTargetStageId(action.target_stage_id ?? "");
      setConditionLogic(action.condition_logic ?? "and");
      setConditions(action.conditions ?? []);
      setSavedActionId(action.id);
    } else {
      setActionType("create");
      setDuplicateMode("reject");
      setLookupBy("external_id");
      setLookupSourceKey("");
      setTargetPipelineId("");
      setTargetStageId("");
      setConditionLogic("and");
      setConditions([]);
      setSavedActionId(null);
    }
  }, [action]);

  const payload =
    mappingContext?.payload ??
    deliveryPayload?.request_payload ??
    sample?.payload ??
    null;
  const mappableKeys = payload ? mappablePayloadKeys(payload) : [];
  const fieldMapActionId = savedActionId ?? action?.id ?? null;

  function buildBody(): Record<string, unknown> {
    const body: Record<string, unknown> = {
      action: actionType,
      condition_logic: conditionLogic,
      conditions,
    };
    if (actionType === "create") {
      body.duplicate_mode = duplicateMode;
      if (targetPipelineId) body.target_pipeline_id = targetPipelineId;
    } else {
      body.lookup_by = lookupBy;
      if (lookupSourceKey) body.lookup_source_key = lookupSourceKey;
    }
    if (actionType === "move_stage" && targetStageId) body.target_stage_id = targetStageId;
    return body;
  }

  function save() {
    const body = buildBody();
    if (editing && action) {
      update.mutate({ webhookId, eventId: action.id, body }, {
        onSuccess: (res: WebhookEvent) => { setSavedActionId(res.id); onSaved?.(res); },
        onError: (e: unknown) => toast.error(errorMessage(e)),
      });
    } else {
      create.mutate({ webhookId, body }, {
        onSuccess: (res: WebhookEvent) => { setSavedActionId(res.id); onSaved?.(res); },
        onError: (e: unknown) => toast.error(errorMessage(e)),
      });
    }
  }

  const saving = create.isPending || update.isPending;

  return (
    <FormDrawer
      open
      onClose={onClose}
      title={editing ? "Edit action" : "New action"}
      subtitle="Conditions and field mapping"
      width={640}
      footer={
        <>
          <Button variant="secondary" onClick={onClose}>Cancel</Button>
          <Button disabled={saving} onClick={save}>{editing ? "Save" : "Create"}</Button>
        </>
      }
    >
      <div className="space-y-4">
        <div>
          <Label>Action</Label>
          <select className="w-full rounded-md border border-gray-200 px-3 py-2 text-sm" value={actionType} onChange={(e) => setActionType(e.target.value as WebhookEvent["action"])}>
            <option value="create">Create lead</option>
            <option value="update">Update lead</option>
            <option value="delete">Delete lead (soft)</option>
            <option value="move_stage">Move to stage</option>
          </select>
        </div>

        <div className="space-y-2 rounded-md border border-gray-100 p-3">
          <div className="flex items-center justify-between">
            <Label>Conditions</Label>
            <select className="rounded border border-gray-200 px-2 py-1 text-xs" value={conditionLogic} onChange={(e) => setConditionLogic(e.target.value as "and" | "or")}>
              <option value="and">AND</option>
              <option value="or">OR</option>
            </select>
          </div>
          <div className="flex flex-col gap-2">
            {conditions.map((c, i) => (
              <InboundConditionRow
                key={i}
                condition={c}
                mappableKeys={mappableKeys}
                onChange={(next) => setConditions((prev) => prev.map((x, j) => (j === i ? next : x)))}
                onRemove={() => setConditions((prev) => prev.filter((_, j) => j !== i))}
              />
            ))}
            <Button
              size="sm"
              variant="outline"
              onClick={() => setConditions((prev) => [...prev, { field: mappableKeys[0] ?? "", op: "eq", value: "" }])}
            >
              <Plus className="h-3.5 w-3.5" /> Add condition
            </Button>
          </div>
          <p className="text-xs text-gray-400">No conditions = always matches (catch-all)</p>
        </div>

        {actionType === "create" && (
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

        {actionType !== "create" && (
          <div className="space-y-2">
            <div>
              <Label>Lookup lead by</Label>
              <select className="w-full rounded-md border border-gray-200 px-3 py-2 text-sm" value={lookupBy} onChange={(e) => setLookupBy(e.target.value as NonNullable<WebhookEvent["lookup_by"]>)}>
                <option value="phone">phone</option>
                <option value="email">email</option>
                <option value="external_id">external_id</option>
                <option value="public_id">public_id</option>
              </select>
            </div>
            <div>
              <Label>Lookup value from payload key</Label>
              <select className="w-full rounded-md border border-gray-200 px-3 py-2 text-sm" value={lookupSourceKey} onChange={(e) => setLookupSourceKey(e.target.value)}>
                <option value="">Select payload key</option>
                {mappableKeys.map((k) => <option key={k} value={k}>{k}</option>)}
              </select>
            </div>
          </div>
        )}

        {actionType === "move_stage" && (
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

        {(actionType === "create" || actionType === "update") && fieldMapActionId && (
          <ActionFieldMapping webhookId={webhookId} actionId={fieldMapActionId} payload={payload} />
        )}
        {(actionType === "create" || actionType === "update") && !fieldMapActionId && (
          <p className="text-sm text-gray-500">Save the action first, then map fields.</p>
        )}
      </div>
    </FormDrawer>
  );
}

function ActionFieldMapping({
  webhookId,
  actionId,
  payload,
}: {
  webhookId: number;
  actionId: number;
  payload: Record<string, unknown> | null;
}) {
  const { data: entries } = useWebhookFieldMap(webhookId, actionId);
  const { data: customFields } = useCustomFields();
  const add = useAddWebhookFieldMap();
  const remove = useDeleteWebhookFieldMap();
  const createField = useCreateField();

  const [sourceKey, setSourceKey] = useState("");
  const [target, setTarget] = useState("first_name");
  const [createFieldOpen, setCreateFieldOpen] = useState(false);

  const mappableKeys = payload ? mappablePayloadKeys(payload) : [];
  const mapped = mappedKeys(entries ?? []);
  const unmappedKeys = mappableKeys.filter((k) => !mapped.has(k));

  const suggestions = useMemo(
    () => buildPayloadSuggestions(unmappedKeys, customFields ?? []),
    [unmappedKeys, customFields]
  );

  function selectSourceKey(key: string) {
    setSourceKey(key);
    if (suggestions[key]) setTarget(suggestions[key]);
  }

  useEffect(() => {
    if (sourceKey && suggestions[sourceKey]) {
      setTarget(suggestions[sourceKey]);
    }
  }, [sourceKey, suggestions]);

  function addMapping(key: string, targetVal: string) {
    const isCustom = targetVal.startsWith("cf:");
    const body: Record<string, unknown> = isCustom
      ? { source_key: key, target_type: "custom", custom_field_id: Number(targetVal.slice(3)) }
      : { source_key: key, target_type: "builtin", builtin_field: targetVal };
    add.mutate({ webhookId, eventId: actionId, body }, {
      onSuccess: () => setSourceKey(""),
      onError: (e) => toast.error(errorMessage(e)),
    });
  }

  return (
    <div className="space-y-3 rounded-md border border-gray-100 p-3">
      <Label>Field mapping</Label>
      {payload ? (
        <pre className="max-h-32 overflow-auto rounded-md border border-gray-100 bg-gray-50 p-2 font-mono text-xs">{JSON.stringify(payload, null, 2)}</pre>
      ) : (
        <p className="text-sm text-gray-500">No sample payload yet — send a webhook or use Actions from delivery log.</p>
      )}
      {unmappedKeys.length > 0 && (
        <div className="flex flex-wrap gap-1.5">
          {unmappedKeys.map((k) => (
            <button key={k} type="button" onClick={() => selectSourceKey(k)} className="rounded-full border border-jade-200 bg-jade-50 px-2 py-0.5 font-mono text-xs">{k}</button>
          ))}
        </div>
      )}
      <div className="flex gap-2 items-end">
        <div className="flex-1">
          <Input value={sourceKey} onChange={(e) => setSourceKey(e.target.value)} placeholder="payload key" />
        </div>
        <div>
          <BuiltinCustomFieldSelect
            label="Lead field"
            builtins={BUILTINS}
            customFields={customFields ?? []}
            value={target}
            onChange={setTarget}
            onAddCustomField={() => setCreateFieldOpen(true)}
          />
          {sourceKey && suggestions[sourceKey] && target === suggestions[sourceKey] && (
            <p className="mt-0.5 text-xs text-gray-400">Suggested</p>
          )}
        </div>
        <Button disabled={!sourceKey} onClick={() => addMapping(sourceKey, target)}>Map</Button>
      </div>
      {(entries ?? []).length > 0 && (
        <Table>
          <THead><tr><TH>Payload key</TH><TH>Lead field</TH><TH className="min-w-0 w-12" /></tr></THead>
          <TBody>
            {(entries ?? []).map((e) => (
              <TR key={e.id}>
                <TD className="font-mono text-xs">{e.source_key}</TD>
                <TD className="text-xs">{e.target_type === "builtin" ? e.builtin_field : `custom #${e.custom_field_id}`}</TD>
                <TD><IconButton variant="danger" onClick={() => remove.mutate(e.id)}><Trash2 className="h-4 w-4" /></IconButton></TD>
              </TR>
            ))}
          </TBody>
        </Table>
      )}
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
    </div>
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

function resolvePayloadTemplate(template?: string): string {
  if (!template || template === "{}") return DEFAULT_TEMPLATE;
  return template;
}

type TemplateVariable = {
  key: string;
  label?: string;
  searchText: string;
};

function OutboundPayloadTemplateEditor({
  value,
  onChange,
}: {
  value: string;
  onChange: (v: string) => void;
}) {
  const { data: customFields = [] } = useCustomFields();
  const [templateCursor, setTemplateCursor] = useState<HTMLTextAreaElement | null>(null);
  const [variablesOpen, setVariablesOpen] = useState(false);
  const [variableSearch, setVariableSearch] = useState("");
  const selectionRef = useRef({ start: 0, end: 0 });

  function saveSelection(el: HTMLTextAreaElement) {
    selectionRef.current = { start: el.selectionStart, end: el.selectionEnd };
  }

  function insertField(field: string) {
    if (templateCursor) {
      const { start, end } = selectionRef.current;
      const before = value.slice(0, start);
      const after = value.slice(end);
      onChange(before + field + after);
      const cursor = start + field.length;
      selectionRef.current = { start: cursor, end: cursor };
      setTimeout(() => {
        templateCursor.selectionStart = cursor;
        templateCursor.selectionEnd = cursor;
        templateCursor.focus();
      }, 0);
    } else {
      onChange(value + field);
    }
  }

  function selectVariable(field: string) {
    insertField(field);
    setVariablesOpen(false);
    setVariableSearch("");
  }

  const staticVariables: TemplateVariable[] = TEMPLATE_FIELDS.map((f) => ({
    key: f,
    searchText: f,
  }));

  const customVariables: TemplateVariable[] = customFields
    .filter((f) => f.is_active !== false)
    .map((f) => {
      const key = `{{lead.custom.${f.id}}}`;
      return {
        key,
        label: f.name,
        searchText: `${key} ${f.name} ${f.field_key}`,
      };
    });

  const q = variableSearch.toLowerCase();
  const filteredStatic = staticVariables.filter((v) => v.searchText.toLowerCase().includes(q));
  const filteredCustom = customVariables.filter((v) => v.searchText.toLowerCase().includes(q));
  const hasResults = filteredStatic.length > 0 || filteredCustom.length > 0;

  return (
    <div>
      <div className="mb-2 flex items-center justify-between">
        <Label>JSON payload template</Label>
        <div className="flex items-center gap-3">
          <Dropdown
            open={variablesOpen}
            onClose={() => {
              setVariablesOpen(false);
              setVariableSearch("");
            }}
            align="right"
            className="max-h-48 min-w-[260px] overflow-y-auto"
            trigger={
              <button
                type="button"
                onClick={() => setVariablesOpen(!variablesOpen)}
                className="flex items-center gap-1 text-xs text-indigo-600 hover:text-indigo-800"
              >
                <Plus className="h-3 w-3" /> Add Variables
              </button>
            }
          >
            <DropdownSearch
              value={variableSearch}
              onChange={setVariableSearch}
              placeholder="Search variables…"
            />
            {!hasResults ? (
              <p className="px-2.5 py-2 text-xs text-gray-400">No variables match</p>
            ) : (
              <>
                {filteredStatic.map((v) => (
                  <DropdownItem key={v.key} onClick={() => selectVariable(v.key)} className="font-mono text-xs">
                    {v.key}
                  </DropdownItem>
                ))}
                {filteredStatic.length > 0 && filteredCustom.length > 0 && (
                  <div className="my-1 border-t border-gray-100 px-2.5 py-1 text-xs font-semibold uppercase tracking-wide text-gray-400">
                    Custom fields
                  </div>
                )}
                {filteredCustom.map((v) => (
                  <DropdownItem key={v.key} onClick={() => selectVariable(v.key)} className="h-auto py-2">
                    <div className="text-xs text-gray-700">{v.label}</div>
                    <div className="font-mono text-xs text-gray-400">{v.key}</div>
                  </DropdownItem>
                ))}
              </>
            )}
          </Dropdown>
          <p className="text-xs text-gray-500">Use {"{{field}}"} placeholders</p>
        </div>
      </div>
      <textarea
        ref={(el) => setTemplateCursor(el)}
        rows={12}
        value={value}
        onChange={(e) => onChange(e.target.value)}
        onSelect={(e) => saveSelection(e.currentTarget)}
        onBlur={(e) => saveSelection(e.currentTarget)}
        onFocus={(e) => saveSelection(e.currentTarget)}
        className="w-full rounded-md border border-gray-200 bg-white px-3 py-2 font-mono text-xs focus:outline-none focus:ring-2 focus:ring-jade-500"
        spellCheck={false}
      />
    </div>
  );
}

function OutboundFieldMapping({
  entries,
  onChange,
}: {
  entries: OutboundFieldMapEntry[];
  onChange: (entries: OutboundFieldMapEntry[]) => void;
}) {
  const { data: customFields = [] } = useCustomFields();

  function addRow() {
    onChange([...entries, { dest_key: "", source_type: "builtin", builtin_field: "last_name" }]);
  }

  function removeRow(idx: number) {
    onChange(entries.filter((_, i) => i !== idx));
  }

  function updateRow(idx: number, patch: Partial<OutboundFieldMapEntry>) {
    const next = [...entries];
    next[idx] = { ...next[idx], ...patch };
    onChange(next);
  }

  return (
    <div className="space-y-3">
      <div className="flex items-center justify-between">
        <Label>URL parameter mapping</Label>
        <Button size="sm" variant="secondary" onClick={addRow}><Plus className="h-3.5 w-3.5" /> Add param</Button>
      </div>
      <p className="text-xs text-gray-500">Map external query parameter names to lead fields or static values.</p>
      {entries.length === 0 ? (
        <p className="text-sm text-gray-500">No parameters mapped yet.</p>
      ) : (
        <Table>
          <THead><tr><TH>Param name</TH><TH>Source</TH><TH>Value</TH><TH className="min-w-0 w-12" /></tr></THead>
          <TBody>
            {entries.map((e, idx) => (
              <TR key={idx}>
                <TD>
                  <Input
                    value={e.dest_key}
                    onChange={(ev) => updateRow(idx, { dest_key: ev.target.value })}
                    placeholder="last_name"
                    className="font-mono text-xs"
                  />
                </TD>
                <TD>
                  <select
                    className="rounded border border-gray-200 px-2 py-1 text-xs"
                    value={e.source_type}
                    onChange={(ev) => {
                      const source_type = ev.target.value as OutboundFieldMapEntry["source_type"];
                      const patch: Partial<OutboundFieldMapEntry> = { source_type };
                      if (source_type === "builtin") patch.builtin_field = "last_name";
                      if (source_type === "static") patch.static_value = "";
                      if (source_type === "meta") patch.meta_field = "event";
                      updateRow(idx, patch);
                    }}
                  >
                    <option value="builtin">Lead field</option>
                    <option value="custom">Custom field</option>
                    <option value="static">Static value</option>
                    <option value="meta">Event meta</option>
                  </select>
                </TD>
                <TD>
                  {e.source_type === "builtin" && (
                    <select
                      className="w-full rounded border border-gray-200 px-2 py-1 text-xs"
                      value={e.builtin_field ?? "last_name"}
                      onChange={(ev) => updateRow(idx, { builtin_field: ev.target.value })}
                    >
                      {OUTBOUND_BUILTINS.map((b) => <option key={b} value={b}>{b}</option>)}
                    </select>
                  )}
                  {e.source_type === "custom" && (
                    <select
                      className="w-full rounded border border-gray-200 px-2 py-1 text-xs"
                      value={e.custom_field_id ?? ""}
                      onChange={(ev) => updateRow(idx, { custom_field_id: Number(ev.target.value) })}
                    >
                      <option value="">Select field</option>
                      {customFields.filter((f) => f.is_active !== false).map((f) => (
                        <option key={f.id} value={f.id}>{f.name}</option>
                      ))}
                    </select>
                  )}
                  {e.source_type === "static" && (
                    <Input
                      value={e.static_value ?? ""}
                      onChange={(ev) => updateRow(idx, { static_value: ev.target.value })}
                      placeholder="YourSchema"
                      className="text-xs"
                    />
                  )}
                  {e.source_type === "meta" && (
                    <select
                      className="w-full rounded border border-gray-200 px-2 py-1 text-xs"
                      value={e.meta_field ?? "event"}
                      onChange={(ev) => updateRow(idx, { meta_field: ev.target.value })}
                    >
                      {OUTBOUND_META_FIELDS.map((m) => <option key={m.value} value={m.value}>{m.label}</option>)}
                    </select>
                  )}
                </TD>
                <TD>
                  <IconButton variant="danger" aria-label="Remove" onClick={() => removeRow(idx)}>
                    <Trash2 className="h-4 w-4" />
                  </IconButton>
                </TD>
              </TR>
            ))}
          </TBody>
        </Table>
      )}
    </div>
  );
}

function OutboundResponseMapping({
  entries,
  onChange,
}: {
  entries: ResponseMapEntry[];
  onChange: (entries: ResponseMapEntry[]) => void;
}) {
  const { data: customFields = [] } = useCustomFields();

  function addRow() {
    onChange([...entries, { response_key: "", target_type: "builtin", builtin_field: "external_id" }]);
  }

  function removeRow(idx: number) {
    onChange(entries.filter((_, i) => i !== idx));
  }

  function updateRow(idx: number, field: string, val: string) {
    const next = [...entries];
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
    onChange(next);
  }

  function targetValue(entry: ResponseMapEntry): string {
    if (entry.target_type === "custom" && entry.custom_field_id !== undefined) {
      return `cf:${entry.custom_field_id}`;
    }
    return entry.builtin_field ?? "external_id";
  }

  return (
    <div>
      <div className="mb-2 flex items-center justify-between">
        <Label>Response mapping</Label>
        <button
          type="button"
          onClick={addRow}
          className="flex items-center gap-1 text-xs text-indigo-600 hover:text-indigo-800"
        >
          <Plus className="h-3 w-3" /> Add field
        </button>
      </div>
      {entries.length === 0 ? (
        <p className="text-xs text-gray-400">
          Map fields from the external API response back to the lead. Example: extract{" "}
          <code className="font-mono">data.id</code> and write it to{" "}
          <code className="font-mono">external_id</code>.
        </p>
      ) : (
        <div className="space-y-2">
          {entries.map((entry, idx) => (
            <div key={idx} className="flex items-center gap-2">
              <input
                type="text"
                placeholder="data.id"
                value={entry.response_key}
                onChange={(e) => updateRow(idx, "response_key", e.target.value)}
                className="w-32 rounded border border-gray-200 px-2 py-1 font-mono text-xs focus:outline-none focus:ring-1 focus:ring-jade-500"
              />
              <span className="text-xs text-gray-400">→</span>
              <select
                value={targetValue(entry)}
                onChange={(e) => updateRow(idx, "target", e.target.value)}
                className="flex-1 rounded border border-gray-200 bg-white px-2 py-1 text-xs focus:outline-none focus:ring-1 focus:ring-jade-500"
              >
                <optgroup label="Built-in">
                  {["first_name", "last_name", "phone", "email", "address", "city", "state", "zip", "source", "external_id", "tags"].map((b) => (
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
                onClick={() => removeRow(idx)}
                className="text-gray-400 hover:text-red-500"
              >
                <Trash2 className="h-3.5 w-3.5" />
              </button>
            </div>
          ))}
        </div>
      )}
    </div>
  );
}

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

  const [triggerEvent, setTriggerEvent] = useState<OutboundTriggerEvent>(
    trigger?.trigger_event ?? "lead.create"
  );
  const [conditionLogic, setConditionLogic] = useState<"and" | "or">(
    trigger?.condition_logic ?? "and"
  );
  const [isActive, setIsActive] = useState(trigger?.is_active ?? true);

  useEffect(() => {
    setTriggerEvent(trigger?.trigger_event ?? "lead.create");
    setConditionLogic(trigger?.condition_logic ?? "and");
    setIsActive(trigger?.is_active ?? true);
  }, [trigger]);

  function save() {
    const body = {
      trigger_event: triggerEvent,
      condition_logic: conditionLogic,
      conditions: [],
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
      subtitle="Configure when this webhook fires"
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

        <div className="rounded-md border border-gray-100 bg-gray-50 px-3 py-2 text-xs text-gray-500">
          <p className="font-semibold text-gray-700 mb-1">Conditions</p>
          <p>Condition builder is available for advanced filtering — all triggers fire unconditionally by default. Configure the JSON payload template and response mapping in Edit Webhook.</p>
        </div>
      </div>
    </FormDrawer>
  );
}
