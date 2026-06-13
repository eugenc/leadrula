import { useEffect, useState } from "react";
import { Check } from "lucide-react";
import { cn } from "@/lib/utils";
import {
  sectionComplete,
  type ContractTabId,
} from "@/features/admin/contractSectionCompleteness";
import type { CompensationDraft } from "@/features/admin/CreateContractCompensationList";
import type { ContractDeliveryDraft } from "@/features/admin/contractCompensation";
import type { ContractOfferDraft } from "@/features/admin/contractOffer";
import type { ContractLeadCriteria } from "@/types";

const BASE_TABS: { id: ContractTabId; label: string; optional?: boolean }[] = [
  { id: "details", label: "Details" },
  { id: "compensation", label: "Compensation" },
  { id: "delivery", label: "Delivery" },
  { id: "criteria", label: "Lead criteria" },
  { id: "returns", label: "Return routes" },
];

export function ContractFormTabs({
  form,
  compensations,
  delivery,
  leadCriteria,
  offer,
  panels,
  showCheckmarks = true,
  extraTabs,
  resetKey,
  initialTab = "details",
}: {
  form: { contract_type: string; buyer_id: number; name: string; lead_type: string };
  compensations: CompensationDraft[];
  delivery: ContractDeliveryDraft;
  leadCriteria: ContractLeadCriteria;
  offer?: ContractOfferDraft;
  panels: Partial<Record<ContractTabId, React.ReactNode>>;
  showCheckmarks?: boolean;
  extraTabs?: { id: ContractTabId; label: string; optional?: boolean }[];
  resetKey?: string | number;
  initialTab?: ContractTabId;
}) {
  const [tab, setTab] = useState<ContractTabId>(initialTab);
  const ctx = { form, compensations, delivery, leadCriteria, offer };

  useEffect(() => {
    if (resetKey !== undefined) {
      setTab(initialTab);
    }
  }, [resetKey, initialTab]);
  const tabs = [...BASE_TABS, ...(extraTabs ?? [])];

  return (
    <div>
      <div className="mb-4 flex flex-wrap border-b border-gray-100">
        {tabs.filter((t) => panels[t.id] != null).map((t) => {
          const done = showCheckmarks && sectionComplete(t.id, ctx);
          return (
            <button
              key={t.id}
              type="button"
              onClick={() => setTab(t.id)}
              className={cn(
                "-mb-px flex items-center gap-1.5 border-b-2 px-3 py-2 text-sm font-semibold transition-colors",
                tab === t.id ? "border-jade-500 text-jade-700" : "border-transparent text-gray-400"
              )}
            >
              {done && <Check className="h-3.5 w-3.5 text-jade-600" />}
              <span>
                {t.label}
                {t.optional ? " (optional)" : ""}
              </span>
            </button>
          );
        })}
      </div>
      {panels[tab] ?? null}
    </div>
  );
}
