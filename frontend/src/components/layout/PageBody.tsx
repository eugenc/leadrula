import { cn } from "@/lib/utils";

export function PageBody({
  children,
  className,
}: {
  children: React.ReactNode;
  className?: string;
}) {
  return <div className={cn("flex-1 px-8 pb-8 pt-5", className)}>{children}</div>;
}
