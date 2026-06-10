import { useMemo, useState, type KeyboardEvent } from "react";
import { useQueryClient } from "@tanstack/react-query";
import Papa from "papaparse";
import { X } from "lucide-react";
import { Dialog } from "@/components/ui/dialog";
import { Button } from "@/components/ui/button";
import { Input, Label, FilterSelect } from "@/components/ui/input";
import { Badge } from "@/components/ui/misc";
import { useAuthStore } from "@/store/authStore";
import { toast } from "@/store/toastStore";
import { ns, post, errorMessage } from "@/lib/api";
import { chunk } from "@/lib/chunk";
import {
  IMPORT_BATCH_SIZE,
  usePipelines,
  useStages,
  useCustomFields,
  useTagSuggestions,
  type ImportLeadsResult,
} from "./hooks";
import { buildInitialMapping, mappingTargetsWithCustom } from "./csvMapping";
import { ADD_CUSTOM_FIELD, slugFieldKey } from "@/features/admin/customFieldConstants";
import { CreateCustomFieldDrawer } from "@/features/admin/CreateCustomFieldDrawer";
import { useCreateField } from "@/features/admin/hooks";

type Step = "upload" | "map" | "destination" | "preview" | "done";

function normalizeTags(tags: string[]): string[] {
  const seen = new Set<string>();
  const out: string[] = [];
  for (const raw of tags) {
    const t = raw.trim();
    if (!t) continue;
    const key = t.toLowerCase();
    if (seen.has(key)) continue;
    seen.add(key);
    out.push(t);
  }
  return out;
}

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

