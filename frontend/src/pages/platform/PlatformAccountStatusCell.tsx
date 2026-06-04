import { Select } from "@/components/ui/input";
import type { AccountOperationalStatus } from "@/types";

export function PlatformAccountStatusCell({
  value,
  disabled,
  onChange,
}: {
  value: AccountOperationalStatus;
  disabled?: boolean;
  onChange: (status: AccountOperationalStatus) => void;
}) {
  return (
    <Select
      value={value}
      disabled={disabled}
      onChange={(e) => onChange(e.target.value as AccountOperationalStatus)}
      className="h-8 min-w-[7.5rem] text-sm"
    >
      <option value="active">Active</option>
      <option value="suspended">Suspended</option>
    </Select>
  );
}
