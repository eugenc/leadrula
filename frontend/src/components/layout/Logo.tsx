import { cn } from "@/lib/utils";

export function Logo({ className = "h-8 w-auto" }: { className?: string }) {
  return (
    <img
      src="/leadrula-logo-light.png"
      alt="LeadRula"
      className={cn(className, "dark:brightness-0 dark:invert")}
    />
  );
}
