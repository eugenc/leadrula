import { useEffect, useState } from "react";
import { Button } from "@/components/ui/button";
import { Input, Label } from "@/components/ui/input";
import { SectionLabel } from "@/components/layout/SectionLabel";
import { toast } from "@/store/toastStore";
import { errorMessage } from "@/lib/api";
import {
  useContractAppointmentSlots,
  useSaveContractAppointmentSlots,
  WEEKDAYS,
} from "@/features/appointments/hooks";
import type { ContractAppointmentSlot } from "@/types";

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
        Buyer has not configured appointment slots yet. Booking is blocked until they set up availability.
      </p>
    );
  }

  function toggle(id: number) {
    setSlots((prev) =>
      prev.map((s) => (s.buyer_slot_id === id ? { ...s, enabled: !s.enabled } : s))
    );
  }

  function setOverride(id: number, field: "duration_min_override" | "capacity_override", raw: string) {
    const val = raw.trim() === "" ? null : Number(raw);
    setSlots((prev) =>
      prev.map((s) => (s.buyer_slot_id === id ? { ...s, [field]: val } : s))
    );
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

  return (
    <div className="mt-4 space-y-3 border-t border-gray-100 pt-4">
      <SectionLabel>Appointment slots</SectionLabel>
      <p className="text-xs text-gray-400">
        Choose which buyer slots are bookable on this contract. Overrides are optional.
      </p>
      <div className="max-h-64 space-y-2 overflow-y-auto">
        {active.map((s) => (
          <div
            key={s.buyer_slot_id}
            className="grid grid-cols-[auto_1fr_auto_auto_auto] items-center gap-2 rounded border border-gray-100 px-2 py-1.5 text-sm"
          >
            <input type="checkbox" checked={s.enabled} onChange={() => toggle(s.buyer_slot_id)} />
            <span>
              {WEEKDAYS[s.weekday]} {s.start_time.slice(0, 5)} · {s.duration_min}m · cap {s.capacity}
            </span>
            <div>
              <Label className="text-xs">Dur</Label>
              <Input
                className="h-8 w-16"
                placeholder={String(s.duration_min)}
                value={s.duration_min_override ?? ""}
                onChange={(e) => setOverride(s.buyer_slot_id, "duration_min_override", e.target.value)}
              />
            </div>
            <div>
              <Label className="text-xs">Cap</Label>
              <Input
                className="h-8 w-14"
                placeholder={String(s.capacity)}
                value={s.capacity_override ?? ""}
                onChange={(e) => setOverride(s.buyer_slot_id, "capacity_override", e.target.value)}
              />
            </div>
          </div>
        ))}
      </div>
      <Button type="button" onClick={submit} disabled={save.isPending}>
        Save appointment slots
      </Button>
    </div>
  );
}
