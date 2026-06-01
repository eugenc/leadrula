import { useMemo, useState } from "react";
import { useQueryClient } from "@tanstack/react-query";
import Papa from "papaparse";
import { Dialog } from "@/components/ui/dialog";
import { Button } from "@/components/ui/button";
import { Label, FilterSelect } from "@/components/ui/input";
import { toast } from "@/store/toastStore";
import { ns, post, errorMessage } from "@/lib/api";
import { chunk } from "@/lib/chunk";
import {
  CUSTOM_FIELD_IMPORT_BATCH_SIZE,
  type ImportCustomFieldsResult,
} from "./hooks";
import {
  buildFieldInitialMapping,
  FIELD_MAPPING_TARGETS,
  missingRequiredMappings,
} from "./customFieldCsvMapping";

type Step = "upload" | "map" | "preview" | "done";

function sanitizeImportRows(rows: Record<string, unknown>[]): Record<string, string>[] {
  return rows
    .map((row) => {
      const out: Record<string, string> = {};
      for (const [key, val] of Object.entries(row)) {
        if (key === "__parsed_extra") continue;
        if (val == null) continue;
        if (typeof val === "string") out[key] = val;
        else if (typeof val === "number" || typeof val === "boolean") out[key] = String(val);
      }
      return out;
    })
    .filter((row) => Object.values(row).some((v) => v.trim()));
}

interface Props {
  open: boolean;
  onClose: () => void;
}

