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
  COMPENSATION_DELIVERY,
  COMPENSATION_KINDS,
  COMPENSATION_TRIGGERS,
  defaultTriggerForKind,
  type CompensationKind,
} from "@/features/admin/contractCompensation";
import { useBuyerPipelines, useBuyerStages } from "@/features/admin/hooks";
import { usePipelines, useStages } from "@/features/leads/hooks";

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
  source_pipeline_id: number;
  source_stage_id: number;
  counterparty_pipeline_id: number;
  counterparty_stage_id: number;
  return_stage_id: number;
  delivery: string;
};

export function emptyCompensationDraft(pipelines: {
  source_pipeline_id: number;
  source_stage_id: number;
  counterparty_pipeline_id: number;
  return_stage_id: number;
}): CompensationDraft {
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
    source_pipeline_id: pipelines.source_pipeline_id,
    source_stage_id: pipelines.source_stage_id,
    counterparty_pipeline_id: pipelines.counterparty_pipeline_id,
    counterparty_stage_id: 0,
    return_stage_id: pipelines.return_stage_id,
    delivery: "leads_pipeline",
  };
}

export function compensationDraftToBody(d: CompensationDraft): Record<string, unknown> {
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
  buyerId,
  onChange,
  onRemove,
  canRemove,
}: {
  draft: CompensationDraft;
  index: number;
  buyerId: number;
  onChange: (d: CompensationDraft) => void;
  onRemove: () => void;
  canRemove: boolean;
}) {
  const { data: pubPipelines } = usePipelines();
  const { data: sourceStages } = useStages(draft.source_pipeline_id || undefined);
  const { data: buyerPipelines } = useBuyerPipelines(buyerId || null);
  const { data: buyerStages } = useBuyerStages(buyerId, draft.counterparty_pipeline_id || null);

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
          <Label>Kind</Label>
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
                <Label>Trigger stage</Label>
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

        <SectionLabel className="mt-1">Delivery</SectionLabel>
        <p className="text-xs text-gray-400">Default for this contract. Routes can override Lead vs Pipeline.</p>
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
        {draft.delivery === "leads_pipeline" && (
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
        )}
      </div>
    </div>
  );
}

export function CreateContractCompensationList({
  buyerId,
  items,
  onChange,
}: {
  buyerId: number;
  items: CompensationDraft[];
  onChange: (items: CompensationDraft[]) => void;
}) {
  return (
    <div className="flex flex-col gap-3">
      {items.map((d, i) => (
        <CompensationCard
          key={i}
          index={i}
          draft={d}
          buyerId={buyerId}
          canRemove={items.length > 1}
          onChange={(next) => onChange(items.map((x, j) => (j === i ? next : x)))}
          onRemove={() => onChange(items.filter((_, j) => j !== i))}
        />
      ))}
      <Button
        variant="secondary"
        onClick={() =>
          onChange([
            ...items,
            emptyCompensationDraft({
              source_pipeline_id: items[0]?.source_pipeline_id ?? 0,
              source_stage_id: items[0]?.source_stage_id ?? 0,
              counterparty_pipeline_id: items[0]?.counterparty_pipeline_id ?? 0,
              return_stage_id: items[0]?.return_stage_id ?? 0,
            }),
          ])
        }
      >
        Add compensation
      </Button>
    </div>
  );
}
