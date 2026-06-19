import { LeadSearchInput } from "@/features/leads/LeadSearchInput";
import type { Lead } from "@/types";

export function LogLeadSearch({
  source,
  value,
  onChange,
  selectedLeadId,
  onSelectLead,
  onClear,
  className,
}: {
  source: "publisher" | "buyer";
  value: string;
  onChange: (value: string) => void;
  selectedLeadId: number | null;
  onSelectLead: (lead: Lead) => void;
  onClear: () => void;
  className?: string;
}) {
  return (
    <LeadSearchInput
      value={value}
      onChange={onChange}
      onClear={onClear}
      className={className}
      inputClassName="w-full text-sm"
      placeholder="Search lead name, phone, or email…"
      leadFilters={{ namespace: source }}
      suggestionsEnabled={!selectedLeadId}
      onSelectLead={onSelectLead}
    />
  );
}
