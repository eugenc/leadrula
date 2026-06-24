import { useEffect, useRef, useState } from "react";
import { X } from "lucide-react";
import { Input } from "@/components/ui/input";
import { useCalls, useBuyerCalls, type CallLogFilters } from "./hooks";
import type { Call } from "@/types";

function callPrimary(call: Call): string {
  return (
    call.lead_name ||
    call.caller_number ||
    call.publisher_name ||
    call.contract_name ||
    `Call #${call.public_id}`
  );
}

function callSecondary(call: Call, role: "publisher" | "buyer"): string {
  const parts =
    role === "publisher"
      ? [call.caller_number, call.winner_buyer_name, call.contract_name]
      : [call.lead_phone, call.publisher_name, call.contract_name, call.lead_name];
  return parts.filter(Boolean).join(" · ");
}

export function CallSearchInput({
  value,
  onChange,
  role,
  callFilters,
  onSelectCall,
  onClear,
  placeholder,
  className,
  inputClassName,
}: {
  value: string;
  onChange: (value: string) => void;
  role: "publisher" | "buyer";
  callFilters?: Omit<CallLogFilters, "q" | "limit">;
  onSelectCall?: (call: Call) => void;
  onClear?: () => void;
  placeholder?: string;
  className?: string;
  inputClassName?: string;
}) {
  const [open, setOpen] = useState(false);
  const [debouncedQuery, setDebouncedQuery] = useState("");
  const containerRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    const t = setTimeout(() => setDebouncedQuery(value.trim()), 300);
    return () => clearTimeout(t);
  }, [value]);

  const canFetch = debouncedQuery.length > 0;
  const showPanel = open && canFetch;

  const suggestionFilters = { ...callFilters, q: debouncedQuery, limit: 8 };
  const publisherQuery = useCalls(suggestionFilters, {
    enabled: canFetch && role === "publisher",
  });
  const buyerQuery = useBuyerCalls(suggestionFilters, {
    enabled: canFetch && role === "buyer",
  });
  const { data, isFetching, isError } = role === "publisher" ? publisherQuery : buyerQuery;

  useEffect(() => {
    if (!open) return;
    function handleClick(e: MouseEvent) {
      if (containerRef.current && !containerRef.current.contains(e.target as Node)) {
        setOpen(false);
      }
    }
    document.addEventListener("mousedown", handleClick);
    return () => document.removeEventListener("mousedown", handleClick);
  }, [open]);

  const suggestions = data ?? [];

  const resolvedPlaceholder =
    placeholder ??
    (role === "buyer"
      ? "Search lead, caller, publisher, or contract…"
      : "Search lead, caller, buyer, or contract…");

  function handleChange(next: string) {
    setOpen(true);
    onChange(next);
  }

  function handleClear() {
    onChange("");
    onClear?.();
    setOpen(false);
  }

  return (
    <div ref={containerRef} className={`relative ${className ?? ""}`}>
      <div className="relative w-full">
        <Input
          type="search"
          value={value}
          onChange={(e) => handleChange(e.target.value)}
          onFocus={() => setOpen(true)}
          placeholder={resolvedPlaceholder}
          className={`pr-8 ${inputClassName ?? ""}`}
          autoComplete="off"
        />
        {value && (
          <button
            type="button"
            aria-label="Clear search"
            className="absolute right-2 top-1/2 -translate-y-1/2 text-gray-400 hover:text-gray-600"
            onClick={handleClear}
          >
            <X className="h-3.5 w-3.5" />
          </button>
        )}
      </div>
      {showPanel && (
        <div className="absolute left-0 top-full z-[100] mt-1 max-h-64 w-full min-w-[18rem] overflow-y-auto rounded-md border border-gray-100 bg-surface-card py-1 shadow-lg">
          {isError ? (
            <p className="px-3 py-3 text-sm text-red-500">Could not load suggestions</p>
          ) : isFetching && suggestions.length === 0 ? (
            <p className="px-3 py-3 text-sm text-gray-400">Searching…</p>
          ) : suggestions.length === 0 ? (
            <p className="px-3 py-3 text-sm text-gray-400">No matching calls</p>
          ) : (
            suggestions.map((call) => {
              const label = callPrimary(call);
              const secondary = callSecondary(call, role);
              return (
                <button
                  key={call.id}
                  type="button"
                  className="flex w-full flex-col px-3 py-2 text-left hover:bg-jade-50"
                  onMouseDown={(e) => e.preventDefault()}
                  onClick={() => {
                    onChange(label);
                    onSelectCall?.(call);
                    setOpen(false);
                  }}
                >
                  <span className="text-sm font-medium text-gray-800">{label}</span>
                  {secondary && <span className="text-xs text-gray-500">{secondary}</span>}
                </button>
              );
            })
          )}
        </div>
      )}
    </div>
  );
}
