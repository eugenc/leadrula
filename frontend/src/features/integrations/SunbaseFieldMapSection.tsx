import { Plus, Trash2 } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Input, Label, Select } from "@/components/ui/input";
import { IconButton } from "@/components/layout/IconButton";
import { Table, THead, TH, TBody, TR, TD } from "@/components/ui/table";
import { useCustomFields } from "@/features/leads/hooks";
import { SUNBASE_OUTBOUND_BUILTINS } from "@/features/integrations/sunbaseConstants";
import type { OutboundFieldMapEntry } from "@/types";

export function SunbaseFieldMapSection({
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
                      const patch: Partial<OutboundFieldMapEntry> = { source_type };
                      if (source_type === "builtin") patch.builtin_field = "last_name";
                      if (source_type === "static") patch.static_value = "";
                      updateRow(idx, patch);
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
                    <Select
                      className="!h-8 w-full !text-sm"
                      value={e.custom_field_id ?? ""}
                      onChange={(ev) => updateRow(idx, { custom_field_id: Number(ev.target.value) })}
                    >
                      <option value="">Select field</option>
                      {customFields
                        .filter((f) => f.is_active !== false)
                        .map((f) => (
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
    </div>
  );
}
