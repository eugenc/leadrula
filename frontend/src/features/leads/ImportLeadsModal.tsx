import { useMemo, useState } from "react";
import Papa from "papaparse";
import { Dialog } from "@/components/ui/dialog";
import { Button } from "@/components/ui/button";
import { Label, FilterSelect } from "@/components/ui/input";
import { useAuthStore } from "@/store/authStore";
import { toast } from "@/store/toastStore";
import { apiError } from "@/lib/api";
import { useImportLeads, usePipelines, useStages, useCustomFields } from "./hooks";
import { buildInitialMapping, mappingTargetsWithCustom } from "./csvMapping";

type Step = "upload" | "map" | "destination" | "preview" | "done";

interface Props {
  open: boolean;
  onClose: () => void;
}

export function ImportLeadsModal({ open, onClose }: Props) {
  const isPublisher = useAuthStore((s) => s.user?.account_type === "publisher");
  const importLeads = useImportLeads();
  const { data: pipelines } = usePipelines();
  const { data: customFields } = useCustomFields();

  const [step, setStep] = useState<Step>("upload");
  const [headers, setHeaders] = useState<string[]>([]);
  const [rows, setRows] = useState<Record<string, string>[]>([]);
  const [mapping, setMapping] = useState<Record<string, string>>({});
  const [destination, setDestination] = useState<"pipeline" | "intake">("pipeline");
  const [pipelineId, setPipelineId] = useState(0);
  const [stageId, setStageId] = useState(0);
  const [result, setResult] = useState<{ created: number; skipped: number; errors: { row: number; message: string }[] } | null>(null);

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
    setResult(null);
  }

  function handleClose() {
    reset();
    onClose();
  }

  function onFile(file: File | null) {
    if (!file) return;
    if (!file.name.toLowerCase().endsWith(".csv")) {
      toast.error("Please upload a .csv file");
      return;
    }
    Papa.parse<Record<string, string>>(file, {
      header: true,
      skipEmptyLines: true,
      complete: (res) => {
        const cols = res.meta.fields ?? [];
        if (!cols.length || !res.data.length) {
          toast.error("CSV has no data");
          return;
        }
        setHeaders(cols);
        setRows(res.data);
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
    const mappingArr = headers
      .filter((h) => mapping[h] && mapping[h] !== "skip")
      .map((h) => ({ csv_column: h, target: mapping[h]! }));

    try {
      const res = await importLeads.mutateAsync({
        destination,
        pipeline_id: pipelineId || undefined,
        stage_id: stageId || undefined,
        mapping: mappingArr,
        rows,
      });
      setResult(res);
      setStep("done");
      if (res.created > 0) toast.success(`Imported ${res.created} lead${res.created === 1 ? "" : "s"}`);
    } catch (err) {
      toast.error(apiError(err).message);
    }
  }

  const previewRows = rows.slice(0, 5);

  return (
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
                ? `${rows.length} rows ready to import.`
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
            <Button onClick={() => setStep("preview")}>Next</Button>
          </>
        ) : step === "preview" ? (
          <>
            <Button variant="secondary" onClick={() => setStep("destination")}>
              Back
            </Button>
            <Button disabled={importLeads.isPending} onClick={runImport}>
              Import {rows.length} leads
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
                onChange={(e) => setMapping((m) => ({ ...m, [h]: e.target.value }))}
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
                Intake queue (review before routing)
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
        </div>
      )}

      {step === "preview" && (
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
  );
}
