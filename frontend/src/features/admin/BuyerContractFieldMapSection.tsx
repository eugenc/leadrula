import { useEffect, useMemo, useState } from "react";
import { Button } from "@/components/ui/button";
import { Label } from "@/components/ui/input";
import { IconButton } from "@/components/layout/IconButton";
import { BuiltinCustomFieldSelect } from "@/features/admin/BuiltinCustomFieldSelect";
import { CreateCustomFieldDrawer } from "@/features/admin/CreateCustomFieldDrawer";
import { slugFieldKey } from "@/features/admin/customFieldConstants";
import {
  useAddBuyerContractFieldMap,
  useAddBuyerParticipationFieldMap,
  useBuyerContractFieldMap,
  useBuyerContractFieldMapOptions,
  useBuyerParticipationFieldMap,
  useBuyerParticipationFieldMapOptions,
  useDeleteBuyerContractFieldMap,
  useDeleteBuyerParticipationFieldMap,
} from "@/features/admin/hooks";
import { buildPayloadSuggestions, MAP_BUILTIN_FIELDS } from "@/features/leads/csvMapping";
import { useCreateField } from "@/features/admin/hooks";
import { toast } from "@/store/toastStore";
import { errorMessage } from "@/lib/api";
import { Spinner } from "@/components/ui/misc";
import { Trash2 } from "lucide-react";
import type {
  ContractAvailableField,
  ContractFieldMapEntry,
  ContractFieldMapOptions,
} from "@/types";

function entrySourceKey(e: ContractFieldMapEntry): string | null {
  if (e.src_type === "custom" && e.src_custom_field_id != null) {
    return `cf:${e.src_custom_field_id}`;
  }
  if (e.src_type === "builtin" && e.src_builtin) {
    return e.src_builtin;
  }
  return null;
}

function mappedSourceKeys(entries: ContractFieldMapEntry[] | undefined): Set<string> {
  const keys = new Set<string>();
  for (const e of entries ?? []) {
    const k = entrySourceKey(e);
    if (k) keys.add(k);
  }
  return keys;
}

function entryMatchesAvailable(
  e: ContractFieldMapEntry,
  af: ContractFieldMapOptions["available_fields"][number]
): boolean {
  const k = entrySourceKey(e);
  return k != null && k === af.key;
}

export function fieldMappingComplete(
  options: ContractFieldMapOptions | undefined,
  entries: ContractFieldMapEntry[] | undefined
): boolean {
  const available = options?.available_fields ?? [];
  if (available.length === 0) return true;
  const mapped = mappedSourceKeys(entries);
  return available.every((af) => mapped.has(af.key));
}

function fieldBody(prefix: "src" | "dst", val: string) {
  if (val.startsWith("cf:")) {
    return { [`${prefix}_type`]: "custom", [`${prefix}_custom_field_id`]: Number(val.slice(3)) };
  }
  return { [`${prefix}_type`]: "builtin", [`${prefix}_builtin`]: val };
}

function srcBodyFromAvailable(af: ContractFieldMapOptions["available_fields"][number]) {
  if (af.field_type === "custom" && af.custom_field_id) {
    return { src_type: "custom", src_custom_field_id: af.custom_field_id };
  }
  return { src_type: "builtin", src_builtin: af.builtin_field ?? af.key };
}

function availableFieldLabel(af: ContractAvailableField): string {
  if (af.field_type === "custom") {
    return `${af.label} (custom)`;
  }
  return af.label;
}

function renderEntryLabel(e: ContractFieldMapEntry, options: ContractFieldMapOptions): string {
  const af = options.available_fields.find((a) => entryMatchesAvailable(e, a));
  const src =
    (af ? availableFieldLabel(af) : null) ??
    (e.src_type === "custom" ? `Custom #${e.src_custom_field_id}` : e.src_builtin);
  const dst =
    e.dst_type === "custom"
      ? options.buyer_fields.find((f) => f.id === e.dst_custom_field_id)?.name ?? `Custom #${e.dst_custom_field_id}`
      : e.dst_builtin;
  return `${src} → ${dst}`;
}

