import { useState } from "react";
import { cn, initials } from "@/lib/utils";
import { Loader2 } from "lucide-react";

export function Card({ className, children }: { className?: string; children: React.ReactNode }) {
  return (
    <div className={cn("rounded-lg border border-gray-100 bg-surface-card shadow-xs", className)}>
      {children}
    </div>
  );
}

type BadgeVariant =
  | "default"
  | "review"
  | "distributed"
  | "returned"
  | "closed"
  | "overdue"
  | "pending";

export function Badge({
  children,
  variant = "default",
  className,
}: {
  children: React.ReactNode;
  variant?: BadgeVariant;
  className?: string;
}) {
  const styles: Record<BadgeVariant, string> = {
    default: "border border-neutral-border bg-neutral-bg text-neutral-fg",
    review: "border border-info-border bg-info-bg text-info-fg",
    distributed: "border border-success-border bg-success-bg text-success-fg",
    returned: "border border-neutral-border bg-neutral-bg text-neutral-fg",
    closed: "border border-neutral-border bg-neutral-bg text-neutral-fg",
    overdue: "border border-danger-border bg-danger-bg text-danger-fg",
    pending: "border border-warning-border bg-warning-bg text-warning-fg",
  };
  return (
    <span
      className={cn(
        "inline-flex items-center whitespace-nowrap rounded-full px-2 py-0.5 text-xs font-semibold",
        styles[variant],
        className
      )}
    >
      {children}
    </span>
  );
}

export function Avatar({
  name,
  src,
  className,
  variant = "default",
}: {
  name: string;
  src?: string | null;
  className?: string;
  variant?: "default" | "card";
}) {
  const [failed, setFailed] = useState(false);
  const base = cn(
    "flex shrink-0 items-center justify-center overflow-hidden rounded-full text-sm font-semibold",
    variant === "card" ? "h-6 w-6 text-[10px]" : "h-8 w-8"
  );

  if (src && !failed) {
    return (
      <img
        src={src}
        alt={name}
        title={name}
        onError={() => setFailed(true)}
        className={cn(base, "object-cover", className)}
      />
    );
  }

  return (
    <div
      title={name}
      className={cn(
        base,
        variant === "card" ? "bg-jade-100 text-jade-700" : "bg-jade-500 text-white",
        className
      )}
    >
      {initials(name) || "?"}
    </div>
  );
}

export function Spinner({ className }: { className?: string }) {
  return <Loader2 className={cn("h-4 w-4 animate-spin text-gray-400", className)} />;
}

export function Switch({
  checked,
  onChange,
  disabled = false,
}: {
  checked: boolean;
  onChange: (v: boolean) => void;
  disabled?: boolean;
}) {
  return (
    <button
      type="button"
      role="switch"
      aria-checked={checked}
      disabled={disabled}
      onClick={() => onChange(!checked)}
      className={cn(
        "relative inline-flex h-5 w-9 items-center rounded-full transition-colors",
        checked ? "bg-jade-500" : "bg-gray-200",
        disabled && "cursor-not-allowed opacity-50"
      )}
    >
      <span
        className={cn(
          "inline-block h-4 w-4 transform rounded-full bg-[#FFFFFF] shadow-xs transition-transform",
          checked ? "translate-x-4" : "translate-x-0.5"
        )}
      />
    </button>
  );
}

export function EmptyState({
  title,
  subtitle,
  action,
}: {
  title: string;
  subtitle?: string;
  action?: React.ReactNode;
}) {
  return (
    <div className="flex flex-col items-center justify-center gap-3 py-16 text-center text-gray-400">
      <p className="text-sm">{title}</p>
      {subtitle && <p className="text-xs">{subtitle}</p>}
      {action}
    </div>
  );
}

export function StatCard({ label, value }: { label: string; value: React.ReactNode }) {
  return (
    <Card className="p-4">
      <div className="text-xs font-semibold uppercase tracking-wide text-gray-400">{label}</div>
      <div className="mt-1 text-2xl font-bold text-gray-800">{value}</div>
    </Card>
  );
}
