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

export function inputModeForFormat(type: string, formatToken: string): "date" | "datetime-local" | "text" {
  if (type === "date" && formatToken === "yyyy-MM-DD") return "date";
  if (type === "datetime" && formatToken === "yyyy-MM-DDTHH:mm") return "datetime-local";
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
const DISPLAY_DATETIME = "MMM d, h:mma";

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
