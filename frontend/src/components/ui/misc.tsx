import { cn, initials } from "@/lib/utils";
import { Loader2 } from "lucide-react";

export function Card({ className, children }: { className?: string; children: React.ReactNode }) {
  return (
    <div className={cn("rounded border border-pd-border bg-pd-surface", className)}>{children}</div>
  );
}

export function Badge({
  children,
  variant = "default",
  className,
}: {
  children: React.ReactNode;
  variant?: "default" | "green" | "amber" | "red" | "blue" | "muted";
  className?: string;
}) {
  const styles: Record<string, string> = {
    default: "bg-pd-stage text-pd-text",
    green: "bg-pd-green/10 text-pd-green",
    amber: "bg-pd-amber/15 text-pd-amber",
    red: "bg-pd-red/10 text-pd-red",
    blue: "bg-pd-blue/10 text-pd-blue",
    muted: "bg-pd-stage text-pd-muted",
  };
  return (
    <span
      className={cn(
        "inline-flex items-center rounded px-2 py-0.5 text-xs font-semibold",
        styles[variant],
        className
      )}
    >
      {children}
    </span>
  );
}

export function Avatar({ name, className }: { name: string; className?: string }) {
  return (
    <div
      title={name}
      className={cn(
        "flex h-7 w-7 items-center justify-center rounded-full bg-pd-blue/15 text-xs font-semibold text-pd-blue",
        className
      )}
    >
      {initials(name) || "?"}
    </div>
  );
}

export function Spinner({ className }: { className?: string }) {
  return <Loader2 className={cn("h-4 w-4 animate-spin text-pd-muted", className)} />;
}

export function Switch({
  checked,
  onChange,
}: {
  checked: boolean;
  onChange: (v: boolean) => void;
}) {
  return (
    <button
      type="button"
      onClick={() => onChange(!checked)}
      className={cn(
        "relative inline-flex h-5 w-9 items-center rounded-full transition-colors",
        checked ? "bg-pd-green" : "bg-pd-border"
      )}
    >
      <span
        className={cn(
          "inline-block h-4 w-4 transform rounded-full bg-white transition-transform",
          checked ? "translate-x-4" : "translate-x-0.5"
        )}
      />
    </button>
  );
}

export function EmptyState({ title, action }: { title: string; action?: React.ReactNode }) {
  return (
    <div className="flex flex-col items-center justify-center gap-3 py-16 text-center text-pd-muted">
      <p className="text-sm">{title}</p>
      {action}
    </div>
  );
}
