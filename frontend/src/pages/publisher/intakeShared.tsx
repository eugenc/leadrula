import { useEffect, useMemo, useState } from "react";
import {
  useMapQueueField,
  useRouteQueue,
  useBuyers,
  useSources,
  useRoutes,
  useCreateField,
} from "@/features/admin/hooks";
import { useCustomFields } from "@/features/leads/hooks";
import { payloadValuePreview } from "@/features/intake/payloadKeys";
import { BuiltinCustomFieldSelect } from "@/features/admin/BuiltinCustomFieldSelect";
import { CreateCustomFieldDrawer } from "@/features/admin/CreateCustomFieldDrawer";
import { slugFieldKey } from "@/features/admin/customFieldConstants";
import { buildPayloadSuggestions, MAP_BUILTIN_FIELDS } from "@/features/leads/csvMapping";
import { Button } from "@/components/ui/button";
import { Label, Select } from "@/components/ui/input";
import { Dialog, FormDrawer } from "@/components/ui/dialog";
import { toast } from "@/store/toastStore";
import { errorMessage } from "@/lib/api";
import { RerunIntakeButton } from "@/features/intake/RerunIntakeButton";
import type { QueueItem, Route } from "@/types";

export { LOG_FILTERS, PAGE_SIZES, statusBadge, type LogFilter } from "@/features/intake/logShared";

export const BUILTINS = MAP_BUILTIN_FIELDS;

function routeLabel(rt: Route): string {
  if (rt.destination === "contract") {
    const buyer = rt.buyer_name ?? rt.contract_name ?? "Buyer";
    const stage = rt.target_stage_name ? ` → ${rt.target_stage_name}` : "";
    return `${buyer} (buyer pipeline${stage})`;
  }
  const pipeline = rt.target_pipeline_name ?? "Your pipeline";
  const stage = rt.target_stage_name ? ` → ${rt.target_stage_name}` : "";
  return `${pipeline} (your pipeline${stage})`;
}

export function RouteDialog({ item, onClose }: { item: QueueItem; onClose: () => void }) {
  const { data: buyers } = useBuyers();
  const { data: sources } = useSources();
  const { data: routes } = useRoutes();
  const routeQueue = useRouteQueue();
  const [selectedRouteId, setSelectedRouteId] = useState(0);
  const [buyerId, setBuyerId] = useState(0);

  const sourceId = useMemo(() => {
    if (!item.source) return null;
    return (sources ?? []).find((s) => s.slug === item.source)?.id ?? null;
  }, [item.source, sources]);

  const sourceRoutes = useMemo(() => {
    if (!sourceId) return [];
    return (routes ?? []).filter((rt) => rt.is_active && rt.source_id === sourceId);
  }, [routes, sourceId]);

  const useFallback = sourceRoutes.length === 0;
  const canSubmit = useFallback ? buyerId > 0 : selectedRouteId > 0;

  function submit() {
    const body = useFallback ? { buyer_id: buyerId } : { route_id: selectedRouteId };
    routeQueue.mutate(
      { id: item.id, body },
      {
        onSuccess: () => {
          toast.success("Lead routed");
          onClose();
        },
        onError: (e) => toast.error(errorMessage(e)),
      }
    );
  }

  return (
    <Dialog open onClose={onClose} title={`Route ${item.first_name} ${item.last_name}`}>
      <div className="space-y-3">
        {sourceRoutes.length > 0 ? (
          <div className="space-y-2">
            <Label>Route</Label>
            {sourceRoutes.map((rt) => (
              <label
                key={rt.id}
                className="flex cursor-pointer items-start gap-2 rounded-md border border-gray-100 p-3 hover:bg-gray-50"
              >
                <input
                  type="radio"
                  name="route"
                  className="mt-0.5"
                  checked={selectedRouteId === rt.id}
                  onChange={() => setSelectedRouteId(rt.id)}
                />
                <div>
                  <p className="text-sm font-medium text-gray-800">{rt.name}</p>
                  <p className="text-xs text-gray-500">{routeLabel(rt)}</p>
                </div>
              </label>
            ))}
          </div>
        ) : (
          <>
            <p className="text-sm text-gray-500">
              {item.source
                ? `No active routes configured for source "${item.source}". Pick a buyer manually.`
                : "No source slug on this lead. Pick a buyer manually."}
            </p>
            <div>
              <Label>Send to buyer</Label>
              <Select value={buyerId} onChange={(e) => setBuyerId(Number(e.target.value))}>
                <option value={0}>Select a buyer…</option>
                {(buyers ?? []).map((b) => (
                  <option key={b.id} value={b.id}>
                    {b.name}
                  </option>
                ))}
              </Select>
              <p className="mt-1 text-xs text-gray-400">
                The lead lands in the buyer's contract pipeline and the buyer is charged the contract rate.
              </p>
            </div>
          </>
        )}
        <div className="flex justify-end gap-2 pt-2">
          <Button variant="secondary" onClick={onClose}>
            Cancel
          </Button>
          <Button disabled={!canSubmit || routeQueue.isPending} onClick={submit}>
            Route Lead
          </Button>
        </div>
      </div>
    </Dialog>
  );
}

