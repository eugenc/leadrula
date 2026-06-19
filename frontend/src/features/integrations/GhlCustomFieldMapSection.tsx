import { useState } from "react";
import { Plus, Trash2 } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Input, Label, Select } from "@/components/ui/input";
import { IconButton } from "@/components/layout/IconButton";
import { Table, THead, TH, TBody, TR, TD } from "@/components/ui/table";
import { useCustomFields } from "@/features/leads/hooks";
import { SUNBASE_OUTBOUND_BUILTINS } from "@/features/integrations/sunbaseConstants";
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

export function GhlCustomFieldMapSection({
  entries,
  onChange,
}: {
  entries: OutboundFieldMapEntry[];
  onChange: (entries: OutboundFieldMapEntry[]) => void;
}) {
  const { data: customFields } = useCustomFields();
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

  return (
    <div className="space-y-3 rounded-lg border border-gray-100 p-3">
      <div className="flex items-center justify-between">
        <Label>GHL custom field mapping</Label>
        <Button size="sm" variant="secondary" onClick={addRow}>
          <Plus className="h-3.5 w-3.5" /> Add field
        </Button>
      </div>
      <p className="text-xs text-gray-400">Map Leadrula fields to GHL custom field keys.</p>
      {entries.length === 0 ? (
        <p className="text-sm text-gray-400">No custom fields mapped.</p>
      ) : (
        <Table>
          <THead>
            <tr>
              <TH>GHL field key</TH>
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
                    placeholder="customField_xyz"
                    className="!h-8 !text-sm font-mono"
                  />
                </TD>
                <TD>
                  <Select
                    className="!h-8 !text-sm"
                    value={e.source_type}
                    onChange={(ev) => {
                      const source_type = ev.target.value as OutboundFieldMapEntry["source_type"];
                      updateRow(idx, applySourceTypeChange(e, source_type));
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
                      {SUNBASE_OUTBOUND_BUILTINS.filter((b) => b !== "schema_name").map((b) => (
                        <option key={b} value={b}>
                          {b}
                        </option>
                      ))}
                    </Select>
                  )}
                  {e.source_type === "custom" && (
                    <Select
                      className="!h-8 w-full !text-sm"
                      value={String(e.custom_field_id ?? "")}
                      onChange={(ev) => {
                        const v = ev.target.value;
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
                    </Select>
                  )}
                  {e.source_type === "static" && (
                    <Input
                      value={e.static_value ?? ""}
                      onChange={(ev) => updateRow(idx, { static_value: ev.target.value })}
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
    </div>
  );
}
