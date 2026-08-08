import { RotateCcw } from "lucide-react";
import { cn } from "@/lib/utils";
import { formatPendingReturnLabel } from "./returnScheduleDisplay";

export function ReturnIndicator({
  pendingReturnAt,
  pendingReturnTimezone,
  variant = "card",
  className,
}: {
  pendingReturnAt?: string | null;
  pendingReturnTimezone?: string | null;
  variant?: "card" | "detail";
  className?: string;
}) {
  const label = formatPendingReturnLabel(
    pendingReturnAt,
    pendingReturnTimezone,
    variant === "detail" ? "detail" : "card"
  );
  if (!label) return null;

  const tooltip =
    pendingReturnTimezone && pendingReturnAt
      ? `${pendingReturnAt} (${pendingReturnTimezone})`
      : pendingReturnAt ?? undefined;

  if (variant === "detail") {
    return (
      <div
        className={cn(
          "mt-3 flex flex-nowrap items-center gap-1.5 rounded-md border border-amber-200 bg-amber-50 px-2.5 py-1.5",
          className
        )}
        title={tooltip}
      >
        <RotateCcw className="h-3.5 w-3.5 shrink-0 text-amber-700" aria-hidden />
        <span className="text-xs font-medium text-amber-900">{label}</span>
      </div>
    );
  }

  return (
    <span
      className={cn(
        "inline-flex max-w-[9rem] items-center gap-0.5 truncate rounded-full bg-amber-50 px-1.5 py-0.5 text-[10px] font-medium leading-tight text-amber-800",
        className
      )}
      title={tooltip ?? label}
    >
      <RotateCcw className="h-2.5 w-2.5 shrink-0" aria-hidden />
      <span className="truncate">{label}</span>
    </span>
  );
}
