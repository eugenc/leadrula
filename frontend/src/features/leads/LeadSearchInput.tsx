import { useEffect, useRef, useState } from "react";
import { X } from "lucide-react";
import { Input } from "@/components/ui/input";
import { useLeads, type LeadFilters } from "./hooks";
import { leadDisplayName } from "@/features/intake/logShared";
import type { Lead } from "@/types";

function leadSecondary(lead: Lead): string {
  const parts = [lead.phone, lead.email].filter(Boolean);
  return parts.join(" · ");
}

export function LeadSearchInput({
  value,
  onChange,
  onSelectLead,
  onClear,
  placeholder = "Search name, email, phone, address, buyer, or status…",
  className,
  inputClassName,
  leadFilters,
  suggestionsEnabled = true,
}: {
  value: string;
  onChange: (value: string) => void;
  onSelectLead?: (lead: Lead) => void;
  onClear?: () => void;
  placeholder?: string;
  className?: string;
  inputClassName?: string;
  leadFilters?: Omit<LeadFilters, "q" | "page" | "limit">;
  suggestionsEnabled?: boolean;
}) {
  const [open, setOpen] = useState(false);
  const [debouncedQuery, setDebouncedQuery] = useState("");
  const containerRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    const t = setTimeout(() => setDebouncedQuery(value.trim()), 300);
    return () => clearTimeout(t);
  }, [value]);

  const canFetch = suggestionsEnabled && debouncedQuery.length > 0;
  const showPanel = open && canFetch;

  const { data, isFetching, isError } = useLeads(
    { ...leadFilters, q: debouncedQuery, page: 1, limit: 8 },
    { enabled: canFetch }
  );

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

  const suggestions = data?.items ?? [];

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
          placeholder={placeholder}
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
        <div className="absolute left-0 top-full z-[100] mt-1 max-h-64 w-full min-w-[16rem] overflow-y-auto rounded-md border border-gray-100 bg-surface-card py-1 shadow-lg">
          {isError ? (
            <p className="px-3 py-3 text-sm text-red-500">Could not load suggestions</p>
          ) : isFetching && suggestions.length === 0 ? (
            <p className="px-3 py-3 text-sm text-gray-400">Searching…</p>
          ) : suggestions.length === 0 ? (
            <p className="px-3 py-3 text-sm text-gray-400">No matching leads</p>
          ) : (
            suggestions.map((lead) => {
              const label = leadDisplayName(lead.first_name, lead.last_name, lead.public_id);
              const secondary = leadSecondary(lead);
              return (
                <button
                  key={lead.id}
                  type="button"
                  className="flex w-full flex-col px-3 py-2 text-left hover:bg-jade-50"
                  onMouseDown={(e) => e.preventDefault()}
                  onClick={() => {
                    onChange(label);
                    onSelectLead?.(lead);
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