export function BuyerContractFieldMapSection({
  contractId,
  participationId,
  onCompleteChange,
}: {
  contractId?: number;
  participationId?: number;
  onCompleteChange?: (complete: boolean) => void;
}) {
  const isParticipation = participationId != null;
  const { data: contractOptions, isLoading: contractOptsLoading } = useBuyerContractFieldMapOptions(
    !isParticipation ? contractId ?? null : null
  );
  const { data: partOptions, isLoading: partOptsLoading } = useBuyerParticipationFieldMapOptions(
    isParticipation ? participationId : null
  );
  const { data: contractEntries, isLoading: contractEntriesLoading } = useBuyerContractFieldMap(
    !isParticipation ? contractId ?? null : null
  );
  const { data: partEntries, isLoading: partEntriesLoading } = useBuyerParticipationFieldMap(
    isParticipation ? participationId : null
  );

  const options = isParticipation ? partOptions : contractOptions;
  const entries = isParticipation ? partEntries : contractEntries;
  const loading = isParticipation
    ? partOptsLoading || partEntriesLoading
    : contractOptsLoading || contractEntriesLoading;

  const addContract = useAddBuyerContractFieldMap();
  const addParticipation = useAddBuyerParticipationFieldMap();
  const delContract = useDeleteBuyerContractFieldMap();
  const delParticipation = useDeleteBuyerParticipationFieldMap();
  const createField = useCreateField();

  const [rowTargets, setRowTargets] = useState<Record<string, string>>({});
  const [createForKey, setCreateForKey] = useState<string | null>(null);

  const mappedKeys = useMemo(() => mappedSourceKeys(entries), [entries]);

  const unmapped = useMemo(() => {
    const available = options?.available_fields ?? [];
    return available.filter((af) => !mappedKeys.has(af.key));
  }, [options, mappedKeys]);

  const suggestions = useMemo(
    () => buildPayloadSuggestions(unmapped.map((a) => a.label), options?.buyer_fields ?? []),
    [unmapped, options?.buyer_fields]
  );

  useEffect(() => {
    setRowTargets((prev) => {
      let changed = false;
      const next = { ...prev };
      for (const af of unmapped) {
        const sug = suggestions[af.label];
        if (!(af.key in prev) && sug) {
          next[af.key] = sug.startsWith("custom_") ? `cf:${sug.slice(7)}` : sug;
          changed = true;
        }
      }
      return changed ? next : prev;
    });
  }, [suggestions, unmapped]);

  useEffect(() => {
    setRowTargets((prev) => {
      const unmappedKeySet = new Set(unmapped.map((a) => a.key));
      const next: Record<string, string> = {};
      let changed = false;
      for (const [k, v] of Object.entries(prev)) {
        if (unmappedKeySet.has(k)) {
          next[k] = v;
        } else {
          changed = true;
        }
      }
      return changed ? next : prev;
    });
  }, [unmapped]);

  const complete = fieldMappingComplete(options, entries);

  useEffect(() => {
    onCompleteChange?.(complete);
  }, [complete, onCompleteChange]);

  function rowTarget(key: string) {
    return rowTargets[key] ?? suggestions[unmapped.find((a) => a.key === key)?.label ?? ""] ?? "first_name";
  }

  function clearRowTarget(key: string) {
    setRowTargets((prev) => {
      if (!(key in prev)) return prev;
      const next = { ...prev };
      delete next[key];
      return next;
    });
  }

  function addMapping(af: ContractFieldMapOptions["available_fields"][number], dstVal: string) {
    const body = { ...srcBodyFromAvailable(af), ...fieldBody("dst", dstVal) };
    const onSuccess = () => clearRowTarget(af.key);
    const onError = (e: unknown) => toast.error(errorMessage(e));
    if (isParticipation && participationId) {
      addParticipation.mutate({ participationId, body }, { onSuccess, onError });
    } else if (contractId) {
      addContract.mutate({ contractId, body }, { onSuccess, onError });
    }
  }

  function onFieldCreated(field: import("@/types").CustomField) {
    const key = createForKey;
    const val = `cf:${field.id}`;
    if (key) {
      setRowTargets((t) => ({ ...t, [key]: val }));
      const af = unmapped.find((a) => a.key === key);
      if (af) addMapping(af, val);
    }
    setCreateForKey(null);
    return field;
  }

  if (loading) return <Spinner className="h-5 w-5" />;

  if ((options?.available_fields ?? []).length === 0) {
    return <p className="text-sm text-gray-400">Publisher has not defined available fields for this contract.</p>;
  }

  return (
    <div className="space-y-4">
      {!complete && (
        <p className="rounded-md border border-warning-border bg-warning-bg px-3 py-2 text-sm text-warning-fg">
          Map every publisher available field to a field on your account before accepting.
        </p>
      )}

      <div className="space-y-1">
        {(entries ?? []).length === 0 && <p className="text-sm text-gray-400">No mappings yet.</p>}
        {(entries ?? []).map((e) => (
          <div
            key={e.id}
            className="flex items-center justify-between rounded-md border border-gray-100 px-3 py-2 text-sm"
          >
            <span>{options ? renderEntryLabel(e, options) : "—"}</span>
            <IconButton
              variant="danger"
              onClick={() => {
                const onError = (err: unknown) => toast.error(errorMessage(err));
                if (isParticipation && participationId && e.id) {
                  delParticipation.mutate({ participationId, mapId: e.id }, { onError });
                } else if (contractId && e.id) {
                  delContract.mutate({ contractId, mapId: e.id }, { onError });
                }
              }}
            >
              <Trash2 className="h-4 w-4" />
            </IconButton>
          </div>
        ))}
      </div>

      {unmapped.length > 0 && (
        <div className="space-y-3">
          <div>
            <Label>Unmapped fields</Label>
            <p className="text-xs text-gray-500">Map publisher fields to your built-in or custom fields.</p>
          </div>
          {unmapped.map((af) => (
            <div key={`${af.field_type}:${af.key}`} className="rounded-md border border-gray-100 p-3">
              <div className="mb-2 font-medium text-gray-800">{availableFieldLabel(af)}</div>
              <div className="flex flex-wrap items-end gap-2">
                <div className="min-w-[11rem] flex-1">
                  <BuiltinCustomFieldSelect
                    value={rowTarget(af.key)}
                    onChange={(v) => setRowTargets((t) => ({ ...t, [af.key]: v }))}
                    customFields={options?.buyer_fields ?? []}
                    builtins={MAP_BUILTIN_FIELDS}
                    label="Your field"
                    onAddCustomField={() => setCreateForKey(af.key)}
                  />
                </div>
                <Button
                  variant="secondary"
                  disabled={addContract.isPending || addParticipation.isPending}
                  onClick={() => addMapping(af, rowTarget(af.key))}
                >
                  Map
                </Button>
              </div>
            </div>
          ))}
        </div>
      )}

      <CreateCustomFieldDrawer
        open={createForKey !== null}
        onClose={() => setCreateForKey(null)}
        defaultName={createForKey ? unmapped.find((a) => a.key === createForKey)?.label ?? "" : ""}
        defaultFieldKey={
          createForKey ? slugFieldKey(unmapped.find((a) => a.key === createForKey)?.label ?? "") : ""
        }
        subtitle="Buyer custom field"
        isPending={createField.isPending}
        onSubmit={(body) => createField.mutateAsync(body).then(onFieldCreated)}
      />
    </div>
  );
}
