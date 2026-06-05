import { cn } from "@/lib/utils";
import type { AccountOperationalStatus } from "@/types";

export function PlatformAccountStatusBadge({ value }: { value: AccountOperationalStatus }) {
  const suspended = value === "suspended";
  return (
    <span
      className={cn(
        "rounded-full px-2 py-0.5 text-xs font-medium",
        suspended
          ? "border border-neutral-border bg-neutral-bg text-neutral-fg"
          : "border border-success-border bg-success-bg text-success-fg"
      )}
    >
      {suspended ? "Suspended" : "Active"}
    </span>
  );
}
