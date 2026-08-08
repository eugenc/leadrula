import { useMemo, useState } from "react";
import { useQueryClient } from "@tanstack/react-query";
import { Dialog } from "@/components/ui/dialog";
import { Button } from "@/components/ui/button";
import { FilterSelect } from "@/components/ui/input";
import { Spinner } from "@/components/ui/misc";
import { toast } from "@/store/toastStore";
import { ns, post, errorMessage } from "@/lib/api";
import { INTEGRATION_CATEGORY } from "@/features/integrations/constants";
import { useIntegrationConnections } from "@/features/integrations/hooks";
import { useCrmCustomFields, type ImportFromCrmResult } from "./hooks";

interface Props {
  open: boolean;
  onClose: () => void;
}

export function ImportFromCrmModal({ open, onClose }: Props) {
  const qc = useQueryClient();
  const { data: connections, isLoading: connectionsLoading } = useIntegrationConnections();
  const [connectionId, setConnectionId] = useState<number | null>(null);
  const [selected, setSelected] = useState<Set<string>>(new Set());
  const [importing, setImporting] = useState(false);
  const [result, setResult] = useState<ImportFromCrmResult | null>(null);

  const crmConnections = useMemo(
    () =>
      (connections ?? []).filter(
        (c) => c.status === "active" && INTEGRATION_CATEGORY[c.provider_slug] === "crm"
      ),
    [connections]
  );

  const { data: crmFieldsData, isLoading: fieldsLoading, error: fieldsError } = useCrmCustomFields(connectionId);

  const importableFields = useMemo(
    () => (crmFieldsData?.custom_fields ?? []).filter((f) => !f.already_imported),
    [crmFieldsData]
  );

  function reset() {
    setConnectionId(null);
    setSelected(new Set());
    setImporting(false);
    setResult(null);
  }

  function handleClose() {
    if (importing) return;
    reset();
    onClose();
  }

  function toggleField(id: string) {
    setSelected((prev) => {
      const next = new Set(prev);
      if (next.has(id)) next.delete(id);
      else next.add(id);
      return next;
    });
  }

  function toggleAll() {
    if (selected.size === importableFields.length) {
      setSelected(new Set());
    } else {
      setSelected(new Set(importableFields.map((f) => f.id)));
    }
  }

  async function runImport() {
    if (!connectionId || selected.size === 0) return;
    const fields = (crmFieldsData?.custom_fields ?? [])
      .filter((f) => selected.has(f.id))
      .map((f) => ({
        crm_field_id: f.id,
        crm_field_key: f.field_key,
        name: f.name,
        data_type: f.data_type,
        object: f.object,
        options: f.options ?? [],
        lead_type: f.lead_type,
        inbound_source_key: f.inbound_source_key,
      }));

    setImporting(true);
    try {
      const res = await post<ImportFromCrmResult>(`${ns()}/custom-fields/import-from-crm`, {
        connection_id: connectionId,
        fields,
      });
      qc.invalidateQueries({ queryKey: ["custom-fields"] });
      if (connectionId) {
        qc.invalidateQueries({ queryKey: ["crm-custom-fields", connectionId] });
      }
      setResult(res);
      if (res.created > 0) {
        toast.success(`Imported ${res.created} field${res.created === 1 ? "" : "s"} from CRM`);
      } else if (res.skipped > 0 && res.created === 0) {
        toast.info("No new fields imported");
      }
    } catch (err) {
      toast.error(errorMessage(err));
    } finally {
      setImporting(false);
    }
  }

  const done = result != null;

  return (
    <Dialog
      open={open}
      onClose={handleClose}
      title="Import from CRM"
      subtitle={
        done
          ? "Import complete."
          : "Select a CRM connection and choose custom fields to create in Leadrula."
      }
      className="max-w-2xl"
      footer={
        done ? (
          <Button onClick={handleClose}>Done</Button>
        ) : (
          <>
            <Button variant="secondary" disabled={importing} onClick={handleClose}>
              Cancel
            </Button>
            <Button
              disabled={importing || !connectionId || selected.size === 0}
              onClick={runImport}
            >
              {importing ? "Importing…" : `Import ${selected.size} field${selected.size === 1 ? "" : "s"}`}
            </Button>
          </>
        )
      }
    >
      {done && result ? (
        <div className="space-y-2 text-sm text-gray-700">
          <p>
            <strong>{result.created}</strong> created
            {result.skipped > 0 && (
              <>
                , <strong>{result.skipped}</strong> skipped
              </>
            )}
          </p>
          {result.errors.length > 0 && (
            <ul className="max-h-32 overflow-y-auto rounded border border-gray-100 p-2 text-xs text-danger">
              {result.errors.slice(0, 20).map((e) => (
                <li key={e.row}>
                  Row {e.row}: {e.message}
                </li>
              ))}
            </ul>
          )}
        </div>
      ) : (
        <div className="space-y-4">
          <div>
            <label className="mb-1 block text-sm font-medium text-gray-700">CRM connection</label>
            {connectionsLoading ? (
              <Spinner className="h-5 w-5" />
            ) : (
              <FilterSelect
                value={connectionId ?? ""}
                onChange={(e) => {
                  const id = e.target.value ? Number(e.target.value) : null;
                  setConnectionId(id);
                  setSelected(new Set());
                  setResult(null);
                }}
                className="w-full"
              >
                <option value="">Select connection…</option>
                {crmConnections.map((c) => (
                  <option key={c.id} value={c.id}>
                    {c.name} ({c.provider_slug})
                  </option>
                ))}
              </FilterSelect>
            )}
            {!connectionsLoading && crmConnections.length === 0 && (
              <p className="mt-1 text-xs text-gray-500">No active CRM connections found.</p>
            )}
          </div>

          {connectionId != null && (
            <div>
              {fieldsLoading ? (
                <div className="flex justify-center py-8">
                  <Spinner className="h-6 w-6" />
                </div>
              ) : fieldsError ? (
                <p className="text-sm text-danger">{errorMessage(fieldsError)}</p>
              ) : importableFields.length === 0 ? (
                <p className="text-sm text-gray-500">
                  {(crmFieldsData?.custom_fields ?? []).length === 0
                    ? "No custom fields found in this CRM."
                    : "All CRM custom fields are already imported."}
                </p>
              ) : (
                <>
                  <div className="mb-2 flex items-center justify-between">
                    <span className="text-sm text-gray-600">
                      {importableFields.length} field{importableFields.length === 1 ? "" : "s"} available
                    </span>
                    <button
                      type="button"
                      className="text-sm text-jade-600 hover:underline"
                      onClick={toggleAll}
                    >
                      {selected.size === importableFields.length ? "Deselect all" : "Select all"}
                    </button>
                  </div>
                  <div className="max-h-72 overflow-y-auto rounded border border-gray-100">
                    <table className="w-full text-left text-sm">
                      <thead className="sticky top-0 bg-gray-50">
                        <tr>
                          <th className="w-10 px-2 py-2" />
                          <th className="px-2 py-2 font-medium text-gray-500">Name</th>
                          <th className="px-2 py-2 font-medium text-gray-500">Key</th>
                          <th className="px-2 py-2 font-medium text-gray-500">Type</th>
                        </tr>
                      </thead>
                      <tbody>
                        {importableFields.map((f) => (
                          <tr key={f.id} className="border-t border-gray-50">
                            <td className="px-2 py-2">
                              <input
                                type="checkbox"
                                checked={selected.has(f.id)}
                                onChange={() => toggleField(f.id)}
                              />
                            </td>
                            <td className="px-2 py-2 text-gray-800">{f.name}</td>
                            <td className="px-2 py-2 font-mono text-xs text-gray-500">{f.field_key}</td>
                            <td className="px-2 py-2 text-gray-600">{f.lead_type}</td>
                          </tr>
                        ))}
                      </tbody>
                    </table>
                  </div>
                </>
              )}
            </div>
          )}
        </div>
      )}
    </Dialog>
  );
}
