import { useState } from "react";
import { Button } from "@/components/ui/button";
import { Input, Label, Select } from "@/components/ui/input";
import { SectionLabel } from "@/components/layout/SectionLabel";
import {
  CONTRACT_CAP_PERIODS,
  capPeriodShowsDailyCap,
  capInputValue,
  formatCapPeriod,
  isContractCapPeriod,
  parseCapInput,
} from "@/features/admin/contractCap";
import {
  COMPENSATION_DELIVERY,
  COMPENSATION_KINDS,
  COMPENSATION_TRIGGERS,
  defaultTriggerForKind,
  type CompensationKind,
} from "@/features/admin/contractCompensation";
import {
  useAddContractCompensation,
  useDeleteContractCompensation,
  useUpdateContractCompensation,
  useBuyerStages,
  useBuyerPipelines,
} from "@/features/admin/hooks";
import { usePipelines, useStages } from "@/features/leads/hooks";
import { toast } from "@/store/toastStore";
import { errorMessage } from "@/lib/api";
import type { Contract, ContractCompensation } from "@/types";

type Draft = {
  kind: CompensationKind;
  flat_amount: string;
  bid_min: string;
  bid_max: string;
  rev_percent: string;
  profit_percent: string;
  cap_period: string;
  cap_total: string;
  cap_max_daily: string;
  trigger: string;
  trigger_stage_id: number;
  source_pipeline_id: number;
  source_stage_id: number;
  counterparty_pipeline_id: number;
  counterparty_stage_id: number;
  return_stage_id: number;
  delivery: string;
};

function draftFromComp(c: ContractCompensation): Draft {
  return {
    kind: c.kind as CompensationKind,
    flat_amount: c.flat_amount != null ? String(c.flat_amount) : "",
    bid_min: c.bid_min != null ? String(c.bid_min) : "",
    bid_max: c.bid_max != null ? String(c.bid_max) : "",
    rev_percent: c.rev_percent != null ? String(c.rev_percent) : "",
    profit_percent: c.profit_percent != null ? String(c.profit_percent) : "",
    cap_period: c.cap_period ?? "one_time",
    cap_total: capInputValue(c.cap_total),
    cap_max_daily: capInputValue(c.cap_max_daily),
    trigger: c.trigger,
    trigger_stage_id: c.trigger_stage_id ?? 0,
    source_pipeline_id: c.source_pipeline_id ?? 0,
    source_stage_id: c.source_stage_id ?? 0,
    counterparty_pipeline_id: c.counterparty_pipeline_id ?? 0,
    counterparty_stage_id: c.counterparty_stage_id ?? 0,
    return_stage_id: c.return_stage_id ?? 0,
    delivery: c.delivery ?? "leads_pipeline",
  };
}

function emptyDraft(contract: Contract): Draft {
  return {
    kind: "flat_rate",
    flat_amount: String(contract.rate_per_lead ?? 0),
    bid_min: "",
    bid_max: "",
    rev_percent: "",
    profit_percent: "",
    cap_period: contract.cap_period ?? "one_time",
    cap_total: capInputValue(contract.cap_total),
    cap_max_daily: capInputValue(contract.cap_max_daily),
    trigger: "per_lead",
    trigger_stage_id: 0,
    source_pipeline_id: contract.source_pipeline_id,
    source_stage_id: contract.source_stage_id,
    counterparty_pipeline_id: contract.buyer_pipeline_id,
    counterparty_stage_id: 0,
    return_stage_id: contract.return_stage_id,
    delivery: "leads_pipeline",
  };
}

function draftToBody(d: Draft): Record<string, unknown> {
  const capTotal = parseCapInput(d.cap_total);
  const capMaxDaily = parseCapInput(d.cap_max_daily);
  return {
    kind: d.kind,
    flat_amount: d.flat_amount === "" ? null : Number(d.flat_amount),
    bid_min: d.bid_min === "" ? null : Number(d.bid_min),
    bid_max: d.bid_max === "" ? null : Number(d.bid_max),
    rev_percent: d.rev_percent === "" ? null : Number(d.rev_percent),
    profit_percent: d.profit_percent === "" ? null : Number(d.profit_percent),
    cap_period: d.cap_period,
    cap_total: capTotal,
    cap_max_daily: capPeriodShowsDailyCap(d.cap_period) ? capMaxDaily : null,
    trigger: d.trigger,
    trigger_stage_id: d.trigger_stage_id || null,
    source_pipeline_id: d.source_pipeline_id || null,
    source_stage_id: d.source_stage_id || null,
    counterparty_pipeline_id: d.counterparty_pipeline_id || null,
    counterparty_stage_id: d.counterparty_stage_id || null,
    return_stage_id: d.return_stage_id || null,
    delivery: d.delivery,
    position: 0,
  };
}