export function ImportCustomFieldsModal({ open, onClose }: Props) {
  const qc = useQueryClient();
  const [step, setStep] = useState<Step>("upload");
  const [headers, setHeaders] = useState<string[]>([]);
  const [rows, setRows] = useState<Record<string, string>[]>([]);
  const [mapping, setMapping] = useState<Record<string, string>>({});
  const [result, setResult] = useState<ImportCustomFieldsResult | null>(null);
  const [importing, setImporting] = useState(false);

  const missing = useMemo(() => missingRequiredMappings(mapping), [mapping]);

  function reset() {
    setStep("upload");
    setHeaders([]);
    setRows([]);
    setMapping({});
    setResult(null);
    setImporting(false);
  }

  function handleClose() {
    if (importing) return;
    reset();
    onClose();
  }

  function onFile(file: File | null) {
    if (!file) return;
    if (!file.name.toLowerCase().endsWith(".csv")) {
      toast.error("Please upload a .csv file");
      return;
    }
    Papa.parse<Record<string, unknown>>(file, {
      header: true,
      skipEmptyLines: true,
      complete: (res) => {
        const cols = res.meta.fields ?? [];
        const data = sanitizeImportRows(res.data);
        if (!cols.length || !data.length) {
          toast.error("CSV has no data");
          return;
        }
        setHeaders(cols);
        setRows(data);
        setMapping(buildFieldInitialMapping(cols));
        setStep("map");
      },
      error: () => toast.error("Could not parse CSV"),
    });
  }

  async function runImport() {
    const cleanRows = sanitizeImportRows(rows as Record<string, unknown>[]);
    if (!cleanRows.length) {
      toast.error("No rows to import — check your CSV has data");
      return;
    }
    if (missing.length) {
      toast.error(`Map required columns: ${missing.join(", ")}`);
      return;
    }

    const mappingArr = headers
      .filter((h) => mapping[h] && mapping[h] !== "skip")
      .map((h) => ({ csv_column: h, target: mapping[h]! }));

    const batches = chunk(cleanRows, CUSTOM_FIELD_IMPORT_BATCH_SIZE);
    const total = cleanRows.length;
    const totalLabel = total.toLocaleString();
    let progressToastId = toast.progress(`Importing 0 of ${totalLabel} fields…`);

    setImporting(true);

    let created = 0;
    let updated = 0;
    let skipped = 0;
    let processed = 0;
    const errors: { row: number; message: string }[] = [];

    try {
      for (let i = 0; i < batches.length; i++) {
        const res = await post<ImportCustomFieldsResult>(`${ns()}/custom-fields/import`, {
          mapping: mappingArr,
          rows: batches[i],
        });
        created += res.created;
        updated += res.updated;
        skipped += res.skipped;
        processed += batches[i].length;
        toast.update(progressToastId, `Importing ${processed.toLocaleString()} of ${totalLabel} fields…`);
        const rowOffset = i * CUSTOM_FIELD_IMPORT_BATCH_SIZE;
        for (const e of res.errors) {
          errors.push({ row: rowOffset + e.row, message: e.message });
        }
      }
      toast.dismiss(progressToastId);
      progressToastId = 0;
      qc.invalidateQueries({ queryKey: ["custom-fields"] });
      setResult({ created, updated, skipped, errors });
      setStep("done");
      if (created + updated > 0) {
        toast.success(
          `Imported ${created.toLocaleString()} new, updated ${updated.toLocaleString()} existing`
        );
      }
    } catch (err) {
      if (progressToastId) toast.dismiss(progressToastId);
      toast.error(errorMessage(err));
    } finally {
      setImporting(false);
    }
  }

  const previewRows = rows.slice(0, 5);

  return (
    <Dialog
      open={open}
      onClose={handleClose}
      title="Import Custom Fields"
      subtitle={
        step === "upload"
          ? "Upload a CSV file of custom field definitions."
          : step === "map"
            ? "Map columns to field properties."
            : step === "preview"
              ? `${rows.length.toLocaleString()} rows ready to import.`
              : "Import complete."
      }
      className="max-w-xl"
      footer={
        step === "upload" ? (
          <Button variant="secondary" onClick={handleClose}>
            Cancel
          </Button>
        ) : step === "map" ? (
          <>
            <Button variant="secondary" onClick={() => setStep("upload")}>
              Back
            </Button>
            <Button
              onClick={() => {
                if (missing.length) {
                  toast.error(`Map required columns: ${missing.join(", ")}`);
                  return;
                }
                setStep("preview");
              }}
            >
              Next
            </Button>
          </>
        ) : step === "preview" ? (
          <>
            <Button variant="secondary" disabled={importing} onClick={() => setStep("map")}>
              Back
            </Button>
            <Button disabled={importing} onClick={runImport}>
              {importing ? "Importing…" : `Import ${rows.length.toLocaleString()} fields`}
            </Button>
          </>
        ) : (
          <Button onClick={handleClose}>Done</Button>
        )
      }
    >
      {step === "upload" && (
        <div>
          <Label>CSV file</Label>
          <p className="mt-1 text-xs text-gray-500">
            Columns: name, field_key, type, options (dropdown), is_active
          </p>
          <input
            type="file"
            accept=".csv,text/csv"
            className="mt-2 block w-full text-sm text-gray-600"
            onChange={(e) => onFile(e.target.files?.[0] ?? null)}
          />
        </div>
      )}

      {step === "map" && (
        <div>
          {missing.length > 0 && (
            <p className="mb-2 text-xs text-danger">
              Required mappings missing: {missing.join(", ")}
            </p>
          )}
          <div className="max-h-64 space-y-2 overflow-y-auto">
            {headers.map((h) => (
              <div key={h} className="flex items-center gap-2">
                <span className="w-36 shrink-0 truncate text-sm text-gray-600" title={h}>
                  {h}
                </span>
                <FilterSelect
                  value={mapping[h] ?? "skip"}
                  onChange={(e) => setMapping((m) => ({ ...m, [h]: e.target.value }))}
                  className="flex-1"
                >
                  {FIELD_MAPPING_TARGETS.map((t) => (
                    <option key={t.value} value={t.value}>
                      {t.label}
                    </option>
                  ))}
                </FilterSelect>
              </div>
            ))}
          </div>
        </div>
      )}

      {step === "preview" && (
        <div className="overflow-x-auto">
          <table className="w-full text-left text-sm">
            <thead>
              <tr className="border-b border-gray-100">
                {headers.slice(0, 5).map((h) => (
                  <th key={h} className="px-2 py-1 font-medium text-gray-500">
                    {mapping[h] && mapping[h] !== "skip"
                      ? FIELD_MAPPING_TARGETS.find((t) => t.value === mapping[h])?.label ?? h
                      : h}
                  </th>
                ))}
              </tr>
            </thead>
            <tbody>
              {previewRows.map((row, i) => (
                <tr key={i} className="border-b border-gray-50">
                  {headers.slice(0, 5).map((h) => (
                    <td key={h} className="max-w-[120px] truncate px-2 py-1 text-gray-700">
                      {row[h] ?? "—"}
                    </td>
                  ))}
                </tr>
              ))}
            </tbody>
          </table>
          {rows.length > 5 && (
            <p className="mt-2 text-xs text-gray-400">Showing 5 of {rows.length} rows</p>
          )}
        </div>
      )}

      {step === "done" && result && (
        <div className="space-y-2 text-sm text-gray-700">
          <p>
            <strong>{result.created}</strong> created, <strong>{result.updated}</strong> updated
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
              {result.errors.length > 20 && <li>…and {result.errors.length - 20} more</li>}
            </ul>
          )}
        </div>
      )}
    </Dialog>
  );
}
