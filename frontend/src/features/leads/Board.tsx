import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import {
  DragDropContext,
  Droppable,
  Draggable,
  type DropResult,
} from "@hello-pangea/dnd";
import {
  usePipelines,
  useStages,
  useLeads,
  useChangeStage,
  useCustomFields,
} from "./hooks";
import { BoardSortPicker } from "./BoardSortPicker";
import { LeadCard } from "./LeadCard";
import { LeadsColumnPicker } from "./LeadsColumnPicker";
import { StagePromptModal, type PromptResult } from "./StagePromptModal";
import { LeadFilterBuilder } from "./LeadFilterBuilder";
import { LeadViewsMenu } from "./LeadViewsMenu";
import {
  useSavedLeadViews,
  useActiveViewId,
  mergeViews,
  getViewById,
  viewStateEqual,
  type FilterCondition,
  type SavedLeadView,
} from "./leadsViews";
import { FilterSelect } from "@/components/ui/input";
import { Button } from "@/components/ui/button";
import { Spinner, EmptyState } from "@/components/ui/misc";
import { useUIStore } from "@/store/uiStore";
import { apiError } from "@/lib/api";
import { toast } from "@/store/toastStore";
import { cn } from "@/lib/utils";
import { stageColorDot, stageColorLine } from "@/features/pipelines/stageColors";
import { SYSTEM_COLUMNS, boardCardFields, DEFAULT_BOARD_CARD_FIELDS, PIPELINE_COLUMNS } from "./leadsListColumns";
import type { Lead, Stage } from "@/types";

