import { cn } from "@/lib/utils";

export function PageHeader({
  title,
  subtitle,
  action,
  className,
}: {
  title?: string;
  subtitle?: string;
  action?: React.ReactNode;
  className?: string;
}) {
  return (
    <div className={cn("flex items-start justify-between px-8 pt-6", className)}>
      <div>
        {title && <h2 className="text-xl font-semibold text-gray-800">{title}</h2>}
        {subtitle && (
          <p className={cn(title ? "mt-0.5" : "", "text-base text-gray-400")}>{subtitle}</p>
        )}
      </div>
      {action && <div className="shrink-0">{action}</div>}
    </div>
  );
}
