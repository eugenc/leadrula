import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { get } from "@/lib/api";
import type { Me } from "@/types";
import {
  DndContext,
  DragOverlay,
  PointerSensor,
  closestCenter,
  useSensor,
  useSensors,
  type DragEndEvent,
  type DragStartEvent,
} from "@dnd-kit/core";
import {
  usePipelines,
  useStages,
  useLeads,
  useChangeStage,
  useCustomFields,
} from "./hooks";
import { BoardSortPicker } from "./BoardSortPicker";
import { BoardColumn } from "./BoardColumn";
import {
  initialActionAtForStageMove,
  stageNeedsPrompt,
  stagePromptMissingError,
} from "@/features/pipelines/stageTypes";
import { LeadCard } from "./LeadCard";
import { LeadsColumnPicker } from "./LeadsColumnPicker";
import { StagePromptModal, type PromptResult } from "./StagePromptModal";
import { LeadFilterBuilder } from "./LeadFilterBuilder";
import { LeadSearchInput } from "./LeadSearchInput";
import { LeadViewsMenu } from "./LeadViewsMenu";
import {
  useSavedLeadViews,
  useActiveViewId,
  mergeViews,
  getViewById,
  filtersViewChanged,
  type FilterCondition,
  type SavedLeadView,
} from "./leadsViews";
import { loadBoardUi, saveBoardUi, defaultBoardUi } from "./leadsUiStorage";
import { FilterSelect } from "@/components/ui/input";
import { Button } from "@/components/ui/button";
import { Spinner, EmptyState } from "@/components/ui/misc";
import { useUIStore } from "@/store/uiStore";
import { apiError, errorMessage } from "@/lib/api";
import { toast } from "@/store/toastStore";
import {
  SYSTEM_COLUMNS,
  DEFAULT_BOARD_CARD_FIELDS,
  normalizeBoardCardFields,
  parseBoardCardFields,
  BOARD_CARD_FIELDS_PREF_KEY,
  PIPELINE_COLUMNS,
} from "./leadsListColumns";
import { useAuthStore } from "@/store/authStore";
import { groupLeadsForBoard, isPublisherTrackedLead, UNPLACED_BOARD_STAGE_ID } from "./boardStage";
import type { AccountType, Lead, Stage } from "@/types";

const unplacedStage: Stage = {
  id: UNPLACED_BOARD_STAGE_ID,
  public_id: "unplaced",
  pipeline_id: 0,
  name: "Unplaced",
  position: -1,
  color: "gray",
  stage_type: "standard",
};

function isDragBlocked(lead: Lead, accountType: AccountType | undefined): boolean {
  return accountType === "publisher" && isPublisherTrackedLead(lead);
}

function resolveDropStage(over: DragEndEvent["over"]): number | null {
  if (!over) return null;
  const stageId = over.data.current?.stageId;
  if (stageId != null) return Number(stageId);
  const n = Number(over.id);
  return Number.isFinite(n) ? n : null;
}

function estimateRowHeight(cardFieldCount: number): number {
  return 88 + Math.max(0, cardFieldCount - 1) * 22;
}

