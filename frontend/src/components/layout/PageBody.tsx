import { cn } from "@/lib/utils";

export function PageBody({
  children,
  className,
}: {
  children: React.ReactNode;
  className?: string;
}) {
  return <div className={cn("flex-1 px-4 pb-8 pt-5 sm:px-6 lg:px-8", className)}>{children}</div>;
}
