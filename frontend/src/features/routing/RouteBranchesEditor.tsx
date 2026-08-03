import { useContracts, useBuyerContracts } from "@/features/admin/hooks";
import { usePipelines, useStages } from "@/features/leads/hooks";
import { useWebhooks } from "@/features/webhooks/hooks";
import {
  RouteDestinationIntegrationsEditor,
  type RouteDestinationIntegrationSelection,
} from "@/features/integrations/RouteDestinationIntegrationsEditor";
import { Button } from "@/components/ui/button";
import { Input, Label, Select } from "@/components/ui/input";
import { Spinner } from "@/components/ui/misc";
import { Plus, Trash2 } from "lucide-react";
import type { RouteBranch } from "@/types";
import {
  BUYER_DESTINATIONS,
  DESTINATION_LABELS,
  PUBLISHER_DESTINATIONS,
} from "./routeFormatters";
import { RouteConditionsEditor } from "./RouteConditionsEditor";
import { blankBranch, reindexBranches } from "./routeBranchUtils";

type Destination = RouteBranch["destination"];

function BranchDestinationFields({
  accountType,
  branch,
  onChange,
  integrations,
  onIntegrationsChange,
  integrationsLoading,
  disabled,
}: {
  accountType: "publisher" | "buyer";
  branch: RouteBranch;
  onChange: (next: RouteBranch) => void;
  integrations: RouteDestinationIntegrationSelection[];
  onIntegrationsChange: (next: RouteDestinationIntegrationSelection[]) => void;
  integrationsLoading?: boolean;
  disabled?: boolean;
}) {
  const destinations = accountType === "publisher" ? PUBLISHER_DESTINATIONS : BUYER_DESTINATIONS;
  const { data: publisherContracts } = useContracts(accountType === "publisher");
  const { data: buyerContracts } = useBuyerContracts();
  const contracts = accountType === "publisher" ? publisherContracts : buyerContracts;
  const { data: pipelines } = usePipelines();
  const { data: webhooks } = useWebhooks();
  const { data: targetStages } = useStages(
    branch.destination === "pipeline" && branch.delivery === "leads_pipeline"
      ? branch.target_pipeline_id ?? undefined
      : undefined
  );
  const selectedContract = (contracts ?? []).find((c) => c.id === branch.contract_id);
  const { data: contractBuyerStages } = useStages(
    branch.destination === "contract" && branch.delivery === "leads_pipeline"
      ? selectedContract?.buyer_pipeline_id ?? undefined
      : undefined
  );

  return (
    <div className="space-y-3">
      <div>
        <Label>Destination type</Label>
        <Select
          value={branch.destination}
          disabled={disabled}
          onChange={(e) =>
            onChange({
              ...branch,
              destination: e.target.value as Destination,
              delivery: e.target.value === "integration" || e.target.value === "webhook" ? "leads" : "leads_pipeline",
            })
          }
        >
          {destinations.map((d) => (
            <option key={d} value={d}>
              {DESTINATION_LABELS[d]}
            </option>
          ))}
        </Select>
      </div>
      {branch.destination === "contract" && (
        <>
          <div>
            <Label>Contract</Label>
            <Select
              value={branch.contract_id ?? 0}
              disabled={disabled}
              onChange={(e) => onChange({ ...branch, contract_id: Number(e.target.value) || null })}
            >
              <option value={0}>Select…</option>
              {(contracts ?? []).map((c) => (
                <option key={c.id} value={c.id}>
                  {c.buyer_name ?? c.publisher_name ?? c.name}
                </option>
              ))}
            </Select>
          </div>
          <div>
            <Label>Delivery</Label>
            <Select
              value={branch.delivery}
              disabled={disabled}
              onChange={(e) => onChange({ ...branch, delivery: e.target.value as RouteBranch["delivery"] })}
            >
              <option value="leads">Lead</option>
              <option value="leads_pipeline">Pipeline</option>
            </Select>
          </div>
          {branch.delivery === "leads_pipeline" && (
            <div>
              <Label>Target stage</Label>
              <Select
                value={branch.target_stage_id ?? 0}
                disabled={disabled}
                onChange={(e) => onChange({ ...branch, target_stage_id: Number(e.target.value) || null })}
              >
                <option value={0}>First stage (default)</option>
                {(contractBuyerStages ?? []).map((s) => (
                  <option key={s.id} value={s.id}>
                    {s.name}
                  </option>
                ))}
              </Select>
            </div>
          )}
        </>
      )}
      {branch.destination === "pipeline" && (
        <>
          <div>
            <Label>Delivery</Label>
            <Select
              value={branch.delivery}
              disabled={disabled}
              onChange={(e) => onChange({ ...branch, delivery: e.target.value as RouteBranch["delivery"] })}
            >
              <option value="leads">Lead</option>
              <option value="leads_pipeline">Pipeline</option>
            </Select>
          </div>
          {branch.delivery === "leads_pipeline" && (
            <div className="grid grid-cols-2 gap-3">
              <div>
                <Label>Pipeline</Label>
                <Select
                  value={branch.target_pipeline_id ?? 0}
                  disabled={disabled}
                  onChange={(e) =>
                    onChange({ ...branch, target_pipeline_id: Number(e.target.value) || null, target_stage_id: null })
                  }
                >
                  <option value={0}>Select…</option>
                  {(pipelines ?? []).map((p) => (
                    <option key={p.id} value={p.id}>
                      {p.name}
                    </option>
                  ))}
                </Select>
              </div>
              <div>
                <Label>Target stage</Label>
                <Select
                  value={branch.target_stage_id ?? 0}
                  disabled={disabled}
                  onChange={(e) => onChange({ ...branch, target_stage_id: Number(e.target.value) || null })}
                >
                  <option value={0}>Select…</option>
                  {(targetStages ?? []).map((s) => (
                    <option key={s.id} value={s.id}>
                      {s.name}
                    </option>
                  ))}
                </Select>
              </div>
            </div>
          )}
        </>
      )}
      {branch.destination === "webhook" && (
        <div>
          <Label>Webhook</Label>
          <Select
            value={branch.dest_webhook_id ?? 0}
            disabled={disabled}
            onChange={(e) => onChange({ ...branch, dest_webhook_id: Number(e.target.value) || null })}
          >
            <option value={0}>Select…</option>
            {(webhooks ?? []).filter((w) => w.outbound_enabled).map((w) => (
              <option key={w.id} value={w.id}>
                {w.name}
              </option>
            ))}
          </Select>
        </div>
      )}
      {branch.destination === "integration" &&
        (integrationsLoading ? (
          <Spinner className="h-5 w-5" />
        ) : (
          <RouteDestinationIntegrationsEditor
            selected={integrations}
            onChange={onIntegrationsChange}
            disabled={disabled}
          />
        ))}
    </div>
  );
}

