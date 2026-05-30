import {
  DragDropContext,
  Droppable,
  Draggable,
  type DropResult,
} from "@hello-pangea/dnd";
import { GripVertical, Columns3 } from "lucide-react";
import { Dropdown, DropdownItem } from "@/components/ui/dropdown";
import { Button } from "@/components/ui/button";
import type { CustomField } from "@/types";
import { SYSTEM_COLUMNS, columnLabel } from "./leadsListColumns";
import { cn } from "@/lib/utils";

interface Props {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  visibleCols: string[];
  allColumnIds: string[];
  customFields: CustomField[];
  onChange: (cols: string[]) => void;
  label?: string;
}

export function LeadsColumnPicker({
  open,
  onOpenChange,
  visibleCols,
  allColumnIds,
  customFields,
  onChange,
  label = "Columns",
}: Props) {
  const hiddenCols = allColumnIds.filter((id) => !visibleCols.includes(id));

  function onDragEnd(result: DropResult) {
    if (!result.destination) return;
    const next = [...visibleCols];
    const [moved] = next.splice(result.source.index, 1);
    next.splice(result.destination.index, 0, moved!);
    onChange(next);
  }

  function showColumn(id: string) {
    onChange([...visibleCols, id]);
  }

  function hideColumn(id: string) {
    const next = visibleCols.filter((c) => c !== id);
    if (next.length === 0) return;
    onChange(next);
  }

  return (
    <Dropdown
      open={open}
      onClose={() => onOpenChange(false)}
      align="right"
      className="max-h-96 w-64 overflow-y-auto"
      trigger={
        <Button variant="outline" size="sm" onClick={() => onOpenChange(!open)}>
          <Columns3 className="h-4 w-4" />
          {label}
        </Button>
      }
    >
      <div className="px-2 py-1 text-xs font-semibold uppercase tracking-wide text-gray-400">
        Visible — drag to reorder
      </div>
      <DragDropContext onDragEnd={onDragEnd}>
        <Droppable droppableId="visible-cols">
          {(provided) => (
            <div ref={provided.innerRef} {...provided.droppableProps}>
              {visibleCols.map((id, index) => (
                <Draggable key={id} draggableId={id} index={index}>
                  {(drag, snapshot) => (
                    <div
                      ref={drag.innerRef}
                      {...drag.draggableProps}
                      className={cn(
                        "flex h-9 items-center gap-1 rounded-md px-1 text-base text-gray-700",
                        snapshot.isDragging ? "bg-jade-50 shadow-sm" : "hover:bg-jade-50"
                      )}
                    >
                      <span
                        {...drag.dragHandleProps}
                        className="cursor-grab px-1 text-gray-400 hover:text-gray-600"
                      >
                        <GripVertical className="h-4 w-4" />
                      </span>
                      <span className="min-w-0 flex-1 truncate">
                        {columnLabel(id, customFields)}
                      </span>
                      <button
                        type="button"
                        onClick={() => hideColumn(id)}
                        className="shrink-0 px-1.5 text-xs text-gray-400 hover:text-gray-600"
                        aria-label={`Hide ${columnLabel(id, customFields)}`}
                      >
                        ×
                      </button>
                    </div>
                  )}
                </Draggable>
              ))}
              {provided.placeholder}
            </div>
          )}
        </Droppable>
      </DragDropContext>

      {hiddenCols.length > 0 && (
        <>
          <div className="mt-1 border-t border-gray-100 px-2 py-1 text-xs font-semibold uppercase tracking-wide text-gray-400">
            Hidden
          </div>
          {hiddenCols.map((id) => (
            <DropdownItem key={id} onClick={() => showColumn(id)}>
              + {columnLabel(id, customFields)}
            </DropdownItem>
          ))}
        </>
      )}

      {SYSTEM_COLUMNS.filter((c) => !allColumnIds.includes(c.id)).length === 0 &&
        hiddenCols.length === 0 &&
        visibleCols.length === allColumnIds.length && (
          <div className="px-2 py-2 text-sm text-gray-400">All columns visible</div>
        )}
    </Dropdown>
  );
}
