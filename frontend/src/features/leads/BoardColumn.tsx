import { useRef } from "react";
import { useDraggable, useDroppable } from "@dnd-kit/core";
import { useVirtualizer } from "@tanstack/react-virtual";
import { LeadCard } from "./LeadCard";
import { Button } from "@/components/ui/button";
import { Spinner } from "@/components/ui/misc";
import { cn } from "@/lib/utils";
import { stageColorDot, stageColorLine } from "@/features/pipelines/stageColors";
import { isBoardDraggable } from "./boardStage";
import type { AccountType, CustomField, Lead, Stage, StageType } from "@/types";

function VirtualLeadRow({
  lead,
  stageId,
  stageType,
  customFields,
  cardFields,
  onClick,
  isDragOverlaySource,
  draggable,
}: {
  lead: Lead;
  stageId: number;
  stageType: StageType;
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
        stageType={stageType}
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
  headerHint,
  loading = false,
  hasMore = false,
  onLoadMore,
  loadingMore = false,
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
  headerHint?: string;
  loading?: boolean;
  hasMore?: boolean;
  onLoadMore?: () => void;
  loadingMore?: boolean;
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

  const totalLabel = hasMore ? `${items.length}+` : String(items.length);

  return (
    <div
      className={cn(
        "flex h-full min-h-0 w-[260px] shrink-0 flex-col rounded-lg bg-gray-50 sm:w-[280px]",
        isOver && "ring-2 ring-jade-400/40"
      )}
    >
      <div className="flex shrink-0 items-center gap-2 border-b border-gray-100 px-3.5 py-2.5">
        <span className={cn("h-2 w-2 shrink-0 rounded-full", stageColorDot(stage.color))} />
        <span
          className="flex-1 text-base font-semibold text-gray-700"
          title={headerHint}
        >
          {stage.name}
        </span>
        <span className="text-xs text-gray-400">{totalLabel}</span>
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
        {loading ? (
          <div className="flex justify-center py-8">
            <Spinner className="h-5 w-5" />
          </div>
        ) : items.length === 0 ? (
          <p className="py-4 text-center text-xs text-gray-400">No leads</p>
        ) : (
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
                    stageType={stage.stage_type}
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
        )}
        {hasMore && onLoadMore && (
          <div className="pb-2 pt-1">
            <Button
              variant="ghost"
              size="sm"
              className="w-full text-xs"
              onClick={onLoadMore}
              disabled={loadingMore}
            >
              {loadingMore ? "Loading…" : "Load more"}
            </Button>
          </div>
        )}
      </div>
    </div>
  );
}