export function RouteBranchesEditor({
  accountType,
  branches,
  onChange,
  integrationSelections,
  onIntegrationSelectionsChange,
  integrationsLoading,
  showPayloadDomain,
  disabled,
}: {
  accountType: "publisher" | "buyer";
  branches: RouteBranch[];
  onChange: (next: RouteBranch[]) => void;
  integrationSelections: Record<number, RouteDestinationIntegrationSelection[]>;
  onIntegrationSelectionsChange: (next: Record<number, RouteDestinationIntegrationSelection[]>) => void;
  integrationsLoading?: boolean;
  showPayloadDomain?: boolean;
  disabled?: boolean;
}) {
  function updateBranch(index: number, next: RouteBranch) {
    const copy = [...branches];
    copy[index] = next;
    onChange(copy);
  }

  function removeBranch(index: number) {
    if (branches.length <= 1) return;
    const removed = branches[index];
    const next = reindexBranches(branches.filter((_, i) => i !== index));
    onChange(next);
    const nextIntegrations = { ...integrationSelections };
    delete nextIntegrations[removed.position];
    onIntegrationSelectionsChange(nextIntegrations);
  }

  const multi = branches.length > 1;

  function renderBranch(index: number, branch: RouteBranch) {
    const conditions = (
      <RouteConditionsEditor
        conditionLogic={branch.condition_logic}
        conditions={branch.conditions}
        onConditionLogicChange={(v) => updateBranch(index, { ...branch, condition_logic: v })}
        onConditionsChange={(v) => updateBranch(index, { ...branch, conditions: v })}
        disabled={disabled}
        showPayloadDomain={showPayloadDomain}
        embedded={multi}
      />
    );
    const routeFields = (
      <BranchDestinationFields
        accountType={accountType}
        branch={branch}
        onChange={(next) => updateBranch(index, next)}
        integrations={integrationSelections[branch.position] ?? []}
        onIntegrationsChange={(next) =>
          onIntegrationSelectionsChange({ ...integrationSelections, [branch.position]: next })
        }
        integrationsLoading={integrationsLoading}
        disabled={disabled}
      />
    );

    if (!multi) {
      return (
        <div key={branch.position} className="space-y-3">
          {routeFields}
        </div>
      );
    }

    return (
      <div key={branch.position} className="space-y-3 rounded-lg border border-border p-3">
        <div className="flex items-center justify-between gap-2">
          <Input
            className="font-medium"
            value={branch.name ?? `Route ${index + 1}`}
            disabled={disabled}
            placeholder={`Route ${index + 1}`}
            onChange={(e) => updateBranch(index, { ...branch, name: e.target.value })}
          />
          <Button type="button" size="sm" variant="secondary" disabled={disabled} onClick={() => removeBranch(index)}>
            <Trash2 className="h-3.5 w-3.5" />
          </Button>
        </div>
        <div className="space-y-2">
          <Label>Conditions</Label>
          {conditions}
        </div>
        <div className="space-y-2">
          <Label>Route</Label>
          {routeFields}
        </div>
      </div>
    );
  }

  return (
    <div className="space-y-4">
      {multi && (
        <p className="text-sm text-muted-foreground">
          Routes run in order. The first matching route applies.
        </p>
      )}
      {branches.map((branch, index) => renderBranch(index, branch))}
      <Button
        type="button"
        size="sm"
        variant="outline"
        disabled={disabled}
        onClick={() => onChange(reindexBranches([...branches, blankBranch(branches.length, accountType)]))}
      >
        <Plus className="h-3.5 w-3.5" /> Add New Route
      </Button>
    </div>
  );
}
