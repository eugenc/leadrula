import { useState } from "react";
import { ListFilter } from "lucide-react";
import { Dropdown } from "@/components/ui/dropdown";
import { IconButton } from "@/components/layout/IconButton";
import { SLOT_CHECKBOX_CLASS } from "@/features/appointments/slotGrid";
import { cn } from "@/lib/utils";
import { activityGroupLabel, type ActivityFilterGroup } from "./activityFilterStorage";

export function ActivityFilterDropdown({
  groups,
  isVisible,
  toggleGroup,
  hiddenCount,
}: {
  groups: ActivityFilterGroup[];
  isVisible: (group: ActivityFilterGroup) => boolean;
  toggleGroup: (group: ActivityFilterGroup) => void;
  hiddenCount: number;
}) {
  const [open, setOpen] = useState(false);

  return (
    <Dropdown
      open={open}
      onClose={() => setOpen(false)}
      align="right"
      className="w-auto p-3"
      trigger={
        <IconButton
          aria-label="Filter activity types"
          className={cn(hiddenCount > 0 && "text-jade-600 hover:text-jade-700")}
          onClick={() => setOpen((o) => !o)}
        >
          <ListFilter className="h-4 w-4" />
        </IconButton>
      }
    >
      <div className="grid grid-flow-col grid-rows-3 gap-x-5 gap-y-1.5">
        {groups.map((group) => (
          <label
            key={group}
            className="flex cursor-pointer items-center gap-2 whitespace-nowrap text-xs text-gray-500"
          >
            <input
              type="checkbox"
              className={SLOT_CHECKBOX_CLASS}
              checked={isVisible(group)}
              onChange={() => toggleGroup(group)}
            />
            {activityGroupLabel(group)}
          </label>
        ))}
      </div>
    </Dropdown>
  );
}
