import { cn } from "@/lib/utils";
import {
  type InputHTMLAttributes,
  type TextareaHTMLAttributes,
  type SelectHTMLAttributes,
  forwardRef,
} from "react";

const base =
  "w-full rounded-md border border-gray-200 bg-surface-card text-md text-gray-800 outline-none transition-[border-color,box-shadow] placeholder:text-gray-300 hover:border-gray-300 focus:border-jade-500 focus:ring-[3px] focus:ring-jade-500/12 disabled:cursor-not-allowed disabled:bg-gray-50 disabled:text-gray-400";

export const Input = forwardRef<HTMLInputElement, InputHTMLAttributes<HTMLInputElement>>(
  ({ className, ...props }, ref) => (
    <input ref={ref} className={cn(base, "h-9 px-3", className)} {...props} />
  )
);
Input.displayName = "Input";

export const Textarea = forwardRef<
  HTMLTextAreaElement,
  TextareaHTMLAttributes<HTMLTextAreaElement>
>(({ className, ...props }, ref) => (
  <textarea ref={ref} className={cn(base, "min-h-[80px] px-3 py-2.5", className)} {...props} />
));
Textarea.displayName = "Textarea";

export const Select = forwardRef<HTMLSelectElement, SelectHTMLAttributes<HTMLSelectElement>>(
  ({ className, children, ...props }, ref) => (
    <select ref={ref} className={cn(base, "h-9 px-3", className)} {...props}>
      {children}
    </select>
  )
);
Select.displayName = "Select";

export const FilterSelect = forwardRef<
  HTMLSelectElement,
  SelectHTMLAttributes<HTMLSelectElement>
>(({ className, children, ...props }, ref) => (
  <select
    ref={ref}
    className={cn(base, "h-8 min-w-[160px] cursor-pointer px-3 text-sm text-gray-700", className)}
    {...props}
  >
    {children}
  </select>
));
FilterSelect.displayName = "FilterSelect";

export function Label({ children, className }: { children: React.ReactNode; className?: string }) {
  return (
    <label className={cn("mb-1.5 block text-md font-medium text-gray-700", className)}>
      {children}
    </label>
  );
}
