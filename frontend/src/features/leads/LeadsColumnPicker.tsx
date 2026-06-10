import { useState } from "react";
import {
  DragDropContext,
  Droppable,
  Draggable,
  type DropResult,
} from "@hello-pangea/dnd";
import { GripVertical, Columns3 } from "lucide-react";
import { Dropdown, DropdownItem, DropdownSearch } from "@/components/ui/dropdown";
import { Button } from "@/components/ui/button";
import type { CustomField } from "@/types";
import { SYSTEM_COLUMNS, columnLabel, columnsEqual, resetToDefaultColumns } from "./leadsListColumns";
import { cn } from "@/lib/utils";

interface Props {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  visibleCols: string[];
  allColumnIds: string[];
  customFields: CustomField[];
  defaultCols: string[];
  lockedCols: string[];
  onChange: (cols: string[]) => void;
  label?: string;
}

export function LeadsColumnPicker({
  open,
  onOpenChange,
  visibleCols,
  allColumnIds,
  customFields,
  defaultCols,
  lockedCols,
  onChange,
  label = "Columns",
}: Props) {
  const [query, setQuery] = useState("");
  const hiddenCols = allColumnIds.filter((id) => !visibleCols.includes(id));
  const showReset = !columnsEqual(visibleCols, defaultCols);

  const q = query.trim().toLowerCase();
  const searching = q.length > 0;

  function columnMatches(id: string) {
    if (!searching) return true;
    return (
      columnLabel(id, customFields).toLowerCase().includes(q) ||
      id.toLowerCase().includes(q)
    );
  }

  const filteredVisible = searching ? visibleCols.filter(columnMatches) : visibleCols;
  const filteredHidden = searching ? hiddenCols.filter(columnMatches) : hiddenCols;
  const noResults = searching && filteredVisible.length === 0 && filteredHidden.length === 0;

  function closeDropdown() {
    setQuery("");
    onOpenChange(false);
  }

  function onDragEnd(result: DropResult) {
    if (!result.destination || searching) return;
    const next = [...visibleCols];
    const [moved] = next.splice(result.source.index, 1);
    next.splice(result.destination.index, 0, moved!);
    onChange(next);
  }

  function showColumn(id: string) {
    onChange([...visibleCols, id]);
  }

  function hideColumn(id: string) {
    if (lockedCols.includes(id)) return;
    const next = visibleCols.filter((c) => c !== id);
    if (next.length === 0) return;
    onChange(next);
  }

  function resetColumns() {
    onChange(resetToDefaultColumns(defaultCols, allColumnIds));
  }

  return (
    <Dropdown
      open={open}
      onClose={closeDropdown}
      align="right"
      className="max-h-96 w-64 overflow-y-auto"
      trigger={
        <Button variant="outline" size="sm" onClick={() => onOpenChange(!open)}>
          <Columns3 className="h-4 w-4" />
          {label}
        </Button>
      }
    >
      <DropdownSearch
        value={query}
        onChange={setQuery}
        placeholder={`Search ${label.toLowerCase()}…`}
      />

      {noResults ? (
        <p className="px-2.5 py-2 text-xs text-gray-400">No columns match</p>
      ) : (
        <>
          <div className="px-2 py-1 text-xs font-semibold uppercase tracking-wide text-gray-400">
            Visible — drag to reorder
          </div>
          <DragDropContext onDragEnd={onDragEnd}>
            <Droppable droppableId="visible-cols" isDropDisabled={searching}>
              {(provided) => (
                <div ref={provided.innerRef} {...provided.droppableProps}>
                  {filteredVisible.map((id, index) => (
                    <Draggable key={id} draggableId={id} index={index} isDragDisabled={searching}>
                      {(drag, snapshot) => (
                        <div
                          ref={drag.innerRef}
                          {...drag.draggableProps}
                          className={cn(
                            "flex h-9 items-center gap-1 rounded-md px-1 text-base text-gray-700",
                            snapshot.isDragging ? "bg-jade-50 shadow-sm" : "hover:bg-jade-50"
                          )}
                        >
                          {!searching && (
                            <span
                              {...drag.dragHandleProps}
                              className="cursor-grab px-1 text-gray-400 hover:text-gray-600"
                            >
                              <GripVertical className="h-4 w-4" />
                            </span>
                          )}
                          <span className="min-w-0 flex-1 truncate">
                            {columnLabel(id, customFields)}
                          </span>
                          {!lockedCols.includes(id) && (
                            <button
                              type="button"
                              onClick={() => hideColumn(id)}
                              className="shrink-0 px-1.5 text-xs text-gray-400 hover:text-gray-600"
                              aria-label={`Hide ${columnLabel(id, customFields)}`}
                            >
                              ×
                            </button>
                          )}
                        </div>
                      )}
                    </Draggable>
                  ))}
                  {provided.placeholder}
                </div>
              )}
            </Droppable>
          </DragDropContext>

          {filteredHidden.length > 0 && (
            <>
              <div className="mt-1 border-t border-gray-100 px-2 py-1 text-xs font-semibold uppercase tracking-wide text-gray-400">
                Hidden
              </div>
              {filteredHidden.map((id) => (
                <DropdownItem key={id} onClick={() => showColumn(id)}>
                  + {columnLabel(id, customFields)}
                </DropdownItem>
              ))}
            </>
          )}
        </>
      )}

      {showReset && (
        <div className="mt-1 border-t border-gray-100 px-2 py-1">
          <button
            type="button"
            onClick={resetColumns}
            className="w-full rounded-md px-2 py-1.5 text-left text-sm text-gray-600 hover:bg-jade-50 hover:text-gray-800"
          >
            Reset to default
          </button>
        </div>
      )}

      {!searching &&
        SYSTEM_COLUMNS.filter((c) => !allColumnIds.includes(c.id)).length === 0 &&
        hiddenCols.length === 0 &&
        visibleCols.length === allColumnIds.length && (
          <div className="px-2 py-2 text-sm text-gray-400">All columns visible</div>
        )}
    </Dropdown>
  );
}
