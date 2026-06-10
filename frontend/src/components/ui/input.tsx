import { cn } from "@/lib/utils";
import {
  type InputHTMLAttributes,
  type TextareaHTMLAttributes,
  type SelectHTMLAttributes,
  forwardRef,
  useLayoutEffect,
  useRef,
  useState,
} from "react";

const overflowTooltipClass =
  "pointer-events-none absolute left-0 top-full z-20 mt-1.5 max-w-md rounded-md bg-[#101828] px-2 py-1.5 text-xs font-normal leading-snug text-[#F9FAFB] opacity-0 shadow-sm transition-opacity duration-150 group-hover/overflow:opacity-100 whitespace-normal break-words";

const base =
  "w-full rounded-md border border-gray-200 bg-surface-card text-md text-gray-800 outline-none transition-[border-color,box-shadow] placeholder:text-gray-300 hover:border-gray-300 focus:border-jade-500 focus:ring-[3px] focus:ring-jade-500/12 disabled:cursor-not-allowed disabled:bg-gray-50 disabled:text-gray-400";

export const Input = forwardRef<HTMLInputElement, InputHTMLAttributes<HTMLInputElement>>(
  ({ className, ...props }, ref) => (
    <input ref={ref} className={cn(base, "h-9 px-3", className)} {...props} />
  )
);
Input.displayName = "Input";

export const InputWithOverflowTooltip = forwardRef<
  HTMLInputElement,
  InputHTMLAttributes<HTMLInputElement>
>(({ className, value, ...props }, ref) => {
  const inputRef = useRef<HTMLInputElement>(null);
  const [overflow, setOverflow] = useState(false);
  const displayText = value == null ? "" : String(value);

  useLayoutEffect(() => {
    const el = inputRef.current;
    if (!el) return;
    const check = () => setOverflow(el.scrollWidth > el.clientWidth);
    check();
    const ro = new ResizeObserver(check);
    ro.observe(el);
    return () => ro.disconnect();
  }, [displayText]);

  function setRefs(node: HTMLInputElement | null) {
    inputRef.current = node;
    if (typeof ref === "function") ref(node);
    else if (ref) ref.current = node;
  }

  return (
    <span className="group/overflow relative block w-full">
      <Input
        ref={setRefs}
        value={value}
        className={cn(overflow && "truncate", className)}
        {...props}
      />
      {overflow && displayText && (
        <span role="tooltip" className={overflowTooltipClass}>
          {displayText}
        </span>
      )}
    </span>
  );
});
InputWithOverflowTooltip.displayName = "InputWithOverflowTooltip";

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

export const FilterInput = forwardRef<HTMLInputElement, InputHTMLAttributes<HTMLInputElement>>(
  ({ className, ...props }, ref) => (
    <input
      ref={ref}
      className={cn(base, "h-7 px-2.5 text-sm text-gray-700", className)}
      {...props}
    />
  )
);
FilterInput.displayName = "FilterInput";

export function Label({ children, className }: { children: React.ReactNode; className?: string }) {
  return (
    <label className={cn("mb-1.5 block text-md font-medium text-gray-700", className)}>
      {children}
    </label>
  );
}
