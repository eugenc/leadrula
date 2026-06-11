import { Button } from "@/components/ui/button";
import { Input, Label, Select } from "@/components/ui/input";
import { SectionLabel } from "@/components/layout/SectionLabel";
import {
  CONTRACT_CAP_PERIODS,
  capPeriodShowsDailyCap,
  capInputValue,
  isContractCapPeriod,
  parseCapInput,
} from "@/features/admin/contractCap";
import {
  COMPENSATION_KINDS,
  COMPENSATION_TRIGGERS,
  defaultTriggerForKind,
  flatRateAmountLabel,
  payoutFieldsToBody,
  pipelineFieldsToBody,
  type CompensationKind,
  type ContractDeliveryDraft,
} from "@/features/admin/contractCompensation";
import { type ContractType } from "@/features/admin/contractType";
import {
  CompensationPayoutFields,
  defaultPayoutDraft,
  type PayoutDraftFields,
} from "@/features/admin/CompensationPayoutFields";
import { payoutDraftFromComp } from "@/features/admin/CompensationPayoutFields";
import type { ContractCompensation } from "@/types";

export type CompensationDraft = {
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

export function compensationDraftFromComp(
  c: ContractCompensation,
  ratePerLead?: number
): CompensationDraft {
  let flatAmount = c.flat_amount != null ? String(c.flat_amount) : "";
  if (flatAmount === "" && c.kind === "flat_rate" && ratePerLead != null && ratePerLead > 0) {
    flatAmount = String(ratePerLead);
  }
  return {
    kind: c.kind as CompensationKind,
    flat_amount: flatAmount,
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

export function emptyCompensationDraft(): CompensationDraft {
  return {
    kind: "flat_rate",
    flat_amount: "25",
    bid_min: "",
    bid_max: "",
    rev_percent: "",
    profit_percent: "",
    cap_period: "one_time",
    cap_total: "",
    cap_max_daily: "",
    trigger: "per_lead",
    trigger_stage_id: 0,
    ...defaultPayoutDraft(),
  };
}

/** Empty defaults for the New Contract create drawer (no pre-filled rate or payout). */
export function blankCompensationDraft(): CompensationDraft {
  return {
    kind: "flat_rate",
    flat_amount: "",
    bid_min: "",
    bid_max: "",
    rev_percent: "",
    profit_percent: "",
    cap_period: "one_time",
    cap_total: "",
    cap_max_daily: "",
    trigger: "per_lead",
    trigger_stage_id: 0,
    payout_frequency: "",
    payout_weekday: 0,
    payout_month_day: 0,
  };
}

export function compensationDraftToBody(
  d: CompensationDraft,
  deliveryDraft: ContractDeliveryDraft
): Record<string, unknown> {
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
    ...payoutFieldsToBody(d.payout_frequency, d.payout_weekday, d.payout_month_day),
  };
}

export function compensationsValid(items: CompensationDraft[]): boolean {
  if (items.length === 0) return false;
  return items.every((d) => {
    if (!isContractCapPeriod(d.cap_period)) return false;
    if (d.kind === "flat_rate") return d.flat_amount !== "" && Number(d.flat_amount) >= 0;
    if (d.kind === "bid") return d.bid_min !== "" && d.bid_max !== "";
    if (d.kind === "rev_share") return d.rev_percent !== "";
    if (d.kind === "profit_share") return d.profit_percent !== "";
    return true;
  });
}

function CompensationCard({
  draft,
  index,
  buyerId: _buyerId,
  buyerPipelineId: _buyerPipelineId,
  leadType,
  onChange,
  onRemove,
  canRemove,
}: {
  draft: CompensationDraft;
  index: number;
  buyerId: number;
  buyerPipelineId: number;
  leadType: string;
  onChange: (d: CompensationDraft) => void;
  onRemove: () => void;
  canRemove: boolean;
}) {
  function set<K extends keyof CompensationDraft>(k: K, v: CompensationDraft[K]) {
    onChange({ ...draft, [k]: v });
  }

  return (
    <div className="rounded-lg border border-gray-100 p-3">
      <div className="mb-2 flex items-center justify-between">
        <span className="text-sm font-semibold text-gray-700">Compensation #{index + 1}</span>
        {canRemove && (
          <Button variant="danger" className="h-7 px-2 text-xs" onClick={onRemove}>
            Remove
          </Button>
        )}
      </div>
      <div className="flex flex-col gap-2.5">
        <div>
          <Label>Type</Label>
          <Select
            value={draft.kind}
            onChange={(e) => {
              const kind = e.target.value as CompensationKind;
              onChange({ ...draft, kind, trigger: defaultTriggerForKind(kind) });
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
            <Label>{flatRateAmountLabel(leadType)}</Label>
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

        <SectionLabel className="mt-1">Cap limits</SectionLabel>
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
          onChange={(p) => onChange({ ...draft, ...p })}
        />
      </div>
    </div>
  );
}

export function CreateContractCompensationList({
  buyerId,
  buyerPipelineId,
  leadType,
  items,
  onChange,
  blankNewRows,
}: {
  buyerId: number;
  contractType: ContractType;
  buyerPipelineId: number;
  leadType: string;
  items: CompensationDraft[];
  onChange: (items: CompensationDraft[]) => void;
  blankNewRows?: boolean;
}) {
  const newRow = blankNewRows ? blankCompensationDraft : emptyCompensationDraft;
  return (
    <div className="flex flex-col gap-3">
      {items.map((d, i) => (
        <CompensationCard
          key={i}
          index={i}
          draft={d}
          buyerId={buyerId}
          buyerPipelineId={buyerPipelineId}
          leadType={leadType}
          canRemove={items.length > 1}
          onChange={(next) => onChange(items.map((x, j) => (j === i ? next : x)))}
          onRemove={() => onChange(items.filter((_, j) => j !== i))}
        />
      ))}
      <Button variant="secondary" onClick={() => onChange([...items, newRow()])}>
        Add compensation
      </Button>
    </div>
  );
}
