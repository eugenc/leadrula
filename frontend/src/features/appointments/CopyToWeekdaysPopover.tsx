import { useState } from "react";
import { Copy } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Dropdown } from "@/components/ui/dropdown";
import { IconButton } from "@/components/layout/IconButton";
import { WEEKDAYS } from "@/features/appointments/hooks";
import { SLOT_CHECKBOX_CLASS } from "@/features/appointments/slotGrid";

const ALL_WEEKDAYS = [0, 1, 2, 3, 4, 5, 6];

export function CopyToWeekdaysPopover({
  sourceWeekday,
  targetWeekdays,
  onApply,
  disabled,
}: {
  sourceWeekday: number;
  targetWeekdays?: number[];
  onApply: (toWeekdays: number[]) => void;
  disabled?: boolean;
}) {
  const [open, setOpen] = useState(false);
  const [targets, setTargets] = useState<number[]>([]);
  const candidates = (targetWeekdays ?? ALL_WEEKDAYS).filter((i) => i !== sourceWeekday);
  const triggerDisabled = disabled || candidates.length === 0;

  function close() {
    setOpen(false);
    setTargets([]);
  }

  function apply() {
    if (!targets.length) return;
    onApply(targets);
    close();
  }

  return (
    <Dropdown
      open={open}
      onClose={close}
      align="right"
      className="w-52 p-3"
      trigger={
        <IconButton
          aria-label={`Copy ${WEEKDAYS[sourceWeekday]} to other days`}
          disabled={triggerDisabled}
          onClick={() => !triggerDisabled && setOpen((o) => !o)}
        >
          <Copy className="h-4 w-4" />
        </IconButton>
      }
    >
      <p className="mb-2 text-xs font-medium text-gray-500">Copy to</p>
      {candidates.length === 0 ? (
        <p className="mb-3 text-sm text-gray-500">No other open days</p>
      ) : (
        <div className="mb-3 space-y-1">
          {candidates.map((i) => (
            <label key={i} className="flex cursor-pointer items-center gap-2 text-sm text-gray-700">
              <input
                type="checkbox"
                className={SLOT_CHECKBOX_CLASS}
                checked={targets.includes(i)}
                onChange={(e) =>
                  setTargets((prev) => (e.target.checked ? [...prev, i] : prev.filter((d) => d !== i)))
                }
              />
              {WEEKDAYS[i]}
            </label>
          ))}
        </div>
      )}
      <Button type="button" className="h-8 w-full text-xs" disabled={!targets.length} onClick={apply}>
        Apply
      </Button>
    </Dropdown>
  );
}
