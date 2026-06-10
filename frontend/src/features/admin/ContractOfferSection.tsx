import { Label, Select } from "@/components/ui/input";
import { SectionLabel } from "@/components/layout/SectionLabel";
import {
  DISTRIBUTION_STRATEGIES,
  normalizeContractOffer,
  PUBLISHER_DELIVERY_MODES,
  REQUIRED_OFFER_DELIVERY_MODE,
  type ContractOfferDraft,
} from "@/features/admin/contractOffer";

export function ContractOfferSection({
  value,
  onChange,
}: {
  value: ContractOfferDraft;
  onChange: (v: ContractOfferDraft) => void;
}) {
  function setOffer(next: ContractOfferDraft) {
    onChange(normalizeContractOffer(next));
  }

  function toggleMode(mode: string) {
    if (mode === REQUIRED_OFFER_DELIVERY_MODE) return;
    const set = new Set(value.allowed_delivery_modes);
    if (set.has(mode)) set.delete(mode);
    else set.add(mode);
    setOffer({ ...value, allowed_delivery_modes: [...set] });
  }

  return (
    <div className="flex flex-col gap-2.5">
      <SectionLabel>Publisher delivery offer</SectionLabel>
      <p className="text-xs text-gray-400">
        Buyers choose one of these modes when accepting. CRM integrations are configured per buyer after accept.
        Lead inbox is always included for open offers.
      </p>
      <div>
        <Label>Allowed delivery modes</Label>
        <div className="mt-1.5 flex flex-col gap-1.5">
          {PUBLISHER_DELIVERY_MODES.map((m) => {
            const locked = m.value === REQUIRED_OFFER_DELIVERY_MODE;
            return (
              <label key={m.value} className="flex items-center gap-2 text-sm text-gray-700">
                <input
                  type="checkbox"
                  checked={locked || value.allowed_delivery_modes.includes(m.value)}
                  disabled={locked}
                  onChange={() => {
                    if (!locked) toggleMode(m.value);
                  }}
                />
                {m.label}
              </label>
            );
          })}
        </div>
      </div>
      <div>
        <Label>Distribution strategy</Label>
        <Select
          value={value.distribution_strategy}
          onChange={(e) => setOffer({ ...value, distribution_strategy: e.target.value })}
        >
          {DISTRIBUTION_STRATEGIES.map((s) => (
            <option key={s.value} value={s.value}>
              {s.label}
            </option>
          ))}
        </Select>
        <p className="mt-1 text-xs text-gray-400">
          How leads are assigned among active buyers on this contract. Bid compensation rows are excluded until RTB.
        </p>
      </div>
    </div>
  );
}
