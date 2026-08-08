import { formatDatetimeForDisplay } from "./customFieldDate";

export type PendingReturnDisplayVariant = "card" | "detail";

function isValidTimezone(tz: string): boolean {
  try {
    Intl.DateTimeFormat(undefined, { timeZone: tz });
    return true;
  } catch {
    return false;
  }
}

export function formatPendingReturnLabel(
  iso: string | null | undefined,
  timezone: string | null | undefined,
  variant: PendingReturnDisplayVariant
): string | null {
  if (!iso) return null;
  const date = new Date(iso);
  if (Number.isNaN(date.getTime())) return null;

  const prefix = "Returns ";
  const options: Intl.DateTimeFormatOptions =
    variant === "card"
      ? { weekday: "short", hour: "numeric", minute: "2-digit" }
      : { weekday: "short", month: "short", day: "numeric", hour: "numeric", minute: "2-digit" };

  if (timezone && isValidTimezone(timezone)) {
    try {
      const formatted = new Intl.DateTimeFormat("en-US", { ...options, timeZone: timezone }).format(date);
      return `${prefix}${formatted}`;
    } catch {
      // fall through
    }
  }

  const fallback = formatDatetimeForDisplay(iso);
  if (fallback === "—") return null;
  return `${prefix}${fallback}`;
}