export function Board() {
  const { data: pipelines, isLoading: plLoading } = usePipelines();
  const [pipelineId, setPipelineId] = useState<number | undefined>();
  useEffect(() => {
    if (!pipelineId && pipelines && pipelines.length) setPipelineId(pipelines[0].id);
  }, [pipelines, pipelineId]);

  const { data: apiViews, isLoading: viewsLoading } = useSavedLeadViews("board");
  const views = useMemo(() => mergeViews(apiViews, "board"), [apiViews]);
  const { activeId, isLoading: activeLoading } = useActiveViewId("board");
  const activeView = getViewById(views, activeId);
  const viewApplied = useRef(false);

  const [conditions, setConditions] = useState<FilterCondition[]>([]);
  const [cardFields, setCardFields] = useState<string[]>(DEFAULT_BOARD_CARD_FIELDS);
  const [sort, setSort] = useState("created_at");
  const [sortDir, setSortDir] = useState<"asc" | "desc">("desc");
  const [colsOpen, setColsOpen] = useState(false);
  const [filtersExpanded, setFiltersExpanded] = useState(false);

  const applyView = useCallback((view: SavedLeadView) => {
    setConditions([...view.filters]);
    setCardFields(boardCardFields(view.columns));
    setSort(view.sort ?? "created_at");
    setSortDir(view.sort_dir ?? "desc");
  }, []);

  useEffect(() => {
    if (viewsLoading || activeLoading || viewApplied.current) return;
    applyView(activeView);
    viewApplied.current = true;
  }, [viewsLoading, activeLoading, activeView, applyView]);

  const viewChanged = !viewStateEqual(activeView, {
    filters: conditions,
    columns: cardFields,
    sort,
    sort_dir: sortDir,
  });

  const leadFilters = useMemo(
    () => ({
      pipeline_id: pipelineId,
      all: true as const,
      view_id: viewChanged ? undefined : activeId,
      filters: viewChanged ? JSON.stringify(conditions) : undefined,
      sort,
      sort_dir: sortDir,
    }),
    [pipelineId, viewChanged, activeId, conditions, sort, sortDir]
  );

  const { data: stages } = useStages(pipelineId);
  const { data: leads, isLoading, isError, error } = useLeads(leadFilters);
  const { data: customFields } = useCustomFields();
  const changeStage = useChangeStage();
  const openDetail = useUIStore((s) => s.openDetail);

  const allColumnIds = useMemo(() => {
    const custom = (customFields ?? [])
      .filter((f) => f.is_active)
      .map((f) => `custom_${f.id}`);
    return [
      ...SYSTEM_COLUMNS.map((c) => c.id),
      ...PIPELINE_COLUMNS.filter((c) => c.id !== "position").map((c) => c.id),
      ...custom,
    ];
  }, [customFields]);

  const activeCardFields = cardFields.filter((id) => allColumnIds.includes(id));

  const [board, setBoard] = useState<Record<number, Lead[]>>({});
  useEffect(() => {
    if (!leads?.items) return;
    const grouped: Record<number, Lead[]> = {};
    for (const l of leads.items) {
      const sid = l.stage_id ?? 0;
      (grouped[sid] ??= []).push(l);
    }
    setBoard(grouped);
  }, [leads]);

  const leadItems = leads?.items ?? [];

  const [prompt, setPrompt] = useState<{ leadId: number; stage: Stage } | null>(null);

  const stageList = useMemo(
    () => [...(stages ?? [])].sort((a, b) => a.position - b.position),
    [stages]
  );

  function revert() {
    if (!leadItems.length) return;
    const grouped: Record<number, Lead[]> = {};
    for (const l of leadItems) {
      const sid = l.stage_id ?? 0;
      (grouped[sid] ??= []).push(l);
    }
    setBoard(grouped);
  }

  function moveLocal(leadId: number, fromStage: number, toStage: number) {
    setBoard((prev) => {
      const next: Record<number, Lead[]> = {};
      for (const [k, v] of Object.entries(prev)) next[Number(k)] = [...v];
      const fromArr = next[fromStage] ?? [];
      const idx = fromArr.findIndex((l) => l.id === leadId);
      if (idx === -1) return prev;
      const [lead] = fromArr.splice(idx, 1);
      (next[toStage] ??= []).unshift({ ...lead, stage_id: toStage });
      return next;
    });
  }

  function commit(leadId: number, stage: Stage, extra?: PromptResult) {
    changeStage.mutate(
      { leadId, payload: { stage_id: stage.id, ...extra } },
      {
        onError: (err) => {
          const e = apiError(err);
          if (e.code === "business_rule" && (stage.prompt_action_datetime || stage.prompt_disqualification)) {
            setPrompt({ leadId, stage });
          } else {
            toast.error(e.message);
            revert();
          }
        },
      }
    );
  }

  function onDragEnd(result: DropResult) {
    const { source, destination, draggableId } = result;
    if (!destination) return;
    const fromStage = Number(source.droppableId);
    const toStage = Number(destination.droppableId);
    if (fromStage === toStage) return;
    const leadId = Number(draggableId);
    const stage = stageList.find((s) => s.id === toStage);
    if (!stage) return;

    moveLocal(leadId, fromStage, toStage);

    if (stage.prompt_action_datetime || stage.prompt_disqualification) {
      setPrompt({ leadId, stage });
      return;
    }
    commit(leadId, stage);
  }

  if (plLoading || isLoading || viewsLoading || activeLoading) {
    return (
      <div className="flex justify-center py-16">
        <Spinner className="h-6 w-6" />
      </div>
    );
  }

  if (isError) {
    return <EmptyState title="Could not load leads." subtitle={apiError(error).message} />;
  }

  if (!pipelines || pipelines.length === 0) {
    return <EmptyState title="No pipelines yet. Create one in Pipelines." />;
  }

  return (
    <div className="flex h-full flex-col">
      <div className="relative z-10 mb-4 flex flex-wrap items-center gap-2 px-8 pt-5">
        <LeadViewsMenu
          placement="board"
          filters={conditions}
          columns={cardFields}
          sort={sort}
          sortDir={sortDir}
          onFiltersChange={setConditions}
          onViewApply={applyView}
        />
        <BoardSortPicker
          sort={sort}
          sortDir={sortDir}
          customFields={customFields ?? []}
          onSortChange={setSort}
          onSortDirChange={setSortDir}
        />
        <LeadsColumnPicker
          open={colsOpen}
          onOpenChange={setColsOpen}
          visibleCols={activeCardFields}
          allColumnIds={allColumnIds}
          customFields={customFields ?? []}
          onChange={setCardFields}
          label="Card fields"
        />
        <Button variant="ghost" size="sm" onClick={() => setFiltersExpanded((e) => !e)}>
          {filtersExpanded ? "Hide filters" : "Edit filters"}
        </Button>
        <FilterSelect
          value={pipelineId ?? ""}
          onChange={(e) => setPipelineId(Number(e.target.value))}
          className="ml-auto w-56"
        >
          {pipelines.map((p) => (
            <option key={p.id} value={p.id}>
              {p.name}
            </option>
          ))}
        </FilterSelect>
      </div>

      {filtersExpanded && (
        <div className="relative z-10 mx-8 mb-4 rounded-lg border border-gray-100 bg-gray-50/50 p-4">
          <LeadFilterBuilder conditions={conditions} onChange={setConditions} />
        </div>
      )}

      <DragDropContext onDragEnd={onDragEnd}>
        <div className="relative z-0 flex flex-1 gap-3 overflow-x-auto px-8 pb-8">
          {stageList.map((stage) => {
            const items = board[stage.id] ?? [];
            return (
              <Droppable droppableId={String(stage.id)} key={stage.id}>
                {(provided, snapshot) => (
                  <div
                    ref={provided.innerRef}
                    {...provided.droppableProps}
                    className={cn(
                      "flex w-[280px] shrink-0 flex-col rounded-lg bg-gray-50",
                      snapshot.isDraggingOver && "ring-2 ring-jade-400/40"
                    )}
                  >
                    <div className="flex items-center gap-2 border-b border-gray-100 px-3.5 py-2.5">
                      <span className={cn("h-2 w-2 shrink-0 rounded-full", stageColorDot(stage.color))} />
                      <span className="flex-1 text-base font-semibold text-gray-700">
                        {stage.name}
                      </span>
                      <span className="text-xs text-gray-400">{items.length}</span>
                    </div>
                    <div className="relative flex flex-1 flex-col gap-2 p-2 pl-3">
                      <span
                        aria-hidden
                        className={cn(
                          "pointer-events-none absolute bottom-0 left-0 top-0 w-px",
                          stageColorLine(stage.color)
                        )}
                      />
                      {items.map((lead, i) => (
                        <Draggable draggableId={String(lead.id)} index={i} key={lead.id}>
                          {(p, snap) => (
                            <div ref={p.innerRef} {...p.draggableProps} {...p.dragHandleProps}>
                              <LeadCard
                                lead={lead}
                                customFields={customFields ?? []}
                                cardFields={activeCardFields}
                                onClick={() => openDetail(lead.id)}
                                dragging={snap.isDragging}
                              />
                            </div>
                          )}
                        </Draggable>
                      ))}
                      {provided.placeholder}
                    </div>
                  </div>
                )}
              </Droppable>
            );
          })}
        </div>
      </DragDropContext>

      <StagePromptModal
        open={!!prompt}
        stage={prompt?.stage ?? null}
        onCancel={() => {
          setPrompt(null);
          revert();
        }}
        onConfirm={(r) => {
          if (prompt) commit(prompt.leadId, prompt.stage, r);
          setPrompt(null);
        }}
      />
    </div>
  );
}
