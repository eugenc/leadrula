import { cn } from "@/lib/utils";
import { type ButtonHTMLAttributes, forwardRef } from "react";

export const IconButton = forwardRef<
  HTMLButtonElement,
  ButtonHTMLAttributes<HTMLButtonElement> & { variant?: "ghost" | "danger" }
>(({ className, variant = "ghost", ...props }, ref) => (
  <button
    ref={ref}
    type="button"
    className={cn(
      "inline-flex h-8 w-8 items-center justify-center rounded-md transition-colors",
      variant === "ghost" && "text-gray-400 hover:bg-gray-100 hover:text-gray-700",
      variant === "danger" && "text-gray-400 hover:bg-gray-100 hover:text-danger",
      className
    )}
    {...props}
  />
));
IconButton.displayName = "IconButton";
