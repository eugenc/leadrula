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
  COMPENSATION_KINDS,
  COMPENSATION_TRIGGERS,
  defaultTriggerForKind,
  flatRateAmountLabel,
  formatCompTrigger,
  payoutFieldsToBody,
  pipelineFieldsToBody,
  type CompensationKind,
  type ContractDeliveryDraft,
} from "@/features/admin/contractCompensation";
import {
  CompensationPayoutFields,
  defaultPayoutDraft,
  payoutDraftFromComp,
  type PayoutDraftFields,
} from "@/features/admin/CompensationPayoutFields";
import {
  useAddContractCompensation,
  useDeleteContractCompensation,
  useUpdateContractCompensation,
} from "@/features/admin/hooks";
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
} & PayoutDraftFields;

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
    ...payoutDraftFromComp(c),
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
    ...defaultPayoutDraft(),
  };
}

function draftToBody(d: Draft, deliveryDraft: ContractDeliveryDraft): Record<string, unknown> {
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
    ...pipelineFieldsToBody(deliveryDraft.delivery, deliveryDraft),
    delivery: deliveryDraft.delivery,
    position: 0,
    ...payoutFieldsToBody(d.payout_frequency, d.payout_weekday, d.payout_month_day),
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
  function set<K extends keyof Draft>(k: K, v: Draft[K]) {
    setDraft({ ...draft, [k]: v });
  }

  return (
    <div className="flex flex-col gap-2.5">
      <div>
        <Label>Type</Label>
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
          <Label>{flatRateAmountLabel(contract.lead_type ?? "")}</Label>
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
            <p className="text-xs text-gray-500">
              Buyer will choose the trigger stage on their pipeline after the contract is active.
            </p>
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

      <CompensationPayoutFields
        draft={{
          payout_frequency: draft.payout_frequency,
          payout_weekday: draft.payout_weekday,
          payout_month_day: draft.payout_month_day,
        }}
        onChange={(p) => setDraft({ ...draft, ...p })}
      />
      {(draft.kind === "rev_share" || draft.kind === "profit_share") && (
        <p className="text-xs text-gray-500">
          Payouts are settled via invoice. Cleared amounts auto-generate invoices for the buyer.
        </p>
      )}
    </div>
  );
}

export function ContractCompensationEditor({
  contract,
  items,
  deliveryDraft,
}: {
  contract: Contract;
  items: ContractCompensation[];
  deliveryDraft: ContractDeliveryDraft;
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
    const body = draftToBody(draft, deliveryDraft);
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
                  {formatCompTrigger(c.trigger)} · {formatCapPeriod(c.cap_period)}
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
