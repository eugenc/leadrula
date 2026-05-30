import { ChevronLeft, ChevronRight } from "lucide-react";
import { cn } from "@/lib/utils";
import { isPast } from "date-fns";

type ActionState = "none" | "upcoming" | "overdue";

function actionState(actionAt?: string | null): ActionState {
  if (!actionAt) return "none";
  return isPast(new Date(actionAt)) ? "overdue" : "upcoming";
}

function actionTitle(state: ActionState): string {
  if (state === "overdue") return "Overdue action";
  if (state === "upcoming") return "Upcoming action";
  return "No action set";
}

export function ActionIndicator({
  actionAt,
  variant = "badge",
  size = "md",
  className,
}: {
  actionAt?: string | null;
  variant?: "badge" | "dot";
  size?: "sm" | "md";
  className?: string;
}) {
  const state = actionState(actionAt);
  const title = actionTitle(state);

  if (variant === "dot") {
    return (
      <span
        className={cn(
          "inline-block h-2.5 w-2.5 shrink-0 rounded-full",
          state === "upcoming" && "bg-jade-500",
          state === "overdue" && "bg-red-500",
          state === "none" && "bg-orange-500",
          className
        )}
        title={title}
      />
    );
  }

  const badgeSize = size === "sm" ? "h-3.5 w-3.5" : "h-5 w-5";
  const iconSize = size === "sm" ? "h-2 w-2" : "h-3 w-3";

  return (
    <span
      className={cn(
        "inline-flex shrink-0 items-center justify-center rounded-full",
        badgeSize,
        state === "none" && "bg-orange-500",
        state === "upcoming" && "bg-jade-500",
        state === "overdue" && "bg-red-500",
        className
      )}
      title={title}
    >
      {state === "upcoming" && <ChevronRight className={cn(iconSize, "text-white")} strokeWidth={3} />}
      {state === "overdue" && <ChevronLeft className={cn(iconSize, "text-white")} strokeWidth={3} />}
    </span>
  );
}

/** @deprecated Use ActionIndicator */
export const ActionDot = ActionIndicator;
