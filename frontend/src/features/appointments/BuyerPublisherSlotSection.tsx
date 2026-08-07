import { useEffect, useState } from "react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { SectionLabel } from "@/components/layout/SectionLabel";
import { toast } from "@/store/toastStore";
import { errorMessage } from "@/lib/api";
import {
  useContractPublisherAppointmentSlots,
  useSaveContractPublisherAppointmentSlots,
  WEEKDAYS,
} from "@/features/appointments/hooks";
import type { ContractPublisherAppointmentSlot } from "@/types";

export function BuyerPublisherSlotSection({ contractId }: { contractId: number }) {
  const { data, isLoading } = useContractPublisherAppointmentSlots(contractId);
  const save = useSaveContractPublisherAppointmentSlots();
  const [slots, setSlots] = useState<ContractPublisherAppointmentSlot[]>([]);

  useEffect(() => {
    if (data) setSlots(data);
  }, [data]);

  if (isLoading) return <p className="text-sm text-gray-400">Loading publisher slots…</p>;
  if (!data?.length) {
    return (
      <p className="text-sm text-amber-700">
        No slots on the publisher calendar yet. The publisher must configure their calendar first.
      </p>
    );
  }

  function toggle(id: number) {
    setSlots((prev) =>
      prev.map((s) => (s.publisher_slot_id === id ? { ...s, enabled: !s.enabled } : s))
    );
  }

  function setOverride(id: number, field: "duration_min_override" | "capacity_override", raw: string) {
    const val = raw.trim() === "" ? null : Number(raw);
    setSlots((prev) =>
      prev.map((s) => (s.publisher_slot_id === id ? { ...s, [field]: val } : s))
    );
  }

  function submit() {
    save.mutate(
      {
        contractId,
        slots: slots.map((s) => ({
          publisher_slot_id: s.publisher_slot_id,
          enabled: s.enabled,
          duration_min_override: s.duration_min_override ?? null,
          capacity_override: s.capacity_override ?? null,
        })),
      },
      {
        onSuccess: () => toast.success("Publisher slots saved"),
        onError: (e) => toast.error(errorMessage(e)),
      }
    );
  }

  const active = slots.filter((s) => !s.disabled);

  return (
    <div className="space-y-3 border-t border-gray-100 pt-4">
      <SectionLabel>Publisher calendar slots</SectionLabel>
      <p className="text-xs text-gray-400">
        Choose which publisher slots are bookable on this contract. Overrides are optional.
      </p>
      <div className="max-h-64 space-y-2 overflow-y-auto">
        {active.map((s) => (
          <div
            key={s.publisher_slot_id}
            className="grid grid-cols-[auto_1fr_auto_auto_auto] items-center gap-2 rounded border border-gray-100 px-2 py-1.5 text-sm"
          >
            <input
              type="checkbox"
              checked={s.enabled}
              onChange={() => toggle(s.publisher_slot_id)}
            />
            <span>
              {WEEKDAYS[s.weekday]} {s.start_time.slice(0, 5)} · {s.duration_min}m · cap {s.capacity}
            </span>
            <Input
              className="w-16 px-1 py-0.5 text-xs"
              placeholder="dur"
              defaultValue={s.duration_min_override ?? ""}
              onBlur={(e) => setOverride(s.publisher_slot_id, "duration_min_override", e.target.value)}
            />
            <Input
              className="w-14 px-1 py-0.5 text-xs"
              placeholder="cap"
              defaultValue={s.capacity_override ?? ""}
              onBlur={(e) => setOverride(s.publisher_slot_id, "capacity_override", e.target.value)}
            />
          </div>
        ))}
      </div>
      <Button type="button" size="sm" onClick={submit} disabled={save.isPending}>
        Save publisher slots
      </Button>
    </div>
  );
}
