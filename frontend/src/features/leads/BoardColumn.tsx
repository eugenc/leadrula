import { useRef } from "react";
import { useDraggable, useDroppable } from "@dnd-kit/core";
import { useVirtualizer } from "@tanstack/react-virtual";
import { LeadCard } from "./LeadCard";
import { cn } from "@/lib/utils";
import { stageColorDot, stageColorLine } from "@/features/pipelines/stageColors";
import { isBoardDraggable } from "./boardStage";
import type { AccountType, CustomField, Lead, Stage } from "@/types";

function VirtualLeadRow({
  lead,
  stageId,
  customFields,
  cardFields,
  onClick,
  isDragOverlaySource,
  draggable,
}: {
  lead: Lead;
  stageId: number;
  customFields: CustomField[];
  cardFields: string[];
  onClick: () => void;
  isDragOverlaySource: boolean;
  draggable: boolean;
}) {
  const { attributes, listeners, setNodeRef, isDragging } = useDraggable({
    id: String(lead.id),
    data: { lead, stageId },
    disabled: !draggable,
  });

  return (
    <div
      ref={setNodeRef}
      className={cn(
        draggable ? "cursor-grab active:cursor-grabbing" : "cursor-default",
        (isDragging || isDragOverlaySource) && "opacity-40"
      )}
      {...(draggable ? listeners : {})}
      {...(draggable ? attributes : {})}
    >
      <LeadCard
        lead={lead}
        customFields={customFields}
        cardFields={cardFields}
        onClick={onClick}
      />
    </div>
  );
}

export function BoardColumn({
  stage,
  items,
  customFields,
  cardFields,
  rowHeight,
  onCardClick,
  activeDragId,
  accountType,
  droppable = true,
}: {
  stage: Stage;
  items: Lead[];
  customFields: CustomField[];
  cardFields: string[];
  rowHeight: number;
  onCardClick: (leadId: number) => void;
  activeDragId: string | null;
  accountType?: AccountType;
  droppable?: boolean;
}) {
  const scrollRef = useRef<HTMLDivElement>(null);
  const { setNodeRef, isOver } = useDroppable({
    id: String(stage.id),
    data: { stageId: stage.id },
    disabled: !droppable,
  });

  const virtualizer = useVirtualizer({
    count: items.length,
    getScrollElement: () => scrollRef.current,
    estimateSize: () => rowHeight,
    overscan: 5,
  });

  return (
    <div
      className={cn(
        "flex h-full min-h-0 w-[260px] shrink-0 flex-col rounded-lg bg-gray-50 sm:w-[280px]",
        isOver && "ring-2 ring-jade-400/40"
      )}
    >
      <div className="flex shrink-0 items-center gap-2 border-b border-gray-100 px-3.5 py-2.5">
        <span className={cn("h-2 w-2 shrink-0 rounded-full", stageColorDot(stage.color))} />
        <span className="flex-1 text-base font-semibold text-gray-700">{stage.name}</span>
        <span className="text-xs text-gray-400">{items.length}</span>
      </div>
      <div
        ref={(node) => {
          scrollRef.current = node;
          setNodeRef(node);
        }}
        className="relative min-h-0 flex-1 overflow-y-auto p-2 pl-3"
      >
        <span
          aria-hidden
          className={cn(
            "pointer-events-none absolute bottom-0 left-0 top-0 w-px",
            stageColorLine(stage.color)
          )}
        />
        <div
          style={{
            height: virtualizer.getTotalSize(),
            width: "100%",
            position: "relative",
          }}
        >
          {virtualizer.getVirtualItems().map((vi) => {
            const lead = items[vi.index];
            if (!lead) return null;
            return (
              <div
                key={lead.id}
                data-index={vi.index}
                ref={virtualizer.measureElement}
                className="pb-2"
                style={{
                  position: "absolute",
                  top: 0,
                  left: 0,
                  width: "100%",
                  transform: `translateY(${vi.start}px)`,
                }}
              >
                <VirtualLeadRow
                  lead={lead}
                  stageId={stage.id}
                  customFields={customFields}
                  cardFields={cardFields}
                  onClick={() => onCardClick(lead.id)}
                  isDragOverlaySource={activeDragId === String(lead.id)}
                  draggable={isBoardDraggable(lead, accountType)}
                />
              </div>
            );
          })}
        </div>
      </div>
    </div>
  );
}
