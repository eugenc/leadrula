import { cn } from "@/lib/utils";
import { useEffect, useRef, type ReactNode } from "react";

export function Dropdown({
  open,
  onClose,
  trigger,
  children,
  className,
  align = "right",
}: {
  open: boolean;
  onClose: () => void;
  trigger: ReactNode;
  children: ReactNode;
  className?: string;
  align?: "left" | "right";
}) {
  const ref = useRef<HTMLDivElement>(null);

  useEffect(() => {
    if (!open) return;
    function handleClick(e: MouseEvent) {
      if (ref.current && !ref.current.contains(e.target as Node)) onClose();
    }
    document.addEventListener("mousedown", handleClick);
    return () => document.removeEventListener("mousedown", handleClick);
  }, [open, onClose]);

  return (
    <div ref={ref} className="relative">
      {trigger}
      {open && (
        <div
          className={cn(
            "absolute top-full z-50 mt-1 min-w-[200px] rounded-lg border border-gray-100 bg-surface-card p-1 shadow-md",
            align === "right" ? "right-0" : "left-0",
            className
          )}
        >
          {children}
        </div>
      )}
    </div>
  );
}

export function DropdownItem({
  children,
  onClick,
  selected,
  className,
}: {
  children: ReactNode;
  onClick?: () => void;
  selected?: boolean;
  className?: string;
}) {
  return (
    <button
      type="button"
      onClick={onClick}
      className={cn(
        "flex h-9 w-full items-center rounded-md px-2.5 text-left text-base text-gray-700 hover:bg-jade-50 hover:text-jade-700",
        selected && "bg-jade-100 font-medium text-jade-700",
        className
      )}
    >
      {children}
    </button>
  );
}

export function DropdownSearch({
  value,
  onChange,
  placeholder = "Search…",
  autoFocus,
}: {
  value: string;
  onChange: (v: string) => void;
  placeholder?: string;
  autoFocus?: boolean;
}) {
  return (
    <div className="mb-1 border-b border-gray-100 px-2 py-1.5">
      <input
        value={value}
        onChange={(e) => onChange(e.target.value)}
        placeholder={placeholder}
        autoFocus={autoFocus}
        className="w-full border-none bg-transparent text-base text-gray-800 outline-none placeholder:text-gray-300"
      />
    </div>
  );
}