function CompensationFields({
  contract,
  draft,
  setDraft,
}: {
  contract: Contract;
  draft: Draft;
  setDraft: (d: Draft) => void;
}) {
  const { data: pubPipelines } = usePipelines();
  const { data: sourceStages } = useStages(draft.source_pipeline_id || undefined);
  const { data: buyerPipelines } = useBuyerPipelines(contract.buyer_id || null);
  const { data: buyerStages } = useBuyerStages(
    contract.buyer_id,
    draft.counterparty_pipeline_id || contract.buyer_pipeline_id
  );

  function set<K extends keyof Draft>(k: K, v: Draft[K]) {
    setDraft({ ...draft, [k]: v });
  }

  return (
    <div className="flex flex-col gap-2.5">
      <div>
        <Label>Kind</Label>
        <Select
          value={draft.kind}
          onChange={(e) => {
            const kind = e.target.value as CompensationKind;
            setDraft({
              ...draft,
              kind,
              trigger: defaultTriggerForKind(kind),
            });
          }}
        >
          {COMPENSATION_KINDS.map((k) => (
            <option key={k.value} value={k.value}>
              {k.label}
            </option>
          ))}
        </Select>
      </div>
      {draft.kind === "flat_rate" && (
        <div>
          <Label>Flat amount (USD)</Label>
          <Input type="number" min={0} step={0.01} value={draft.flat_amount} onChange={(e) => set("flat_amount", e.target.value)} />
        </div>
      )}
      {draft.kind === "bid" && (
        <div className="grid grid-cols-2 gap-3">
          <div>
            <Label>Bid min</Label>
            <Input type="number" min={0} value={draft.bid_min} onChange={(e) => set("bid_min", e.target.value)} />
          </div>
          <div>
            <Label>Bid max</Label>
            <Input type="number" min={0} value={draft.bid_max} onChange={(e) => set("bid_max", e.target.value)} />
          </div>
        </div>
      )}
      {draft.kind === "rev_share" && (
        <div>
          <Label>Rev share %</Label>
          <Input type="number" min={0} max={100} value={draft.rev_percent} onChange={(e) => set("rev_percent", e.target.value)} />
        </div>
      )}
      {draft.kind === "profit_share" && (
        <div>
          <Label>Profit share %</Label>
          <Input type="number" min={0} max={100} value={draft.profit_percent} onChange={(e) => set("profit_percent", e.target.value)} />
        </div>
      )}
      {(draft.kind === "rev_share" || draft.kind === "profit_share") && (
        <>
          <div>
            <Label>Trigger</Label>
            <Select value={draft.trigger} onChange={(e) => set("trigger", e.target.value)}>
              {COMPENSATION_TRIGGERS.filter((t) => t.value !== "per_lead").map((t) => (
                <option key={t.value} value={t.value}>
                  {t.label}
                </option>
              ))}
            </Select>
          </div>
          {draft.trigger === "buyer_stage" && (
            <div>
              <Label>Trigger stage (buyer)</Label>
              <Select value={draft.trigger_stage_id} onChange={(e) => set("trigger_stage_id", Number(e.target.value))}>
                <option value={0}>Select…</option>
                {(buyerStages ?? []).map((s) => (
                  <option key={s.id} value={s.id}>
                    {s.name}
                  </option>
                ))}
              </Select>
            </div>
          )}
        </>
      )}

      <SectionLabel className="mt-2">Cap limits</SectionLabel>
      <div>
        <Label>Cap period</Label>
        <Select value={draft.cap_period} onChange={(e) => set("cap_period", e.target.value)}>
          {CONTRACT_CAP_PERIODS.map((p) => (
            <option key={p.value} value={p.value}>
              {p.label}
            </option>
          ))}
        </Select>
      </div>
      <div className="grid grid-cols-2 gap-3">
        <div>
          <Label>Cap total</Label>
          <Input type="number" min={1} placeholder="Unlimited" value={draft.cap_total} onChange={(e) => set("cap_total", e.target.value)} />
        </div>
        {capPeriodShowsDailyCap(draft.cap_period) && (
          <div>
            <Label>Max daily</Label>
            <Input type="number" min={1} placeholder="No daily cap" value={draft.cap_max_daily} onChange={(e) => set("cap_max_daily", e.target.value)} />
          </div>
        )}
      </div>

      <SectionLabel className="mt-2">Delivery</SectionLabel>
      <div>
        <Label>Delivery mode</Label>
        <Select value={draft.delivery} onChange={(e) => set("delivery", e.target.value)}>
          {COMPENSATION_DELIVERY.map((d) => (
            <option key={d.value} value={d.value}>
              {d.label}
            </option>
          ))}
        </Select>
      </div>
      <div className="grid grid-cols-2 gap-3">
        <div>
          <Label>Source pipeline</Label>
          <Select value={draft.source_pipeline_id} onChange={(e) => set("source_pipeline_id", Number(e.target.value))}>
            <option value={0}>Select…</option>
            {(pubPipelines ?? []).map((p) => (
              <option key={p.id} value={p.id}>
                {p.name}
              </option>
            ))}
          </Select>
        </div>
        <div>
          <Label>Source stage</Label>
          <Select value={draft.source_stage_id} onChange={(e) => set("source_stage_id", Number(e.target.value))}>
            <option value={0}>Select…</option>
            {(sourceStages ?? []).map((s) => (
              <option key={s.id} value={s.id}>
                {s.name}
              </option>
            ))}
          </Select>
        </div>
      </div>
      <div className="grid grid-cols-2 gap-3">
        <div>
          <Label>Counterparty pipeline</Label>
          <Select value={draft.counterparty_pipeline_id} onChange={(e) => set("counterparty_pipeline_id", Number(e.target.value))}>
            <option value={0}>Select…</option>
            {(buyerPipelines ?? []).map((p) => (
              <option key={p.id} value={p.id}>
                {p.name}
              </option>
            ))}
          </Select>
        </div>
        <div>
          <Label>Return stage</Label>
          <Select value={draft.return_stage_id} onChange={(e) => set("return_stage_id", Number(e.target.value))}>
            <option value={0}>Select…</option>
            {(sourceStages ?? []).map((s) => (
              <option key={s.id} value={s.id}>
                {s.name}
              </option>
            ))}
          </Select>
        </div>
      </div>
    </div>
  );
}

