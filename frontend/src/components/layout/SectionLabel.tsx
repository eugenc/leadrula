import { cn } from "@/lib/utils";

export function SectionLabel({ children, className }: { children: React.ReactNode; className?: string }) {
  return (
    <div
      className={cn(
        "text-xs font-semibold uppercase tracking-wide text-gray-400",
        className
      )}
    >
      {children}
    </div>
  );
}
