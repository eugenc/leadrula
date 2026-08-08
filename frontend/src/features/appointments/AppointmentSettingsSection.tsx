import { useEffect, useState } from "react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { SectionLabel } from "@/components/layout/SectionLabel";
import { cn } from "@/lib/utils";
import { toast } from "@/store/toastStore";
import { errorMessage } from "@/lib/api";
import {
  useContractAppointmentSlots,
  useSaveContractAppointmentSlots,
  WEEKDAYS,
} from "@/features/appointments/hooks";
import { CONTRACT_SLOT_ROW_GRID, SLOT_CHECKBOX_CLASS, slotEndTime } from "@/features/appointments/slotGrid";
import { TimeFieldInput } from "@/features/appointments/TimeFieldInput";
import type { ContractAppointmentSlot } from "@/types";

function groupSlotsByWeekday(slots: ContractAppointmentSlot[]) {
  const byDay = new Map<number, ContractAppointmentSlot[]>();
  for (const slot of slots) {
    const list = byDay.get(slot.weekday) ?? [];
    list.push(slot);
    byDay.set(slot.weekday, list);
  }
  for (const list of byDay.values()) {
    list.sort((a, b) => a.start_time.localeCompare(b.start_time));
  }
  return [...byDay.entries()].sort(([a], [b]) => a - b);
}

export function AppointmentSettingsSection({ contractId }: { contractId: number }) {
  const { data, isLoading } = useContractAppointmentSlots(contractId);
  const save = useSaveContractAppointmentSlots();
  const [slots, setSlots] = useState<ContractAppointmentSlot[]>([]);

  useEffect(() => {
    if (data) setSlots(data);
  }, [data]);

  if (isLoading) return <p className="text-sm text-gray-400">Loading appointment slots…</p>;
  if (!data?.length) {
    return (
      <p className="text-sm text-amber-700">
        No bookable slots for this contract. The buyer must attach a booking calendar and configure slots
        before publishers can book.
      </p>
    );
  }

  function toggle(id: number) {
    setSlots((prev) =>
      prev.map((s) => (s.buyer_slot_id === id ? { ...s, enabled: !s.enabled } : s))
    );
  }

  function setDurationOverride(id: number, raw: string) {
    const val = raw.trim() === "" ? null : Number(raw);
    setSlots((prev) =>
      prev.map((s) => (s.buyer_slot_id === id ? { ...s, duration_min_override: val } : s))
    );
  }

  function setCapacityOverride(id: number, raw: string) {
    const val = raw.trim() === "" ? null : Number(raw);
    setSlots((prev) =>
      prev.map((s) => (s.buyer_slot_id === id ? { ...s, capacity_override: val } : s))
    );
  }

  function setAllEnabled(enabled: boolean) {
    setSlots((prev) => prev.map((s) => (s.disabled ? s : { ...s, enabled })));
  }

  function submit() {
    save.mutate(
      {
        contractId,
        slots: slots.map((s) => ({
          buyer_slot_id: s.buyer_slot_id,
          enabled: s.enabled,
          duration_min_override: s.duration_min_override ?? null,
          capacity_override: s.capacity_override ?? null,
        })),
      },
      {
        onSuccess: () => toast.success("Appointment slots saved"),
        onError: (e) => toast.error(errorMessage(e)),
      }
    );
  }

  const active = slots.filter((s) => !s.disabled);
  const grouped = groupSlotsByWeekday(active);

  return (
    <div className="mt-4 space-y-3 border-t border-gray-100 pt-4">
      <SectionLabel>Appointment Slots</SectionLabel>
      <p className="text-xs text-gray-400">
        Choose which buyer slots are bookable on this contract. Overrides are optional.
      </p>
      <div className="flex justify-end gap-3">
        <button
          type="button"
          className="text-sm text-jade-600 hover:underline"
          onClick={() => setAllEnabled(true)}
        >
          Select All
        </button>
        <button
          type="button"
          className="text-sm text-jade-600 hover:underline"
          onClick={() => setAllEnabled(false)}
        >
          Clear All
        </button>
      </div>
      <div className="max-h-80 space-y-1 overflow-y-auto">
        <div className={cn(CONTRACT_SLOT_ROW_GRID, "px-0")}>
          <span />
          <span />
          <span className="text-xs font-medium text-gray-400">From</span>
          <span className="text-xs font-medium text-gray-400">To</span>
          <span className="text-xs font-medium text-gray-400">Dur</span>
          <span className="text-xs font-medium text-gray-400">Cap</span>
        </div>
        {grouped.map(([weekday, daySlots]) => (
          <div key={weekday} className="space-y-1">
            {daySlots.map((s, i) => {
              const durationMin = s.duration_min_override ?? s.duration_min;
              return (
                <div key={s.buyer_slot_id} className={CONTRACT_SLOT_ROW_GRID}>
                  <input
                    type="checkbox"
                    className={SLOT_CHECKBOX_CLASS}
                    checked={s.enabled}
                    onChange={() => toggle(s.buyer_slot_id)}
                  />
                  <span className="text-sm font-medium text-gray-700">{i === 0 ? WEEKDAYS[weekday] : ""}</span>
                  <div className="min-w-0 w-full">
                    <TimeFieldInput value={s.start_time} disabled onChange={() => {}} />
                  </div>
                  <div className="min-w-0 w-full">
                    <TimeFieldInput
                      value={slotEndTime(s.start_time, durationMin)}
                      disabled
                      onChange={() => {}}
                    />
                  </div>
                  <div className="min-w-0 w-full">
                    <Input
                      type="number"
                      min={1}
                      className="px-2 text-center"
                      placeholder={String(s.duration_min)}
                      value={s.duration_min_override ?? ""}
                      onChange={(e) => setDurationOverride(s.buyer_slot_id, e.target.value)}
                    />
                  </div>
                  <div className="min-w-0 w-full">
                    <Input
                      type="number"
                      min={1}
                      className="px-2 text-center"
                      placeholder={String(s.capacity)}
                      value={s.capacity_override ?? ""}
                      onChange={(e) => setCapacityOverride(s.buyer_slot_id, e.target.value)}
                    />
                  </div>
                </div>
              );
            })}
          </div>
        ))}
      </div>
      <Button type="button" onClick={submit} disabled={save.isPending}>
        Save appointment slots
      </Button>
    </div>
  );
}
