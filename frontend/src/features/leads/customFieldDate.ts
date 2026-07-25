import { format, isValid, parse } from "date-fns";
import { effectiveFieldFormat } from "@/features/admin/customFieldConstants";

const DATE_FNS_PATTERNS: Record<string, string> = {
  "yyyy-MM-DD": "yyyy-MM-dd",
  "MM/dd/yyyy": "MM/dd/yyyy",
  "dd/MM/yyyy": "dd/MM/yyyy",
  "MMM d, yyyy": "MMM d, yyyy",
  "yyyy-MM-DDTHH:mm": "yyyy-MM-dd'T'HH:mm",
  "yyyy-MM-DD HH:mm": "yyyy-MM-dd HH:mm",
  RFC3339: "yyyy-MM-dd'T'HH:mm:ssXXX",
  "MM/dd/yyyy h:mm a": "MM/dd/yyyy h:mm a",
};

const FALLBACK_PATTERNS = [
  "yyyy-MM-dd'T'HH:mm:ssXXX",
  "yyyy-MM-dd'T'HH:mm:ss",
  "yyyy-MM-dd'T'HH:mm",
  "yyyy-MM-dd HH:mm",
  "yyyy-MM-dd",
  "MM/dd/yyyy h:mm a",
  "MM/dd/yyyy",
  "dd/MM/yyyy",
  "MMM d, yyyy",
];

function toDateFnsPattern(token: string): string {
  return DATE_FNS_PATTERNS[token] ?? token;
}

export function inputModeForFormat(type: string, _formatToken: string): "date" | "datetime-local" | "text" {
  if (type === "date") return "date";
  if (type === "datetime") return "datetime-local";
  return "text";
}

export function parseCustomDate(value: string, formatToken: string): Date | null {
  const trimmed = value.trim();
  if (!trimmed) return null;

  const patterns = [toDateFnsPattern(formatToken), ...FALLBACK_PATTERNS];
  const seen = new Set<string>();
  for (const pattern of patterns) {
    if (seen.has(pattern)) continue;
    seen.add(pattern);
    const d = parse(trimmed, pattern, new Date());
    if (isValid(d)) return d;
  }

  const iso = new Date(trimmed);
  if (isValid(iso)) return iso;
  return null;
}

export function formatCustomDate(date: Date, formatToken: string): string {
  return format(date, toDateFnsPattern(formatToken));
}

const DISPLAY_DATE = "MMM d, yyyy";
const DISPLAY_DATETIME = "EEE. MMM d, h:mm a";

export function formatDatetimeForDisplay(value: string | Date | null | undefined): string {
  if (!value) return "—";
  const d = typeof value === "string" ? new Date(value) : value;
  if (!isValid(d)) return "—";
  return format(d, DISPLAY_DATETIME);
}

export function formatCustomDateForDisplay(
  value: string,
  type: string,
  fieldFormat?: string | null
): string {
  const trimmed = value.trim();
  if (!trimmed) return "—";
  const d = parseCustomDate(trimmed, effectiveFieldFormat(type, fieldFormat));
  if (!d) return trimmed;
  return format(d, type === "datetime" ? DISPLAY_DATETIME : DISPLAY_DATE);
}

export function normalizeCustomDateValue(value: string, type: string, fieldFormat?: string | null): string {
  const trimmed = value.trim();
  if (!trimmed) return "";
  const formatToken = effectiveFieldFormat(type, fieldFormat);
  const d = parseCustomDate(trimmed, formatToken);
  if (!d) return trimmed;
  return formatCustomDate(d, formatToken);
}

export function toNativeDateValue(value: string, type: string, fieldFormat?: string | null): string {
  const d = parseCustomDate(value, effectiveFieldFormat(type, fieldFormat));
  if (!d) return value;
  const p = (n: number) => String(n).padStart(2, "0");
  return `${d.getFullYear()}-${p(d.getMonth() + 1)}-${p(d.getDate())}`;
}

export function toNativeDatetimeLocalValue(value: string, type: string, fieldFormat?: string | null): string {
  const d = parseCustomDate(value, effectiveFieldFormat(type, fieldFormat));
  if (!d) return value;
  const p = (n: number) => String(n).padStart(2, "0");
  return `${d.getFullYear()}-${p(d.getMonth() + 1)}-${p(d.getDate())}T${p(d.getHours())}:${p(d.getMinutes())}`;
}

export function fromNativeDatetimeLocal(value: string, type: string, fieldFormat?: string | null): string {
  const d = parse(value, "yyyy-MM-dd'T'HH:mm", new Date());
  if (!isValid(d)) return value;
  return formatCustomDate(d, effectiveFieldFormat(type, fieldFormat));
}

export const QUARTER_MINUTES = [0, 15, 30, 45] as const;

export type DatetimeLocalParts = {
  date: string;
  hour12: number;
  minute: number;
  period: "AM" | "PM";
};

const pad2 = (n: number) => String(n).padStart(2, "0");

function parseDatetimeLocalString(value: string): Date | null {
  const trimmed = value.trim();
  if (!trimmed) return null;
  const d = parse(trimmed, "yyyy-MM-dd'T'HH:mm", new Date());
  if (isValid(d)) return d;
  const iso = new Date(trimmed);
  return isValid(iso) ? iso : null;
}

export function snapDatetimeLocalToQuarter(value: string): string {
  const d = parseDatetimeLocalString(value);
  if (!d) return value;
  const total = d.getHours() * 60 + d.getMinutes();
  const snapped = Math.round(total / 15) * 15;
  const hours = Math.floor(snapped / 60) % 24;
  const minutes = snapped % 60;
  return `${d.getFullYear()}-${pad2(d.getMonth() + 1)}-${pad2(d.getDate())}T${pad2(hours)}:${pad2(minutes)}`;
}

export function parseDatetimeLocalParts(value: string): DatetimeLocalParts | null {
  const d = parseDatetimeLocalString(value);
  if (!d) return null;
  const hours24 = d.getHours();
  const period: "AM" | "PM" = hours24 >= 12 ? "PM" : "AM";
  const hour12 = hours24 % 12 || 12;
  const minute = QUARTER_MINUTES.reduce((best, m) =>
    Math.abs(m - d.getMinutes()) < Math.abs(best - d.getMinutes()) ? m : best
  );
  return {
    date: `${d.getFullYear()}-${pad2(d.getMonth() + 1)}-${pad2(d.getDate())}`,
    hour12,
    minute,
    period,
  };
}

export function buildDatetimeLocal(parts: DatetimeLocalParts): string {
  let hours24 = parts.hour12 % 12;
  if (parts.period === "PM") hours24 += 12;
  return `${parts.date}T${pad2(hours24)}:${pad2(parts.minute)}`;
}

export function defaultDatetimeLocalParts(): DatetimeLocalParts {
  const now = new Date();
  const snapped = snapDatetimeLocalToQuarter(
    `${now.getFullYear()}-${pad2(now.getMonth() + 1)}-${pad2(now.getDate())}T${pad2(now.getHours())}:${pad2(now.getMinutes())}`
  );
  return parseDatetimeLocalParts(snapped)!;
}

export function isoToDatetimeLocal(iso: string): string {
  const d = new Date(iso);
  return `${d.getFullYear()}-${pad2(d.getMonth() + 1)}-${pad2(d.getDate())}T${pad2(d.getHours())}:${pad2(d.getMinutes())}`;
}
