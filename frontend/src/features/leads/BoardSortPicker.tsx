import { useState } from "react";
import { ArrowDown, ArrowUp, ArrowUpDown, ChevronDown } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Dropdown, DropdownItem } from "@/components/ui/dropdown";
import { boardSortOptions, sortKeyLabel } from "./leadsListColumns";
import type { CustomField } from "@/types";

interface Props {
  sort: string;
  sortDir: "asc" | "desc";
  customFields: CustomField[];
  onSortChange: (sort: string) => void;
  onSortDirChange: (dir: "asc" | "desc") => void;
}

export function BoardSortPicker({ sort, sortDir, customFields, onSortChange, onSortDirChange }: Props) {
  const [open, setOpen] = useState(false);
  const options = boardSortOptions(customFields);
  const groups = [...new Set(options.map((o) => o.group))];

  return (
    <div className="flex items-center gap-1">
      <Dropdown
        open={open}
        onClose={() => setOpen(false)}
        align="left"
        className="max-h-80 w-56 overflow-y-auto"
        trigger={
          <Button variant="outline" size="sm" onClick={() => setOpen((o) => !o)}>
            <ArrowUpDown className="h-4 w-4" />
            {sortKeyLabel(sort, customFields)}
            <ChevronDown className="h-4 w-4" />
          </Button>
        }
      >
        {groups.map((group) => (
          <div key={group}>
            <div className="px-2 py-1.5 text-xs font-semibold uppercase tracking-wide text-gray-400">
              {group}
            </div>
            {options
              .filter((o) => o.group === group)
              .map((o) => (
                <DropdownItem
                  key={o.sortKey}
                  selected={sort === o.sortKey}
                  onClick={() => {
                    onSortChange(o.sortKey);
                    setOpen(false);
                  }}
                >
                  {o.label}
                </DropdownItem>
              ))}
          </div>
        ))}
      </Dropdown>
      <Button
        variant="ghost"
        size="sm"
        className="px-2"
        title={sortDir === "asc" ? "Ascending" : "Descending"}
        onClick={() => onSortDirChange(sortDir === "asc" ? "desc" : "asc")}
      >
        {sortDir === "asc" ? <ArrowUp className="h-4 w-4" /> : <ArrowDown className="h-4 w-4" />}
      </Button>
    </div>
  );
}