export function Board() {
  const user = useAuthStore((s) => s.user);
  const accountType = user?.account_type;
  const { data: pipelines, isLoading: plLoading } = usePipelines();
  const [pipelineId, setPipelineId] = useState<number | undefined>();
  useEffect(() => {
    if (!pipelineId && pipelines && pipelines.length) setPipelineId(pipelines[0].id);
  }, [pipelines, pipelineId]);

  const { data: apiViews, isLoading: viewsLoading } = useSavedLeadViews("board");
  const views = useMemo(() => mergeViews(apiViews, "board"), [apiViews]);
  const { activeId, isLoading: activeLoading } = useActiveViewId("board");
  const { data: me } = useQuery({
    queryKey: ["me"],
    queryFn: () => get<Me>("/auth/me"),
  });
  const activeView = getViewById(views, activeId);
  const prevActiveId = useRef<string | null>(null);
  const uiHydrated = useRef(false);
  const [uiReady, setUiReady] = useState(false);

  const [conditions, setConditions] = useState<FilterCondition[]>([]);
  const [cardFields, setCardFields] = useState<string[]>(DEFAULT_BOARD_CARD_FIELDS);
  const [sort, setSort] = useState("created_at");
  const [sortDir, setSortDir] = useState<"asc" | "desc">("desc");
  const [colsOpen, setColsOpen] = useState(false);
  const [filtersExpanded, setFiltersExpanded] = useState(false);
  const [searchTerm, setSearchTerm] = useState("");
  const [debouncedSearch, setDebouncedSearch] = useState("");

  const { data: customFields, isLoading: customFieldsLoading } = useCustomFields();

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

  const updateCardFields = useCallback(
    (cols: string[]) => {
      const normalized = normalizeBoardCardFields(cols, allColumnIds);
      setCardFields(normalized);
      if (user?.id) saveBoardUi(user.id, { card_fields: normalized });
    },
    [allColumnIds, user?.id]
  );

  const updateSort = useCallback(
    (nextSort: string) => {
      setSort(nextSort);
      if (user?.id) saveBoardUi(user.id, { sort: nextSort });
    },
    [user?.id]
  );

  const updateSortDir = useCallback(
    (nextDir: "asc" | "desc") => {
      setSortDir(nextDir);
      if (user?.id) saveBoardUi(user.id, { sort_dir: nextDir });
    },
    [user?.id]
  );

  const applyViewFilters = useCallback((view: SavedLeadView) => {
    setConditions([...view.filters]);
    setSearchTerm("");
    setDebouncedSearch("");
  }, []);

  useEffect(() => {
    if (viewsLoading || activeLoading) return;
    if (prevActiveId.current === activeId) return;
    prevActiveId.current = activeId;
    applyViewFilters(getViewById(views, activeId));
  }, [activeId, viewsLoading, activeLoading, views, applyViewFilters]);

  useEffect(() => {
    if (viewsLoading || activeLoading || customFieldsLoading || uiHydrated.current || !user?.id) return;
    const legacyCardFields = parseBoardCardFields(me?.user.prefs?.[BOARD_CARD_FIELDS_PREF_KEY]);
    const stored =
      loadBoardUi(user.id, allColumnIds, legacyCardFields) ?? defaultBoardUi(allColumnIds);
    setSort(stored.sort);
    setSortDir(stored.sort_dir);
    setCardFields(stored.card_fields);
    uiHydrated.current = true;
    setUiReady(true);
  }, [
    viewsLoading,
    activeLoading,
    customFieldsLoading,
    user?.id,
    allColumnIds,
    me?.user.prefs,
  ]);

  const filtersChanged = filtersViewChanged(activeView, conditions);

  useEffect(() => {
    const t = setTimeout(() => setDebouncedSearch(searchTerm.trim()), 300);
    return () => clearTimeout(t);
  }, [searchTerm]);

  const leadFilters = useMemo(
    () => ({
      pipeline_id: pipelineId,
      all: true as const,
      view_id: filtersChanged ? undefined : activeId,
      filters: filtersChanged ? JSON.stringify(conditions) : undefined,
      q: debouncedSearch || undefined,
      sort,
      sort_dir: sortDir,
    }),
    [pipelineId, filtersChanged, activeId, conditions, debouncedSearch, sort, sortDir]
  );

  const { data: stages } = useStages(pipelineId);
  const { data: leads, isLoading, isError, error } = useLeads(leadFilters);
  const changeStage = useChangeStage();
  const openDetail = useUIStore((s) => s.openDetail);

  const activeCardFields = cardFields.filter((id) => allColumnIds.includes(id));
  const rowHeight = estimateRowHeight(activeCardFields.filter((id) => id !== "name").length);

  const [board, setBoard] = useState<Record<number, Lead[]>>({});
  const [unplacedLeads, setUnplacedLeads] = useState<Lead[]>([]);
  const [prompt, setPrompt] = useState<{
    leadId: number;
    stage: Stage;
    initialActionAt: string;
  } | null>(null);
  const [activeDrag, setActiveDrag] = useState<Lead | null>(null);

  const stageList = useMemo(
    () => [...(stages ?? [])].sort((a, b) => a.position - b.position),
    [stages]
  );

  const pipelineStageIds = useMemo(() => new Set(stageList.map((s) => s.id)), [stageList]);

  useEffect(() => {
    const { grouped, unplaced } = groupLeadsForBoard(leads?.items ?? [], pipelineStageIds, accountType);
    setBoard(grouped);
    setUnplacedLeads(unplaced);
  }, [leads?.items, accountType, pipelineStageIds]);

  const sensors = useSensors(
    useSensor(PointerSensor, { activationConstraint: { distance: 8 } })
  );

  function revert() {
    const { grouped, unplaced } = groupLeadsForBoard(leads?.items ?? [], pipelineStageIds, accountType);
    setBoard(grouped);
    setUnplacedLeads(unplaced);
  }

  function moveLocal(leadId: number, fromStage: number, toStage: number) {
    if (fromStage === UNPLACED_BOARD_STAGE_ID) {
      setUnplacedLeads((prev) => {
        const idx = prev.findIndex((l) => l.id === leadId);
        if (idx === -1) return prev;
        const lead = prev[idx];
        setBoard((b) => ({
          ...b,
          [toStage]: [{ ...lead, stage_id: toStage }, ...(b[toStage] ?? [])],
        }));
        return [...prev.slice(0, idx), ...prev.slice(idx + 1)];
      });
      return;
    }
    setBoard((prev) => {
      const fromArr = prev[fromStage] ?? [];
      const idx = fromArr.findIndex((l) => l.id === leadId);
      if (idx === -1) return prev;
      const lead = fromArr[idx];
      return {
        ...prev,
        [fromStage]: [...fromArr.slice(0, idx), ...fromArr.slice(idx + 1)],
        [toStage]: [{ ...lead, stage_id: toStage }, ...(prev[toStage] ?? [])],
      };
    });
  }

  function commit(leadId: number, stage: Stage, extra?: PromptResult) {
    changeStage.mutate(
      { leadId, payload: { stage_id: stage.id, ...extra } },
      {
        onSuccess: () => setPrompt(null),
        onError: (err) => {
          const e = apiError(err);
          if (stagePromptMissingError(e.code, e.message, stage.stage_type)) {
            const lead = leads?.items.find((l) => l.id === leadId);
            const fromStage = stageList.find((s) => s.id === lead?.stage_id);
            setPrompt({
              leadId,
              stage,
              initialActionAt: initialActionAtForStageMove(
                fromStage?.stage_type,
                stage.stage_type,
                lead?.action_at
              ),
            });
          } else {
            toast.error(errorMessage(err));
            revert();
          }
        },
      }
    );
  }

  function onDragStart(event: DragStartEvent) {
    const lead = event.active.data.current?.lead as Lead | undefined;
    if (lead && isDragBlocked(lead, accountType)) return;
    if (lead) setActiveDrag(lead);
  }

  function onDragEnd(event: DragEndEvent) {
    setActiveDrag(null);
    const lead = event.active.data.current?.lead as Lead | undefined;
    if (lead && isDragBlocked(lead, accountType)) return;
    const { active, over } = event;
    if (!over) return;

    const fromStage = Number(active.data.current?.stageId);
    const toStage = resolveDropStage(over);
    if (fromStage === UNPLACED_BOARD_STAGE_ID) {
      if (!toStage || toStage === UNPLACED_BOARD_STAGE_ID) return;
    } else if (!fromStage || !toStage || fromStage === toStage || toStage === UNPLACED_BOARD_STAGE_ID) {
      return;
    }

    const leadId = Number(active.id);
    const stage = stageList.find((s) => s.id === toStage);
    if (!stage) return;

    moveLocal(leadId, fromStage, toStage);

    if (stageNeedsPrompt(stage.stage_type)) {
      const fromStageType = stageList.find((s) => s.id === fromStage)?.stage_type;
      setPrompt({
        leadId,
        stage,
        initialActionAt: initialActionAtForStageMove(fromStageType, stage.stage_type, lead?.action_at),
      });
      return;
    }
    commit(leadId, stage);
  }

  const customFieldsList = customFields ?? [];

  if (plLoading || isLoading || viewsLoading || activeLoading || customFieldsLoading || !uiReady) {
    return (
      <div className="flex justify-center py-16">
        <Spinner className="h-6 w-6" />
      </div>
    );
  }

  if (isError) {
    return <EmptyState title="Could not load leads." subtitle={errorMessage(error)} />;
  }

  if (!pipelines || pipelines.length === 0) {
    return <EmptyState title="No pipelines yet. Create one in Pipelines." />;
  }

  return (
    <div className="flex h-full min-h-0 flex-col">
      <div className="relative z-10 mb-4 flex shrink-0 flex-wrap items-center gap-2 px-8 pt-5">
        <FilterSelect
          value={pipelineId ?? ""}
          onChange={(e) => setPipelineId(Number(e.target.value))}
          className="w-56"
        >
          {pipelines.map((p) => (
            <option key={p.id} value={p.id}>
              {p.name}
            </option>
          ))}
        </FilterSelect>
        <LeadSearchInput
          value={searchTerm}
          onChange={setSearchTerm}
          className="w-72"
          inputClassName="h-7 text-sm"
          leadFilters={leadFilters}
          onSelectLead={(lead) => openDetail(lead.id)}
        />
        <LeadViewsMenu
          placement="board"
          filters={conditions}
          columns={cardFields}
          sort={sort}
          sortDir={sortDir}
          onFiltersChange={setConditions}
          onViewApply={applyViewFilters}
        />
        <BoardSortPicker
          sort={sort}
          sortDir={sortDir}
          customFields={customFieldsList}
          onSortChange={updateSort}
          onSortDirChange={updateSortDir}
        />
        <LeadsColumnPicker
          open={colsOpen}
          onOpenChange={setColsOpen}
          visibleCols={activeCardFields}
          allColumnIds={allColumnIds}
          customFields={customFieldsList}
          defaultCols={DEFAULT_BOARD_CARD_FIELDS}
          lockedCols={DEFAULT_BOARD_CARD_FIELDS}
          onChange={updateCardFields}
          label="Card fields"
        />
        <Button variant="outline" size="sm" onClick={() => setFiltersExpanded((e) => !e)}>
          {filtersExpanded ? "Hide filters" : "Edit filters"}
        </Button>
      </div>

      {filtersExpanded && (
        <div className="relative z-10 mx-8 mb-4 shrink-0 rounded-lg border border-gray-100 bg-gray-50/50 p-4">
          <LeadFilterBuilder conditions={conditions} onChange={setConditions} />
        </div>
      )}

      <DndContext
        sensors={sensors}
        collisionDetection={closestCenter}
        onDragStart={onDragStart}
        onDragEnd={onDragEnd}
        onDragCancel={() => setActiveDrag(null)}
      >
        <div className="relative z-0 flex h-full min-h-0 flex-1 gap-3 overflow-x-auto px-8 pb-8">
          {stageList.map((stage) => (
            <BoardColumn
              key={stage.id}
              stage={stage}
              items={board[stage.id] ?? []}
              customFields={customFieldsList}
              cardFields={activeCardFields}
              rowHeight={rowHeight}
              onCardClick={openDetail}
              activeDragId={activeDrag ? String(activeDrag.id) : null}
              accountType={accountType}
            />
          ))}
          {unplacedLeads.length > 0 && (
            <BoardColumn
              key={unplacedStage.id}
              stage={unplacedStage}
              items={unplacedLeads}
              customFields={customFieldsList}
              cardFields={activeCardFields}
              rowHeight={rowHeight}
              onCardClick={openDetail}
              activeDragId={activeDrag ? String(activeDrag.id) : null}
              accountType={accountType}
              droppable={false}
            />
          )}
        </div>
        <DragOverlay dropAnimation={null}>
          {activeDrag ? (
            <LeadCard
              lead={activeDrag}
              customFields={customFieldsList}
              cardFields={activeCardFields}
              onClick={() => {}}
              dragging
            />
          ) : null}
        </DragOverlay>
      </DndContext>

      <StagePromptModal
        key={prompt ? `${prompt.leadId}-${prompt.stage.id}` : "closed"}
        open={!!prompt}
        stage={prompt?.stage ?? null}
        initialActionAt={prompt?.initialActionAt}
        onCancel={() => {
          setPrompt(null);
          revert();
        }}
        onConfirm={(r) => {
          if (prompt) commit(prompt.leadId, prompt.stage, r);
        }}
      />
    </div>
  );
}
