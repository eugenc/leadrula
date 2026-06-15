import { useCallback, useEffect, useMemo, useRef, useState } from "react";
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
import { stageNeedsPrompt } from "@/features/pipelines/stageTypes";
import { LeadCard } from "./LeadCard";
import { LeadsColumnPicker } from "./LeadsColumnPicker";
import { StagePromptModal, type PromptResult } from "./StagePromptModal";
import { LeadFilterBuilder } from "./LeadFilterBuilder";
import { LeadViewsMenu } from "./LeadViewsMenu";
import {
  useSavedLeadViews,
  useActiveViewId,
  useBoardCardFields,
  mergeViews,
  getViewById,
  viewStateEqual,
  type FilterCondition,
  type SavedLeadView,
} from "./leadsViews";
import { FilterSelect, Input } from "@/components/ui/input";
import { Button } from "@/components/ui/button";
import { Spinner, EmptyState } from "@/components/ui/misc";
import { useUIStore } from "@/store/uiStore";
import { apiError, errorMessage } from "@/lib/api";
import { toast } from "@/store/toastStore";
import {
  SYSTEM_COLUMNS,
  boardCardFields,
  DEFAULT_BOARD_CARD_FIELDS,
  normalizeBoardCardFields,
  resolveBoardCardFields,
  PIPELINE_COLUMNS,
} from "./leadsListColumns";
import { useAuthStore } from "@/store/authStore";
import type { Lead, Stage } from "@/types";

function boardStageId(lead: Lead): number {
  return lead.board_stage_id ?? lead.stage_id ?? 0;
}

