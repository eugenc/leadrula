import { useMemo, useState } from "react";
import { useQueryClient } from "@tanstack/react-query";
import { Link } from "react-router-dom";
import { ChevronDown, ChevronRight } from "lucide-react";
import { Dialog } from "@/components/ui/dialog";
import { Button } from "@/components/ui/button";
import { Label, Select } from "@/components/ui/input";
import { Spinner } from "@/components/ui/misc";
import { toast } from "@/store/toastStore";
import { errorMessage } from "@/lib/api";
import { useIntegrationConnections, useCrmPipelines } from "@/features/integrations/hooks";
import { useImportPipelinesFromCrm } from "@/features/admin/hooks";
import {
  INTEGRATION_CATEGORY,
  formatIntegrationConnectionLabel,
} from "@/features/integrations/constants";
import { useAuthStore } from "@/store/authStore";
import { cn } from "@/lib/utils";
import type { ImportPipelinesFromCrmResult } from "@/features/admin/hooks";

const IMPORTABLE_CRM_SLUGS = new Set(["ghl", "pipedrive", "hubspot", "zoho_crm"]);

type Step = "connection" | "pick" | "confirm" | "done";

type CrmStage = {
  external_id: string;
  name: string;
  position: number;
  is_won?: boolean;
  is_closed_lost?: boolean;
  is_closed?: boolean;
};

type CrmPipeline = {
  external_id: string;
  name: string;
  stages: CrmStage[];
};

function inferStageType(stage: CrmStage, isLast: boolean): string {
  if (stage.is_won) return "won";
  if (stage.is_closed_lost || (stage.is_closed && !stage.is_won)) return "disqualification";
  if (isLast) {
    const n = stage.name.toLowerCase().trim();
    if (["won", "closed won", "sold", "closed-won", "deal won"].includes(n)) return "won";
  }
  return "standard";
}

function stageTypeBadge(type: string): string {
  switch (type) {
    case "won":
      return "Won";
    case "disqualification":
      return "Disqualified";
    default:
      return "Standard";
  }
}

interface Props {
  open: boolean;
  onClose: () => void;
}

