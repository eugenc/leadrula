import { Plus, Trash2 } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Input, Label, Select } from "@/components/ui/input";
import { IconButton } from "@/components/layout/IconButton";
import { Table, THead, TH, TBody, TR, TD } from "@/components/ui/table";
import { useCustomFields } from "@/features/leads/hooks";
import { SUNBASE_OUTBOUND_BUILTINS } from "@/features/integrations/sunbaseConstants";
import type { GhlCustomField } from "@/features/integrations/hooks";
import type { OutboundFieldMapEntry } from "@/types";

function applySourceTypeChange(
  row: OutboundFieldMapEntry,
  source_type: OutboundFieldMapEntry["source_type"]
): OutboundFieldMapEntry {
  const base = {
    dest_key: row.dest_key,
    source_type,
    ghl_custom_field_id: row.ghl_custom_field_id,
    ghl_field_model: row.ghl_field_model,
  };
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

function ghlFieldSelectValue(e: OutboundFieldMapEntry, fields: GhlCustomField[]): string {
  if (e.ghl_custom_field_id) return e.ghl_custom_field_id;
  if (e.dest_key) {
    const match = fields.find((f) => f.field_key === e.dest_key);
    if (match) return match.id;
  }
  return "";
}

export function GhlCustomFieldMapSection({
  entries,
  onChange,
  ghlCustomFields,
  ghlCustomFieldsLoading = false,
  webhookMode = false,
}: {
  entries: OutboundFieldMapEntry[];
  onChange: (entries: OutboundFieldMapEntry[]) => void;
  ghlCustomFields: GhlCustomField[];
  ghlCustomFieldsLoading?: boolean;
  webhookMode?: boolean;
}) {
  const { data: customFields } = useCustomFields();
  const activeCustomFields = (customFields ?? []).filter((f) => f.is_active !== false);

  const contactFields = ghlCustomFields.filter((f) => f.model === "contact");
  const opportunityFields = ghlCustomFields.filter((f) => f.model === "opportunity");
  const canUseDropdown = !webhookMode && ghlCustomFields.length > 0;

  function addRow() {
    onChange([
      ...entries,
      {
        dest_key: "",
        source_type: "builtin",
        builtin_field: "last_name",
        ghl_field_model: webhookMode ? "contact" : undefined,
      },
    ]);
  }

  function removeRow(idx: number) {
    onChange(entries.filter((_, i) => i !== idx));
  }

  function updateRow(idx: number, patch: Partial<OutboundFieldMapEntry>) {
    const next = [...entries];
    next[idx] = { ...next[idx], ...patch };
    onChange(next);
  }

  function onGhlFieldSelect(idx: number, value: string) {
    const field = ghlCustomFields.find((f) => f.id === value);
    if (field) {
      updateRow(idx, {
        dest_key: field.field_key,
        ghl_custom_field_id: field.id,
        ghl_field_model: field.model,
      });
      return;
    }
    updateRow(idx, {
      dest_key: "",
      ghl_custom_field_id: undefined,
      ghl_field_model: undefined,
    });
  }

  return (
    <div className="space-y-3 rounded-lg border border-gray-100 p-3">
      <div className="flex items-center justify-between">
        <Label>GHL custom field mapping</Label>
        <Button size="sm" variant="secondary" onClick={addRow}>
          <Plus className="h-3.5 w-3.5" /> Add field
        </Button>
      </div>
      <p className="text-xs text-gray-400">
        Map Leadrula fields to GHL custom fields.
        {!webhookMode && ghlCustomFields.length === 0 && !ghlCustomFieldsLoading && (
          <> Test connection to load GHL fields.</>
        )}
      </p>
      {entries.length === 0 ? (
        <p className="text-sm text-gray-400">No custom fields mapped.</p>
      ) : (
        <Table>
          <THead>
            <tr>
              <TH>GHL field</TH>
              {!webhookMode ? null : <TH>Target</TH>}
              <TH>Source</TH>
              <TH>Value</TH>
              <TH className="min-w-0 w-12" />
            </tr>
          </THead>
          <TBody>
            {entries.map((e, idx) => (
              <TR key={idx}>
                <TD>
                  {webhookMode ? (
                    <Input
                      value={e.dest_key}
                      onChange={(ev) => updateRow(idx, { dest_key: ev.target.value })}
                      placeholder="contact.field_key"
                      className="!h-8 !text-sm font-mono"
                    />
                  ) : ghlCustomFieldsLoading ? (
                    <Select className="!h-8 !text-sm" disabled value="">
                      <option value="">Loading GHL fields…</option>
                    </Select>
                  ) : canUseDropdown ? (
                    <Select
                      className="!h-8 w-full !text-sm"
                      value={ghlFieldSelectValue(e, ghlCustomFields)}
                      onChange={(ev) => onGhlFieldSelect(idx, ev.target.value)}
                    >
                      <option value="">Select GHL field</option>
                      {contactFields.length > 0 && (
                        <optgroup label="Contact fields">
                          {contactFields.map((f) => (
                            <option key={f.id} value={f.id}>
                              {f.name}
                            </option>
                          ))}
                        </optgroup>
                      )}
                      {opportunityFields.length > 0 && (
                        <optgroup label="Opportunity fields">
                          {opportunityFields.map((f) => (
                            <option key={f.id} value={f.id}>
                              {f.name}
                            </option>
                          ))}
                        </optgroup>
                      )}
                    </Select>
                  ) : (
                    <Input
                      value={e.dest_key}
                      onChange={(ev) =>
                        updateRow(idx, {
                          dest_key: ev.target.value,
                          ghl_custom_field_id: undefined,
                        })
                      }
                      placeholder="contact.field_key"
                      className="!h-8 !text-sm font-mono"
                    />
                  )}
                </TD>
                {webhookMode && (
                  <TD>
                    <Select
                      className="!h-8 !text-sm"
                      value={e.ghl_field_model ?? "contact"}
                      onChange={(ev) =>
                        updateRow(idx, {
                          ghl_field_model: ev.target.value as "contact" | "opportunity",
                        })
                      }
                    >
                      <option value="contact">Contact</option>
                      <option value="opportunity">Opportunity</option>
                    </Select>
                  </TD>
                )}
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
