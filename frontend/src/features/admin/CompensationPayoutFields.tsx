import { Label, Select } from "@/components/ui/input";
import { SectionLabel } from "@/components/layout/SectionLabel";
import { PAYOUT_FREQUENCIES, PAYOUT_MONTH_DAYS, PAYOUT_WEEKDAYS } from "@/features/admin/contractCompensation";

export type PayoutDraftFields = {
  payout_frequency: string;
  payout_weekday: number;
  payout_month_day: number;
};

export function defaultPayoutDraft(): PayoutDraftFields {
  return { payout_frequency: "weekly", payout_weekday: 1, payout_month_day: 1 };
}

export function payoutDraftFromComp(c: {
  payout_frequency?: string | null;
  payout_weekday?: number | null;
  payout_month_day?: number | null;
}): PayoutDraftFields {
  return {
    payout_frequency: c.payout_frequency ?? "weekly",
    payout_weekday: c.payout_weekday ?? 1,
    payout_month_day: c.payout_month_day ?? 1,
  };
}

export function CompensationPayoutFields({
  draft,
  onChange,
}: {
  draft: PayoutDraftFields;
  onChange: (d: PayoutDraftFields) => void;
}) {
  const set = <K extends keyof PayoutDraftFields>(k: K, v: PayoutDraftFields[K]) =>
    onChange({ ...draft, [k]: v });

  return (
    <>
      <SectionLabel className="mt-2">Payouts</SectionLabel>
      <p className="text-xs text-gray-400">When earned amounts move from hold to cleared for this row.</p>
      <div>
        <Label>Payout frequency</Label>
        <Select value={draft.payout_frequency} onChange={(e) => set("payout_frequency", e.target.value)}>
          <option value="">Select…</option>
          {PAYOUT_FREQUENCIES.map((f) => (
            <option key={f.value} value={f.value}>
              {f.label}
            </option>
          ))}
        </Select>
      </div>
      {draft.payout_frequency === "weekly" && (
        <div>
          <Label>Day of week</Label>
          <Select
            value={draft.payout_weekday}
            onChange={(e) => set("payout_weekday", Number(e.target.value))}
          >
            {PAYOUT_WEEKDAYS.map((d) => (
              <option key={d.value} value={d.value}>
                {d.label}
              </option>
            ))}
          </Select>
        </div>
      )}
      {draft.payout_frequency === "monthly" && (
        <div>
          <Label>Day of month</Label>
          <Select
            value={draft.payout_month_day}
            onChange={(e) => set("payout_month_day", Number(e.target.value))}
          >
            {PAYOUT_MONTH_DAYS.map((d) => (
              <option key={d.value} value={d.value}>
                {d.label}
              </option>
            ))}
          </Select>
        </div>
      )}
    </>
  );
}
