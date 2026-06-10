import { cn } from "@/lib/utils";

export function Table({ children, className }: { children: React.ReactNode; className?: string }) {
  return (
    <div
      className={cn(
        "min-w-0 overflow-x-auto rounded-lg border border-gray-100 bg-surface-card",
        className
      )}
    >
      <table className="min-w-full w-max border-collapse">{children}</table>
    </div>
  );
}

export function THead({ children }: { children: React.ReactNode }) {
  return (
    <thead className="h-10 border-b border-gray-100 bg-gray-100 text-left">
      {children}
    </thead>
  );
}

export function TH({ children, className }: { children?: React.ReactNode; className?: string }) {
  return (
    <th
      className={cn(
        "min-w-[7.5rem] whitespace-nowrap px-4 text-xs font-semibold uppercase tracking-wide text-gray-500",
        className
      )}
    >
      {children}
    </th>
  );
}

export function TBody({ children }: { children: React.ReactNode }) {
  return <tbody>{children}</tbody>;
}

export function TR({
  children,
  onClick,
  className,
}: {
  children: React.ReactNode;
  onClick?: () => void;
  className?: string;
}) {
  return (
    <tr
      onClick={onClick}
      className={cn(
        "h-11 border-b border-gray-100 transition-colors last:border-0",
        onClick && "cursor-pointer hover:bg-gray-100",
        className
      )}
    >
      {children}
    </tr>
  );
}

export function TD({ children, className }: { children?: React.ReactNode; className?: string }) {
  return (
    <td className={cn("min-w-[7.5rem] px-4 align-middle text-sm text-gray-700", className)}>
      {children}
    </td>
  );
}
