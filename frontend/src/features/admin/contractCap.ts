export const CONTRACT_CAP_PERIODS = [
  { value: "one_time", label: "Lifetime" },
  { value: "weekly", label: "Weekly" },
  { value: "monthly", label: "Monthly" },
] as const;

export type ContractCapPeriod = (typeof CONTRACT_CAP_PERIODS)[number]["value"];

const LABELS = Object.fromEntries(CONTRACT_CAP_PERIODS.map((p) => [p.value, p.label]));

export function formatCapPeriod(period: string | undefined): string {
  if (!period) return "";
  return LABELS[period] ?? period;
}

export function isContractCapPeriod(value: string): value is ContractCapPeriod {
  return value === "one_time" || value === "weekly" || value === "monthly";
}

export function capPeriodShowsDailyCap(period: string | undefined): boolean {
  return period === "weekly" || period === "monthly";
}

export function formatContractCap(c: {
  cap_period?: string;
  cap_total?: number | null;
  cap_max_daily?: number | null;
}): string {
  const period = formatCapPeriod(c.cap_period);
  if (c.cap_total == null) {
    return period ? period : "—";
  }
  const daily = capPeriodShowsDailyCap(c.cap_period) && c.cap_max_daily != null
    ? ` · Max daily: ${c.cap_max_daily}`
    : "";
  const prefix = period ? `${period} · ` : "";
  return `${prefix}Total: ${c.cap_total}${daily}`;
}

export function capInputValue(n: number | null | undefined): string {
  return n == null ? "" : String(n);
}

export function parseCapInput(raw: string): number | null {
  const t = raw.trim();
  if (t === "") return null;
  const n = Number(t);
  if (!Number.isFinite(n) || n <= 0 || !Number.isInteger(n)) return null;
  return n;
}