export function QueueItemDrawer({
  item,
  onClose,
  readOnly = false,
  onUpdated,
  onRoute,
  onReject,
}: {
  item: QueueItem;
  onClose: () => void;
  readOnly?: boolean;
  onUpdated?: (item: QueueItem) => void;
  onRoute?: () => void;
  onReject?: () => void;
}) {
  const { data: sources } = useSources();
  const { data: customFields } = useCustomFields();
  const mapField = useMapQueueField();
  const createField = useCreateField();
  const [targets, setTargets] = useState<Record<string, string>>({});
  const [createFieldKey, setCreateFieldKey] = useState<string | null>(null);
  const [savingKey, setSavingKey] = useState<string | null>(null);

  if (readOnly) {
    return (
      <FormDrawer
        open
        onClose={onClose}
        title={`${item.first_name} ${item.last_name}`}
        subtitle={item.source ?? undefined}
        width={560}
      >
        <div>
          <Label>Payload</Label>
          <pre className="mt-1.5 max-h-96 overflow-auto rounded-md border border-gray-100 bg-gray-50 p-3 font-mono text-xs text-gray-800">
            {JSON.stringify(item.raw_payload ?? {}, null, 2)}
          </pre>
        </div>
      </FormDrawer>
    );
  }

  const sourceRegistered = !!(item.source && (sources ?? []).some((s) => s.slug === item.source));
  const unmapped = item.unmapped_keys ?? [];

  const suggestions = useMemo(
    () => buildPayloadSuggestions(unmapped, customFields ?? []),
    [unmapped, customFields]
  );

  useEffect(() => {
    setTargets((prev) => {
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

  function targetFor(key: string) {
    return targets[key] ?? suggestions[key] ?? "first_name";
  }

  function isSuggested(key: string) {
    return !!suggestions[key] && targetFor(key) === suggestions[key];
  }

  function saveMapping(key: string, targetOverride?: string) {
    const target = targetOverride ?? targetFor(key);
    const isCustom = target.startsWith("cf:");
    const body: Record<string, unknown> = isCustom
      ? { source_key: key, target_type: "custom", custom_field_id: Number(target.slice(3)) }
      : { source_key: key, target_type: "builtin", builtin_field: target };
    setSavingKey(key);
    mapField.mutate(
      { id: item.id, body },
      {
        onSuccess: (updated) => {
          toast.success("Mapping saved");
          onUpdated?.(updated);
          setSavingKey(null);
        },
        onError: (e) => {
          toast.error(errorMessage(e));
          setSavingKey(null);
        },
      }
    );
  }

  function ignoreField(key: string) {
    setSavingKey(key);
    mapField.mutate(
      { id: item.id, body: { source_key: key, target_type: "ignore" } },
      {
        onSuccess: (updated) => {
          toast.success("Field ignored");
          onUpdated?.(updated);
          setSavingKey(null);
        },
        onError: (e) => {
          toast.error(errorMessage(e));
          setSavingKey(null);
        },
      }
    );
  }

  const pending = item.status === "pending_review";

  return (
    <>
      <FormDrawer
        open
        onClose={onClose}
        title={`${item.first_name} ${item.last_name}`}
        subtitle={item.source ?? undefined}
        width={560}
        footer={
          pending ? (
            <>
              <RerunIntakeButton item={item} onSuccess={onUpdated} />
              <Button variant="secondary" onClick={onReject}>
                Reject
              </Button>
              <Button onClick={onRoute}>Route</Button>
            </>
          ) : undefined
        }
      >
        <div className="space-y-4">
          <div>
            <Label>Payload</Label>
            <pre className="mt-1.5 max-h-48 overflow-auto rounded-md border border-gray-100 bg-gray-50 p-3 font-mono text-xs text-gray-800">
              {JSON.stringify(item.raw_payload ?? {}, null, 2)}
            </pre>
          </div>

          {!sourceRegistered && item.source && (
            <p className="text-sm text-gray-500">Mapping applies to this lead only — no registered source for this slug.</p>
          )}
          {!item.source && (
            <p className="text-sm text-gray-500">Mapping applies to this lead only — no source slug on this entry.</p>
          )}

          {unmapped.length > 0 ? (
            <div className="space-y-3">
              <Label>Unmapped fields</Label>
              {unmapped.map((key) => (
                <div key={key} className="rounded-md border border-gray-100 p-3">
                  <div className="mb-2">
                    <span className="font-mono text-sm font-medium text-gray-800">{key}</span>
                    <p className="mt-0.5 truncate text-xs text-gray-400">
                      {payloadValuePreview(item.raw_payload ?? {}, key)}
                    </p>
                  </div>
                  <div className="flex items-end gap-2">
                    <div className="flex-1">
                      <BuiltinCustomFieldSelect
                        value={targetFor(key)}
                        onChange={(v) => setTargets((t) => ({ ...t, [key]: v }))}
                        customFields={customFields ?? []}
                        builtins={BUILTINS}
                        label="Lead field"
                        onAddCustomField={() => setCreateFieldKey(key)}
                      />
                      {isSuggested(key) && (
                        <p className="mt-0.5 text-xs text-gray-400">Suggested</p>
                      )}
                    </div>
                    <Button size="sm" disabled={savingKey === key} onClick={() => saveMapping(key)}>
                      Save
                    </Button>
                    {sourceRegistered && (
                      <Button
                        size="sm"
                        variant="secondary"
                        disabled={savingKey === key}
                        onClick={() => ignoreField(key)}
                      >
                        Ignore
                      </Button>
                    )}
                  </div>
                </div>
              ))}
            </div>
          ) : (
            <p className="text-sm text-gray-400">All payload fields are mapped or ignored.</p>
          )}
        </div>
      </FormDrawer>

      <CreateCustomFieldDrawer
        open={createFieldKey !== null}
        onClose={() => setCreateFieldKey(null)}
        defaultName={createFieldKey?.replace(/_/g, " ") ?? ""}
        defaultFieldKey={createFieldKey ? slugFieldKey(createFieldKey) : ""}
        subtitle={createFieldKey ? `Payload key: ${createFieldKey}` : undefined}
        isPending={createField.isPending}
        onSubmit={(body) =>
          createField.mutateAsync(body).then((field) => {
            const key = createFieldKey;
            setCreateFieldKey(null);
            if (key) {
              setTargets((t) => ({ ...t, [key]: `cf:${field.id}` }));
              saveMapping(key, `cf:${field.id}`);
            }
            return field;
          })
        }
      />
    </>
  );
}
