import { useState } from "react";
import { Link } from "react-router-dom";
import { Plus, Trash2 } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Input, Label, Select } from "@/components/ui/input";
import { IconButton } from "@/components/layout/IconButton";
import { Table, THead, TH, TBody, TR, TD } from "@/components/ui/table";
import { Spinner } from "@/components/ui/misc";
import { useCustomFields } from "@/features/leads/hooks";
import { useCreateField } from "@/features/admin/hooks";
import { CreateCustomFieldDrawer } from "@/features/admin/CreateCustomFieldDrawer";
import { ADD_CUSTOM_FIELD, slugFieldKey } from "@/features/admin/customFieldConstants";
import { SUNBASE_OUTBOUND_BUILTINS } from "@/features/integrations/sunbaseConstants";
import { errorMessage } from "@/lib/api";
import { useAuthStore } from "@/store/authStore";
import { toast } from "@/store/toastStore";
import type { OutboundFieldMapEntry } from "@/types";

function applySourceTypeChange(
  row: OutboundFieldMapEntry,
  source_type: OutboundFieldMapEntry["source_type"]
): OutboundFieldMapEntry {
  const base = { dest_key: row.dest_key, source_type };
  if (source_type === "builtin") {
    return { ...base, builtin_field: row.builtin_field ?? "last_name" };
  }
  if (source_type === "custom") {
    return row.custom_field_id != null && row.custom_field_id > 0
      ? { ...base, custom_field_id: row.custom_field_id }
      : base;
  }
  return { ...base, static_value: row.static_value ?? "" };
}

export function SunbaseFieldMapSection({
  entries,
  onChange,
}: {
  entries: OutboundFieldMapEntry[];
  onChange: (entries: OutboundFieldMapEntry[]) => void;
}) {
  const { data: customFields, isLoading, isError, error } = useCustomFields();
  const createField = useCreateField();
  const accountType = useAuthStore((s) => s.user?.account_type);
  const fieldsPath = accountType === "publisher" ? "/p/fields" : "/b/fields";

  const [createFieldOpen, setCreateFieldOpen] = useState(false);
  const [createForRowIdx, setCreateForRowIdx] = useState<number | null>(null);

  const activeCustomFields = (customFields ?? []).filter((f) => f.is_active !== false);

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

  function openCreateField(idx: number) {
    setCreateForRowIdx(idx);
    setCreateFieldOpen(true);
  }

  function onFieldCreated(field: import("@/types").CustomField) {
    if (createForRowIdx != null) {
      updateRow(createForRowIdx, { source_type: "custom", custom_field_id: field.id });
    }
    setCreateFieldOpen(false);
    setCreateForRowIdx(null);
    return field;
  }

  const createRow = createForRowIdx != null ? entries[createForRowIdx] : undefined;

  return (
    <div className="space-y-3 rounded-lg border border-gray-100 p-3">
      <div className="flex items-center justify-between">
        <Label>Outbound field mapping</Label>
        <Button size="sm" variant="secondary" onClick={addRow}>
          <Plus className="h-3.5 w-3.5" /> Add param
        </Button>
      </div>
      <p className="text-xs text-gray-400">Map Leadrula lead fields to SunBase query parameters.</p>
      {entries.length === 0 ? (
        <p className="text-sm text-gray-400">No parameters mapped yet.</p>
      ) : (
        <Table>
          <THead>
            <tr>
              <TH>SunBase param</TH>
              <TH>Source</TH>
              <TH>Value</TH>
              <TH className="min-w-0 w-12" />
            </tr>
          </THead>
          <TBody>
            {entries.map((e, idx) => (
              <TR key={idx}>
                <TD>
                  <Input
                    value={e.dest_key}
                    onChange={(ev) => updateRow(idx, { dest_key: ev.target.value })}
                    placeholder="last_name"
                    className="!h-8 !text-sm font-mono"
                  />
                </TD>
                <TD>
                  <Select
                    className="!h-8 !text-sm"
                    value={e.source_type}
                    onChange={(ev) => {
                      const source_type = ev.target.value as OutboundFieldMapEntry["source_type"];
                      const next = [...entries];
                      next[idx] = applySourceTypeChange(e, source_type);
                      onChange(next);
                    }}
                  >
                    <option value="builtin">Lead field</option>
                    <option value="custom">Custom field</option>
                    <option value="static">Static value</option>
                  </Select>
                </TD>
                <TD>
                  {e.source_type === "builtin" && (
                    <Select
                      className="!h-8 w-full !text-sm"
                      value={e.builtin_field ?? "last_name"}
                      onChange={(ev) => updateRow(idx, { builtin_field: ev.target.value })}
                    >
                      {SUNBASE_OUTBOUND_BUILTINS.map((b) => (
                        <option key={b} value={b}>
                          {b}
                        </option>
                      ))}
                    </Select>
                  )}
                  {e.source_type === "custom" && (
                    <div className="space-y-1">
                      {isLoading ? (
                        <div className="flex h-8 items-center gap-2 text-xs text-gray-400">
                          <Spinner className="h-3.5 w-3.5" />
                          Loading custom fields…
                        </div>
                      ) : isError ? (
                        <p className="text-xs text-red-600">{errorMessage(error)}</p>
                      ) : (
                        <>
                          <Select
                            className="!h-8 w-full !text-sm"
                            value={String(e.custom_field_id ?? "")}
                            onChange={(ev) => {
                              const v = ev.target.value;
                              if (v === ADD_CUSTOM_FIELD) {
                                openCreateField(idx);
                                return;
                              }
                              if (!v) {
                                updateRow(idx, { custom_field_id: undefined });
                                return;
                              }
                              updateRow(idx, { custom_field_id: Number(v) });
                            }}
                          >
                            <option value="">Select field</option>
                            {activeCustomFields.map((f) => (
                              <option key={f.id} value={f.id}>
                                {f.name}
                              </option>
                            ))}
                            <option value={ADD_CUSTOM_FIELD}>+ Add custom field…</option>
                          </Select>
                          {activeCustomFields.length === 0 && (
                            <p className="text-xs text-gray-400">
                              No custom fields on this account yet.{" "}
                              <Link to={fieldsPath} className="text-indigo-600 hover:underline">
                                Manage custom fields
                              </Link>{" "}
                              or use &ldquo;+ Add custom field&rdquo; above.
                            </p>
                          )}
                        </>
                      )}
                    </div>
                  )}
                  {e.source_type === "static" && (
                    <Input
                      value={e.static_value ?? ""}
                      onChange={(ev) => updateRow(idx, { static_value: ev.target.value })}
                      placeholder="schema_name"
                      className="!h-8 !text-sm"
                    />
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
      <CreateCustomFieldDrawer
        open={createFieldOpen}
        onClose={() => {
          setCreateFieldOpen(false);
          setCreateForRowIdx(null);
        }}
        defaultName={createRow?.dest_key ? createRow.dest_key.replace(/_/g, " ") : ""}
        defaultFieldKey={createRow?.dest_key ? slugFieldKey(createRow.dest_key) : ""}
        subtitle={createRow?.dest_key ? `SunBase param: ${createRow.dest_key}` : undefined}
        isPending={createField.isPending}
        onSubmit={(body) =>
          createField.mutateAsync(body).then(onFieldCreated).catch((err) => {
            toast.error(errorMessage(err));
            throw err;
          })
        }
      />
    </div>
  );
}