export function ImportPipelinesFromCrmModal({ open, onClose }: Props) {
  const qc = useQueryClient();
  const accountType = useAuthStore((s) => s.user?.account_type);
  const integrationsPath = accountType === "buyer" ? "/b/integrations" : "/p/integrations";

  const { data: connections, isLoading: connectionsLoading } = useIntegrationConnections();
  const importMutation = useImportPipelinesFromCrm();

  const [step, setStep] = useState<Step>("connection");
  const [connectionId, setConnectionId] = useState<number | null>(null);
  const [providerSlug, setProviderSlug] = useState("");
  const [selected, setSelected] = useState<Set<string>>(new Set());
  const [expanded, setExpanded] = useState<Set<string>>(new Set());
  const [setupCrmMapping, setSetupCrmMapping] = useState(false);
  const [result, setResult] = useState<ImportPipelinesFromCrmResult | null>(null);

  const crmConnections = useMemo(
    () =>
      (connections ?? []).filter(
        (c) =>
          c.status === "active" &&
          INTEGRATION_CATEGORY[c.provider_slug] === "crm" &&
          IMPORTABLE_CRM_SLUGS.has(c.provider_slug)
      ),
    [connections]
  );

  const { data: crmData, isLoading: pipelinesLoading, error: pipelinesError } = useCrmPipelines(
    step !== "connection" ? connectionId : null
  );
  const pipelines = crmData?.pipelines ?? [];

  function reset() {
    setStep("connection");
    setConnectionId(null);
    setProviderSlug("");
    setSelected(new Set());
    setExpanded(new Set());
    setSetupCrmMapping(false);
    setResult(null);
  }

  function handleClose() {
    if (importMutation.isPending) return;
    reset();
    onClose();
  }

  function onSelectConnection(id: number) {
    const conn = crmConnections.find((c) => c.id === id);
    setConnectionId(id);
    setProviderSlug(conn?.provider_slug ?? "");
    setSelected(new Set());
    setExpanded(new Set());
    setStep("pick");
  }

  function togglePipeline(externalId: string) {
    setSelected((prev) => {
      const next = new Set(prev);
      if (next.has(externalId)) next.delete(externalId);
      else next.add(externalId);
      return next;
    });
  }

  function toggleExpanded(externalId: string) {
    setExpanded((prev) => {
      const next = new Set(prev);
      if (next.has(externalId)) next.delete(externalId);
      else next.add(externalId);
      return next;
    });
  }

  function selectAll() {
    setSelected(new Set(pipelines.map((p) => p.external_id)));
  }

  const selectedPipelines = pipelines.filter((p) => selected.has(p.external_id));
  const totalStages = selectedPipelines.reduce((n, p) => n + p.stages.length, 0);

  async function runImport() {
    if (!connectionId || !providerSlug || selectedPipelines.length === 0) return;
    try {
      const res = await importMutation.mutateAsync({
        connection_id: connectionId,
        provider_slug: providerSlug,
        setup_crm_mapping: setupCrmMapping,
        setup_ghl_mapping: setupCrmMapping && providerSlug === "ghl",
        pipelines: selectedPipelines.map((p) => ({
          external_id: p.external_id,
          name: p.name,
          stages: p.stages.map((s) => ({
            external_id: s.external_id,
            name: s.name,
            position: s.position,
            is_won: s.is_won,
            is_closed_lost: s.is_closed_lost,
            is_closed: s.is_closed,
          })),
        })),
      });
      setResult(res);
      setStep("done");
      qc.invalidateQueries({ queryKey: ["pipelines"] });
      qc.invalidateQueries({ queryKey: ["stages"] });
      if (setupCrmMapping) {
        qc.invalidateQueries({ queryKey: ["ghl-connection", connectionId] });
        qc.invalidateQueries({ queryKey: ["crm-connection", connectionId] });
      }
      const created = res.created.length + res.renamed.length;
      const synced = res.synced?.length ?? 0;
      if (created > 0 || synced > 0) {
        const parts: string[] = [];
        if (created > 0) parts.push(`imported ${created} pipeline${created === 1 ? "" : "s"}`);
        if (synced > 0) parts.push(`synced ${synced} existing pipeline${synced === 1 ? "" : "s"}`);
        toast.success(parts.join(", ").replace(/^./, (c) => c.toUpperCase()));
      }
    } catch (err) {
      toast.error(errorMessage(err));
    }
  }

  const title =
    step === "connection"
      ? "Import from CRM"
      : step === "pick"
        ? "Select pipelines"
        : step === "confirm"
          ? "Confirm import"
          : "Import complete";

  return (
    <Dialog
      open={open}
      onClose={handleClose}
      title={title}
      className="max-w-lg"
      footer={
        step === "connection" ? (
          <>
            <Button variant="secondary" onClick={handleClose}>
              Cancel
            </Button>
          </>
        ) : step === "pick" ? (
          <>
            <Button variant="secondary" onClick={() => setStep("connection")}>
              Back
            </Button>
            <Button
              disabled={selected.size === 0}
              onClick={() => setStep("confirm")}
            >
              Continue
            </Button>
          </>
        ) : step === "confirm" ? (
          <>
            <Button variant="secondary" onClick={() => setStep("pick")}>
              Back
            </Button>
            <Button disabled={importMutation.isPending} onClick={runImport}>
              {importMutation.isPending ? "Importing…" : "Import"}
            </Button>
          </>
        ) : (
          <Button onClick={handleClose}>Done</Button>
        )
      }
    >
      {step === "connection" && (
        <div className="space-y-3">
          <p className="text-sm text-gray-500">
            Choose a connected CRM to create or sync pipelines and stages in Leadrula.
          </p>
          {connectionsLoading ? (
            <Spinner className="h-5 w-5" />
          ) : crmConnections.length === 0 ? (
            <p className="text-sm text-gray-500">
              No eligible CRM connections found.{" "}
              <Link to={integrationsPath} className="text-jade-600 hover:underline" onClick={handleClose}>
                Connect a CRM
              </Link>
            </p>
          ) : (
            <div>
              <Label>CRM connection</Label>
              <Select
                value={connectionId ?? ""}
                onChange={(e) => {
                  const id = Number(e.target.value);
                  if (id) onSelectConnection(id);
                }}
              >
                <option value="">Select connection…</option>
                {crmConnections.map((c) => (
                  <option key={c.id} value={c.id}>
                    {formatIntegrationConnectionLabel(c)}
                  </option>
                ))}
              </Select>
            </div>
          )}
          <p className="text-xs text-gray-400">
            Supported: GoHighLevel, Pipedrive, HubSpot, Zoho CRM. Salesforce and Sunbase are not available for
            pipeline import.
          </p>
        </div>
      )}

      {step === "pick" && (
        <div className="space-y-3">
          {pipelinesLoading ? (
            <Spinner className="h-5 w-5" />
          ) : pipelinesError ? (
            <p className="text-sm text-red-600">{errorMessage(pipelinesError)}</p>
          ) : pipelines.length === 0 ? (
            <p className="text-sm text-gray-500">No pipelines found in this CRM connection.</p>
          ) : (
            <>
              <div className="flex items-center justify-between">
                <p className="text-sm text-gray-500">{pipelines.length} pipeline(s) available</p>
                <Button size="sm" variant="secondary" onClick={selectAll}>
                  Select all
                </Button>
              </div>
              <div className="max-h-64 space-y-2 overflow-y-auto">
                {pipelines.map((p) => {
                  const isExpanded = expanded.has(p.external_id);
                  return (
                    <div key={p.external_id} className="rounded-md border border-gray-100 p-2">
                      <div className="flex items-center gap-2">
                        <input
                          type="checkbox"
                          checked={selected.has(p.external_id)}
                          onChange={() => togglePipeline(p.external_id)}
                          className="h-4 w-4 rounded border-gray-300"
                        />
                        <button
                          type="button"
                          className="flex flex-1 items-center gap-1 text-left text-sm font-medium text-gray-800"
                          onClick={() => toggleExpanded(p.external_id)}
                        >
                          {isExpanded ? (
                            <ChevronDown className="h-3.5 w-3.5 shrink-0 text-gray-400" />
                          ) : (
                            <ChevronRight className="h-3.5 w-3.5 shrink-0 text-gray-400" />
                          )}
                          {p.name}
                          <span className="font-normal text-gray-400">({p.stages.length} stages)</span>
                        </button>
                      </div>
                      {isExpanded && (
                        <ul className="mt-2 space-y-1 border-t border-gray-50 pt-2 pl-6">
                          {p.stages.map((s, idx) => {
                            const type = inferStageType(s, idx === p.stages.length - 1);
                            return (
                              <li key={s.external_id} className="flex items-center justify-between text-xs text-gray-600">
                                <span>{s.name}</span>
                                <span
                                  className={cn(
                                    "rounded px-1.5 py-0.5 text-[10px] font-medium",
                                    type === "won"
                                      ? "bg-green-50 text-green-700"
                                      : type === "disqualification"
                                        ? "bg-red-50 text-red-600"
                                        : "bg-gray-100 text-gray-500"
                                  )}
                                >
                                  {stageTypeBadge(type)}
                                </span>
                              </li>
                            );
                          })}
                        </ul>
                      )}
                    </div>
                  );
                })}
              </div>
            </>
          )}
        </div>
      )}

      {step === "confirm" && (
        <div className="space-y-3">
          <p className="text-sm text-gray-700">
            Import or sync <strong>{selectedPipelines.length}</strong> pipeline
            {selectedPipelines.length === 1 ? "" : "s"} with <strong>{totalStages}</strong> stage
            {totalStages === 1 ? "" : "s"}.
          </p>
          <p className="text-xs text-gray-500">
            Pipelines with the same name as an existing Leadrula pipeline will have stages synced (add, rename,
            reorder). New names create new pipelines.
          </p>
          <ul className="max-h-40 space-y-1 overflow-y-auto text-sm text-gray-600">
            {selectedPipelines.map((p) => (
              <li key={p.external_id}>
                {p.name} ({p.stages.length} stages)
              </li>
            ))}
          </ul>
          <label className="flex items-start gap-2 text-sm text-gray-700">
            <input
              type="checkbox"
              checked={setupCrmMapping}
              onChange={(e) => setSetupCrmMapping(e.target.checked)}
              className="mt-0.5 h-4 w-4 rounded border-gray-300"
            />
            <span>
              Set up CRM pipeline/stage mapping
              <span className="block text-xs text-gray-400">
                Maps imported Leadrula stages to CRM stages for inbound lead stage sync.
              </span>
            </span>
          </label>
          <p className="text-xs text-gray-400">
            Pipelines with duplicate names (when no existing match) will be renamed with the CRM provider suffix.
          </p>
        </div>
      )}

      {step === "done" && result && (
        <div className="space-y-3 text-sm">
          {result.created.length > 0 && (
            <div>
              <p className="font-medium text-gray-700">Created</p>
              <ul className="mt-1 space-y-0.5 text-gray-600">
                {result.created.map((p) => (
                  <li key={p.id}>
                    {p.name} ({p.stage_count} stages)
                  </li>
                ))}
              </ul>
            </div>
          )}
          {(result.synced?.length ?? 0) > 0 && (
            <div>
              <p className="font-medium text-gray-700">Synced (existing pipeline)</p>
              <ul className="mt-1 space-y-0.5 text-gray-600">
                {result.synced!.map((p) => (
                  <li key={p.id}>
                    {p.name}
                    {p.stages_added > 0 && ` · ${p.stages_added} stage${p.stages_added === 1 ? "" : "s"} added`}
                    {p.stages_renamed > 0 &&
                      ` · ${p.stages_renamed} stage${p.stages_renamed === 1 ? "" : "s"} renamed`}
                    {p.stages_reordered && " · reordered"}
                  </li>
                ))}
              </ul>
            </div>
          )}
          {result.renamed.length > 0 && (
            <div>
              <p className="font-medium text-gray-700">Renamed (name conflict)</p>
              <ul className="mt-1 space-y-0.5 text-gray-600">
                {result.renamed.map((p) => (
                  <li key={p.id}>
                    {p.original_name} → {p.final_name} ({p.stage_count} stages)
                  </li>
                ))}
              </ul>
            </div>
          )}
          {result.skipped.length > 0 && (
            <div>
              <p className="font-medium text-gray-700">Skipped</p>
              <ul className="mt-1 space-y-0.5 text-gray-600">
                {result.skipped.map((p, i) => (
                  <li key={`${p.name}-${i}`}>
                    {p.name}: {p.reason}
                  </li>
                ))}
              </ul>
            </div>
          )}
        </div>
      )}
    </Dialog>
  );
}