function isTrackedLead(lead: Lead, accountId: string | undefined): boolean {
  if (!accountId) return false;
  return lead.owner_account_id !== Number(accountId) && lead.publisher_id === Number(accountId);
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
  const accountId = useAuthStore((s) => s.user?.account_id);
  const { data: pipelines, isLoading: plLoading } = usePipelines();
  const [pipelineId, setPipelineId] = useState<number | undefined>();
  useEffect(() => {
    if (!pipelineId && pipelines && pipelines.length) setPipelineId(pipelines[0].id);
  }, [pipelines, pipelineId]);

  const { data: apiViews, isLoading: viewsLoading } = useSavedLeadViews("board");
  const views = useMemo(() => mergeViews(apiViews, "board"), [apiViews]);
  const { activeId, isLoading: activeLoading } = useActiveViewId("board");
  const { savedCardFields, saveCardFields, isLoading: cardFieldsLoading } = useBoardCardFields();
  const activeView = getViewById(views, activeId);
  const prevActiveId = useRef<string | null>(null);
  const cardFieldsHydrated = useRef(false);

  const [conditions, setConditions] = useState<FilterCondition[]>([]);
  const [cardFields, setCardFields] = useState<string[]>(DEFAULT_BOARD_CARD_FIELDS);
  const [sort, setSort] = useState("created_at");
  const [sortDir, setSortDir] = useState<"asc" | "desc">("desc");
  const [colsOpen, setColsOpen] = useState(false);
  const [filtersExpanded, setFiltersExpanded] = useState(false);
  const [searchTerm, setSearchTerm] = useState("");
  const [debouncedSearch, setDebouncedSearch] = useState("");

  const { data: customFields } = useCustomFields();

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
      saveCardFields(normalized);
    },
    [allColumnIds, saveCardFields]
  );

  const applyView = useCallback(
    (view: SavedLeadView, resetCardFields = false) => {
      setConditions([...view.filters]);
      setSort(view.sort ?? "created_at");
      setSortDir(view.sort_dir ?? "desc");
      setSearchTerm("");
      setDebouncedSearch("");
      if (resetCardFields) {
        updateCardFields(boardCardFields(view.columns));
      }
    },
    [updateCardFields]
  );

  useEffect(() => {
    if (viewsLoading || activeLoading) return;
    if (prevActiveId.current === activeId) return;
    const isInitial = prevActiveId.current === null;
    prevActiveId.current = activeId;
    applyView(getViewById(views, activeId), !isInitial);
  }, [activeId, viewsLoading, activeLoading, views, applyView]);

  useEffect(() => {
    if (cardFieldsLoading || cardFieldsHydrated.current) return;
    setCardFields(
      savedCardFields
        ? normalizeBoardCardFields(savedCardFields, allColumnIds)
        : resolveBoardCardFields(undefined)
    );
    cardFieldsHydrated.current = true;
  }, [cardFieldsLoading, savedCardFields, allColumnIds]);

  const viewChanged = !viewStateEqual(activeView, {
    filters: conditions,
    columns: cardFields,
    sort,
    sort_dir: sortDir,
  });

  useEffect(() => {
    const t = setTimeout(() => setDebouncedSearch(searchTerm.trim()), 300);
    return () => clearTimeout(t);
  }, [searchTerm]);

  const leadFilters = useMemo(
    () => ({
      pipeline_id: pipelineId,
      all: true as const,
      view_id: viewChanged ? undefined : activeId,
      filters: viewChanged ? JSON.stringify(conditions) : undefined,
      q: debouncedSearch || undefined,
      sort,
      sort_dir: sortDir,
    }),
    [pipelineId, viewChanged, activeId, conditions, debouncedSearch, sort, sortDir]
  );

  const { data: stages } = useStages(pipelineId);
  const { data: leads, isLoading, isError, error } = useLeads(leadFilters);
  const changeStage = useChangeStage();
  const openDetail = useUIStore((s) => s.openDetail);

  const activeCardFields = cardFields.filter((id) => allColumnIds.includes(id));
  const rowHeight = estimateRowHeight(activeCardFields.filter((id) => id !== "name").length);

  const [board, setBoard] = useState<Record<number, Lead[]>>({});
  useEffect(() => {
    const grouped: Record<number, Lead[]> = {};
    for (const l of leads?.items ?? []) {
      const sid = boardStageId(l);
      (grouped[sid] ??= []).push(l);
    }
    setBoard(grouped);
  }, [leads?.items]);

  const [prompt, setPrompt] = useState<{ leadId: number; stage: Stage } | null>(null);
  const [activeDrag, setActiveDrag] = useState<Lead | null>(null);

  const stageList = useMemo(
    () => [...(stages ?? [])].sort((a, b) => a.position - b.position),
    [stages]
  );

  const sensors = useSensors(
    useSensor(PointerSensor, { activationConstraint: { distance: 8 } })
  );

  function revert() {
    const grouped: Record<number, Lead[]> = {};
    for (const l of leads?.items ?? []) {
      const sid = boardStageId(l);
      (grouped[sid] ??= []).push(l);
    }
    setBoard(grouped);
  }

  function moveLocal(leadId: number, fromStage: number, toStage: number) {
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
        onError: (err) => {
          const e = apiError(err);
          if (e.code === "business_rule" && stageNeedsPrompt(stage.stage_type)) {
            setPrompt({ leadId, stage });
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
    if (lead && isTrackedLead(lead, accountId)) return;
    if (lead) setActiveDrag(lead);
  }

  function onDragEnd(event: DragEndEvent) {
    setActiveDrag(null);
    const lead = event.active.data.current?.lead as Lead | undefined;
    if (lead && isTrackedLead(lead, accountId)) return;
    const { active, over } = event;
    if (!over) return;

    const fromStage = Number(active.data.current?.stageId);
    const toStage = resolveDropStage(over);
    if (!fromStage || !toStage || fromStage === toStage) return;

    const leadId = Number(active.id);
    const stage = stageList.find((s) => s.id === toStage);
    if (!stage) return;

    moveLocal(leadId, fromStage, toStage);

    if (stageNeedsPrompt(stage.stage_type)) {
      setPrompt({ leadId, stage });
      return;
    }
    commit(leadId, stage);
  }

  const customFieldsList = customFields ?? [];

  if (plLoading || isLoading || viewsLoading || activeLoading || cardFieldsLoading) {
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
        <Input
          type="search"
          placeholder="Search name, email, phone, address, buyer, or status…"
          value={searchTerm}
          onChange={(e) => setSearchTerm(e.target.value)}
          className="h-7 w-72 text-sm"
        />
        <LeadViewsMenu
          placement="board"
          filters={conditions}
          columns={cardFields}
          sort={sort}
          sortDir={sortDir}
          onFiltersChange={setConditions}
          onViewApply={(view) => applyView(view, true)}
        />
        <BoardSortPicker
          sort={sort}
          sortDir={sortDir}
          customFields={customFieldsList}
          onSortChange={setSort}
          onSortDirChange={setSortDir}
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
              accountId={accountId}
            />
          ))}
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
