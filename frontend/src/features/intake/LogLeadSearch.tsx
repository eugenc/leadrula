import { useEffect, useRef, useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { X } from "lucide-react";
import { FilterInput } from "@/components/ui/input";
import { get, ns } from "@/lib/api";
import { leadDisplayName } from "./logShared";
import type { Lead, LeadListResponse } from "@/types";

function leadSecondary(lead: Lead): string {
  const parts = [lead.phone, lead.email].filter(Boolean);
  return parts.join(" · ");
}

export function LogLeadSearch({
  value,
  onChange,
  selectedLeadId,
  onSelectLead,
  onClear,
  className,
}: {
  value: string;
  onChange: (value: string) => void;
  selectedLeadId: number | null;
  onSelectLead: (lead: Lead) => void;
  onClear: () => void;
  className?: string;
}) {
  const [open, setOpen] = useState(false);
  const [debouncedQuery, setDebouncedQuery] = useState("");
  const containerRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    const t = setTimeout(() => setDebouncedQuery(value.trim()), 300);
    return () => clearTimeout(t);
  }, [value]);

  const showSuggestions = open && !selectedLeadId && debouncedQuery.length > 0;
  const { data, isFetching } = useQuery({
    queryKey: ["log-lead-search", debouncedQuery],
    queryFn: () => {
      const qs = new URLSearchParams({
        q: debouncedQuery,
        page: "1",
        limit: "8",
      });
      return get<LeadListResponse>(`${ns()}/leads?${qs}`);
    },
    enabled: showSuggestions,
  });

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

  return (
    <div ref={containerRef} className={`relative ${className ?? ""}`}>
      <div className="relative w-full">
        <FilterInput
          value={value}
          onChange={(e) => onChange(e.target.value)}
          onFocus={() => setOpen(true)}
          placeholder="Search lead name, phone, or email…"
          className="w-full pr-8"
          autoComplete="off"
        />
        {(value || selectedLeadId) && (
          <button
            type="button"
            aria-label="Clear search"
            className="absolute right-2 top-1/2 -translate-y-1/2 text-gray-400 hover:text-gray-600"
            onClick={() => {
              onClear();
              setOpen(false);
            }}
          >
            <X className="h-3.5 w-3.5" />
          </button>
        )}
      </div>
      {showSuggestions && (
        <div className="absolute left-0 top-full z-50 mt-1 max-h-64 w-full min-w-[16rem] overflow-y-auto rounded-md border border-gray-100 bg-surface-card py-1 shadow-lg">
          {isFetching && suggestions.length === 0 ? (
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
                    onSelectLead(lead);
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