export function ImportLeadsModal({ open, onClose }: Props) {
  const isPublisher = useAuthStore((s) => s.user?.account_type === "publisher");
  const qc = useQueryClient();
  const { data: pipelines } = usePipelines();
  const { data: customFields } = useCustomFields();
  const { data: tagSuggestions } = useTagSuggestions();

  const [step, setStep] = useState<Step>("upload");
  const [headers, setHeaders] = useState<string[]>([]);
  const [rows, setRows] = useState<Record<string, string>[]>([]);
  const [mapping, setMapping] = useState<Record<string, string>>({});
  const [destination, setDestination] = useState<"pipeline" | "intake">("pipeline");
  const [pipelineId, setPipelineId] = useState(0);
  const [stageId, setStageId] = useState(0);
  const [importTags, setImportTags] = useState<string[]>([]);
  const [importFilename, setImportFilename] = useState("");
  const [tagInput, setTagInput] = useState("");
  const [result, setResult] = useState<{ created: number; skipped: number; errors: { row: number; message: string }[] } | null>(null);
  const [importing, setImporting] = useState(false);
  const [createFieldHeader, setCreateFieldHeader] = useState<string | null>(null);

  const createField = useCreateField();

  const { data: stages } = useStages(pipelineId || undefined);
  const targets = useMemo(
    () => mappingTargetsWithCustom(customFields ?? []),
    [customFields]
  );

  function reset() {
    setStep("upload");
    setHeaders([]);
    setRows([]);
    setMapping({});
    setDestination("pipeline");
    setPipelineId(0);
    setStageId(0);
    setImportTags([]);
    setImportFilename("");
    setTagInput("");
    setResult(null);
    setImporting(false);
    setCreateFieldHeader(null);
  }

  function addImportTag(raw: string) {
    const parts = raw.split(",").map((s) => s.trim()).filter(Boolean);
    if (!parts.length) return;
    setImportTags((prev) => normalizeTags([...prev, ...parts]));
    setTagInput("");
  }

  function removeImportTag(tag: string) {
    setImportTags((prev) => prev.filter((t) => t !== tag));
  }

  function onTagKeyDown(e: KeyboardEvent<HTMLInputElement>) {
    if (e.key === "Enter" || e.key === ",") {
      e.preventDefault();
      addImportTag(tagInput);
    }
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
    setImportFilename(file.name);
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
        setMapping(buildInitialMapping(cols, customFields ?? []));
        setStep("map");
      },
      error: () => toast.error("Could not parse CSV"),
    });
  }

  async function runImport() {
    if (destination === "pipeline" && (!pipelineId || !stageId)) {
      toast.error("Select pipeline and stage");
      return;
    }
    if (tagInput.trim()) addImportTag(tagInput);

    const cleanRows = sanitizeImportRows(rows as Record<string, unknown>[]);
    if (!cleanRows.length) {
      toast.error("No rows to import — check your CSV has data");
      return;
    }

    const mappingArr = headers
      .filter((h) => mapping[h] && mapping[h] !== "skip")
      .map((h) => ({ csv_column: h, target: mapping[h]! }));

    const batches = chunk(cleanRows, IMPORT_BATCH_SIZE);
    const payload = {
      destination,
      pipeline_id: destination === "pipeline" ? Number(pipelineId) : undefined,
      stage_id: destination === "pipeline" ? Number(stageId) : undefined,
      default_tags: importTags.length ? importTags : undefined,
      import_filename: importFilename || undefined,
      mapping: mappingArr,
    };

    const total = cleanRows.length;
    const totalLabel = total.toLocaleString();
    let progressToastId = toast.progress(`Importing 0 of ${totalLabel} contacts…`);

    setImporting(true);

    let created = 0;
    let skipped = 0;
    let processed = 0;
    const errors: { row: number; message: string }[] = [];

    try {
      for (let i = 0; i < batches.length; i++) {
        const res = await post<ImportLeadsResult>(`${ns()}/leads/import`, {
          ...payload,
          rows: batches[i],
        });
        created += res.created;
        skipped += res.skipped;
        processed += batches[i].length;
        toast.update(progressToastId, `Importing ${processed.toLocaleString()} of ${totalLabel} contacts…`);
        const rowOffset = i * IMPORT_BATCH_SIZE;
        for (const e of res.errors) {
          errors.push({ row: rowOffset + e.row, message: e.message });
        }
      }
      toast.dismiss(progressToastId);
      progressToastId = 0;
      qc.invalidateQueries({ queryKey: ["leads"] });
      qc.invalidateQueries({ queryKey: ["lead-tags"] });
      setResult({ created, skipped, errors });
      setStep("done");
      if (created > 0) toast.success(`Imported ${created.toLocaleString()} lead${created === 1 ? "" : "s"}`);
    } catch (err) {
      if (progressToastId) toast.dismiss(progressToastId);
      toast.error(errorMessage(err));
    } finally {
      setImporting(false);
    }
  }

  const previewRows = rows.slice(0, 5);

  return (
    <>
    <Dialog
      open={open}
      onClose={handleClose}
      title="Import CSV"
      subtitle={
        step === "upload"
          ? "Upload a CSV file of leads."
          : step === "map"
            ? "Map columns to lead fields."
            : step === "destination"
              ? "Choose where imported leads go."
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
            <Button onClick={() => setStep("destination")}>Next</Button>
          </>
        ) : step === "destination" ? (
          <>
            <Button variant="secondary" onClick={() => setStep("map")}>
              Back
            </Button>
            <Button
              onClick={() => {
                if (destination === "pipeline" && (!pipelineId || !stageId)) {
                  toast.error("Select pipeline and stage");
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
            <Button variant="secondary" disabled={importing} onClick={() => setStep("destination")}>
              Back
            </Button>
            <Button disabled={importing} onClick={runImport}>
              {importing ? "Importing…" : `Import ${rows.length.toLocaleString()} leads`}
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
          <input
            type="file"
            accept=".csv,text/csv"
            className="mt-2 block w-full text-sm text-gray-600"
            onChange={(e) => onFile(e.target.files?.[0] ?? null)}
          />
        </div>
      )}

      {step === "map" && (
        <div className="max-h-64 space-y-2 overflow-y-auto">
          {headers.map((h) => (
            <div key={h} className="flex items-center gap-2">
              <span className="w-36 shrink-0 truncate text-sm text-gray-600" title={h}>
                {h}
              </span>
              <FilterSelect
                value={mapping[h] ?? "skip"}
                onChange={(e) => {
                  if (e.target.value === ADD_CUSTOM_FIELD) {
                    setCreateFieldHeader(h);
                    return;
                  }
                  setMapping((m) => ({ ...m, [h]: e.target.value }));
                }}
                className="flex-1"
              >
                {targets.map((t) => (
                  <option key={t.value} value={t.value}>
                    {t.label}
                  </option>
                ))}
              </FilterSelect>
            </div>
          ))}
        </div>
      )}

      {step === "destination" && (
        <div className="flex flex-col gap-4">
          <div className="flex flex-col gap-2">
            <label className="flex items-center gap-2 text-sm">
              <input
                type="radio"
                checked={destination === "pipeline"}
                onChange={() => setDestination("pipeline")}
              />
              Place in pipeline
            </label>
            {isPublisher && (
              <label className="flex items-center gap-2 text-sm">
                <input
                  type="radio"
                  checked={destination === "intake"}
                  onChange={() => setDestination("intake")}
                />
                Review Mapping (hold for manual routing)
              </label>
            )}
          </div>
          {destination === "pipeline" && (
            <div className="grid grid-cols-2 gap-3">
              <div>
                <Label>Pipeline</Label>
                <FilterSelect
                  value={pipelineId}
                  onChange={(e) => {
                    setPipelineId(Number(e.target.value));
                    setStageId(0);
                  }}
                  className="mt-1.5 w-full"
                >
                  <option value={0}>Select…</option>
                  {(pipelines ?? []).map((p) => (
                    <option key={p.id} value={p.id}>
                      {p.name}
                    </option>
                  ))}
                </FilterSelect>
              </div>
              <div>
                <Label>Stage</Label>
                <FilterSelect
                  value={stageId}
                  onChange={(e) => setStageId(Number(e.target.value))}
                  className="mt-1.5 w-full"
                  disabled={!pipelineId}
                >
                  <option value={0}>Select…</option>
                  {(stages ?? []).map((s) => (
                    <option key={s.id} value={s.id}>
                      {s.name}
                    </option>
                  ))}
                </FilterSelect>
              </div>
            </div>
          )}
          <div>
            <Label>Tags (optional)</Label>
            <p className="mt-0.5 text-xs text-gray-500">Applied to every imported lead.</p>
            <div className="mt-1.5 flex flex-wrap gap-1.5">
              {importTags.map((tag) => (
                <Badge key={tag} variant="default" className="gap-1 pr-1">
                  {tag}
                  <button
                    type="button"
                    onClick={() => removeImportTag(tag)}
                    className="rounded-full p-0.5 hover:bg-gray-200"
                    aria-label={`Remove tag ${tag}`}
                  >
                    <X className="h-3 w-3" />
                  </button>
                </Badge>
              ))}
            </div>
            <Input
              value={tagInput}
              onChange={(e) => setTagInput(e.target.value)}
              onKeyDown={onTagKeyDown}
              onBlur={() => tagInput.trim() && addImportTag(tagInput)}
              placeholder="Add tag…"
              list="import-tag-suggestions"
              className="mt-2"
            />
            <datalist id="import-tag-suggestions">
              {(tagSuggestions ?? [])
                .filter((s) => !importTags.some((t) => t.toLowerCase() === s.toLowerCase()))
                .map((s) => (
                  <option key={s} value={s} />
                ))}
            </datalist>
          </div>
        </div>
      )}

      {step === "preview" && (
        <div>
          {importTags.length > 0 && (
            <p className="mb-3 text-sm text-gray-600">
              Tags applied to all leads: {importTags.join(", ")}
            </p>
          )}
          <div className="overflow-x-auto">
          <table className="w-full text-left text-sm">
            <thead>
              <tr className="border-b border-gray-100">
                {headers.slice(0, 5).map((h) => (
                  <th key={h} className="px-2 py-1 font-medium text-gray-500">
                    {mapping[h] && mapping[h] !== "skip" ? targets.find((t) => t.value === mapping[h])?.label ?? h : h}
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
        </div>
      )}

      {step === "done" && result && (
        <div className="space-y-2 text-sm text-gray-700">
          <p>
            <strong>{result.created}</strong> leads imported
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
    <CreateCustomFieldDrawer
      open={createFieldHeader !== null}
      onClose={() => setCreateFieldHeader(null)}
      defaultName={createFieldHeader ?? ""}
      defaultFieldKey={createFieldHeader ? slugFieldKey(createFieldHeader) : ""}
      subtitle={createFieldHeader ? `CSV column: ${createFieldHeader}` : undefined}
      isPending={createField.isPending}
      onSubmit={(body) =>
        createField.mutateAsync(body).then((field) => {
          if (createFieldHeader) {
            setMapping((m) => ({ ...m, [createFieldHeader]: `custom_${field.id}` }));
          }
          return field;
        })
      }
    />
  </>
  );
}