export function ContractCompensationEditor({
  contract,
  items,
}: {
  contract: Contract;
  items: ContractCompensation[];
}) {
  const add = useAddContractCompensation();
  const update = useUpdateContractCompensation();
  const remove = useDeleteContractCompensation();
  const [editingId, setEditingId] = useState<number | "new" | null>(null);
  const [draft, setDraft] = useState<Draft>(() => emptyDraft(contract));

  function startEdit(c: ContractCompensation) {
    setEditingId(c.id);
    setDraft(draftFromComp(c));
  }

  function startNew() {
    setEditingId("new");
    setDraft(emptyDraft(contract));
  }

  function save() {
    if (!isContractCapPeriod(draft.cap_period)) {
      toast.error("Invalid cap period");
      return;
    }
    const body = draftToBody(draft);
    if (editingId === "new") {
      add.mutate(
        { contractId: contract.id, body },
        {
          onSuccess: () => {
            toast.success("Compensation added");
            setEditingId(null);
          },
          onError: (e) => toast.error(errorMessage(e)),
        }
      );
    } else if (typeof editingId === "number") {
      update.mutate(
        { contractId: contract.id, compId: editingId, body },
        {
          onSuccess: () => {
            toast.success("Compensation saved");
            setEditingId(null);
          },
          onError: (e) => toast.error(errorMessage(e)),
        }
      );
    }
  }

  return (
    <div className="flex flex-col gap-3">
      {(items ?? []).map((c) => (
        <div key={c.id} className="rounded-lg border border-gray-100 p-3">
          {editingId === c.id ? (
            <>
              <CompensationFields contract={contract} draft={draft} setDraft={setDraft} />
              <div className="mt-3 flex gap-2">
                <Button onClick={save} disabled={add.isPending || update.isPending}>
                  Save
                </Button>
                <Button variant="secondary" onClick={() => setEditingId(null)}>
                  Cancel
                </Button>
              </div>
            </>
          ) : (
            <div className="flex items-start justify-between gap-2">
              <div className="text-sm">
                <div className="font-semibold text-gray-800">
                  {COMPENSATION_KINDS.find((k) => k.value === c.kind)?.label ?? c.kind}
                </div>
                <div className="text-gray-500">
                  {c.trigger} · {formatCapPeriod(c.cap_period)}
                  {c.flat_amount != null ? ` · $${c.flat_amount}/lead` : ""}
                  {c.bid_max != null ? ` · bid up to $${c.bid_max}` : ""}
                  {c.rev_percent != null ? ` · ${c.rev_percent}% rev` : ""}
                  {c.profit_percent != null ? ` · ${c.profit_percent}% profit` : ""}
                </div>
              </div>
              <div className="flex shrink-0 gap-1">
                <Button variant="secondary" className="h-7 px-2 text-xs" onClick={() => startEdit(c)}>
                  Edit
                </Button>
                <Button
                  variant="danger"
                  className="h-7 px-2 text-xs"
                  onClick={() =>
                    remove.mutate(
                      { contractId: contract.id, compId: c.id },
                      { onError: (e) => toast.error(errorMessage(e)) }
                    )
                  }
                >
                  Remove
                </Button>
              </div>
            </div>
          )}
        </div>
      ))}
      {editingId === "new" ? (
        <div className="rounded-lg border border-dashed border-gray-200 p-3">
          <CompensationFields contract={contract} draft={draft} setDraft={setDraft} />
          <div className="mt-3 flex gap-2">
            <Button onClick={save} disabled={add.isPending}>
              Add
            </Button>
            <Button variant="secondary" onClick={() => setEditingId(null)}>
              Cancel
            </Button>
          </div>
        </div>
      ) : (
        <Button variant="secondary" onClick={startNew}>
          Add compensation
        </Button>
      )}
    </div>
  );
}
