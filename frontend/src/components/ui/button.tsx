import { cn } from "@/lib/utils";
import { type ButtonHTMLAttributes, forwardRef } from "react";

type Variant = "primary" | "secondary" | "ghost" | "danger" | "outline";
type Size = "sm" | "md" | "lg" | "icon";

interface Props extends ButtonHTMLAttributes<HTMLButtonElement> {
  variant?: Variant;
  size?: Size;
}

const variants: Record<Variant, string> = {
  primary:
    "bg-jade-500 text-white hover:bg-jade-600 active:bg-jade-700 disabled:opacity-40 focus-visible:outline focus-visible:outline-2 focus-visible:outline-jade-400 focus-visible:outline-offset-2",
  secondary:
    "border border-gray-200 bg-surface-card text-gray-700 hover:bg-gray-100 hover:border-gray-300 active:bg-gray-100",
  ghost: "bg-transparent text-gray-400 hover:bg-gray-100 hover:text-gray-700",
  danger: "bg-danger text-white hover:opacity-90",
  outline: "border border-gray-200 bg-surface-card text-gray-700 hover:bg-gray-100",
};

const sizes: Record<Size, string> = {
  sm: "h-7 px-2.5 text-sm",
  md: "h-8 px-3 text-sm",
  lg: "h-10 px-5 text-md",
  icon: "h-8 w-8 p-0",
};

export const Button = forwardRef<HTMLButtonElement, Props>(
  ({ className, variant = "primary", size = "md", ...props }, ref) => (
    <button
      ref={ref}
      className={cn(
        "inline-flex items-center justify-center gap-2 rounded-md font-semibold transition-colors disabled:cursor-not-allowed disabled:pointer-events-none",
        variants[variant],
        sizes[size],
        className
      )}
      {...props}
    />
  )
);
Button.displayName = "Button";
