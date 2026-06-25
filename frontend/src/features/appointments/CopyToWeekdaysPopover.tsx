import { useState } from "react";
import { Copy } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Dropdown } from "@/components/ui/dropdown";
import { IconButton } from "@/components/layout/IconButton";
import { WEEKDAYS } from "@/features/appointments/hooks";

export function CopyToWeekdaysPopover({
  sourceWeekday,
  onApply,
  disabled,
}: {
  sourceWeekday: number;
  onApply: (toWeekdays: number[]) => void;
  disabled?: boolean;
}) {
  const [open, setOpen] = useState(false);
  const [targets, setTargets] = useState<number[]>([]);

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
          disabled={disabled}
          onClick={() => !disabled && setOpen((o) => !o)}
        >
          <Copy className="h-4 w-4" />
        </IconButton>
      }
    >
      <p className="mb-2 text-xs font-medium text-gray-500">Copy to</p>
      <div className="mb-3 space-y-1">
        {WEEKDAYS.map((label, i) =>
          i === sourceWeekday ? null : (
            <label key={label} className="flex cursor-pointer items-center gap-2 text-sm text-gray-700">
              <input
                type="checkbox"
                checked={targets.includes(i)}
                onChange={(e) =>
                  setTargets((prev) => (e.target.checked ? [...prev, i] : prev.filter((d) => d !== i)))
                }
              />
              {label}
            </label>
          )
        )}
      </div>
      <Button type="button" className="h-8 w-full text-xs" disabled={!targets.length} onClick={apply}>
        Apply
      </Button>
    </Dropdown>
  );
}
