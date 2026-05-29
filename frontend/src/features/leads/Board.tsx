import { useEffect, useMemo, useState } from "react";
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
import { LeadCard } from "./LeadCard";
import { StagePromptModal, type PromptResult } from "./StagePromptModal";
import { Select } from "@/components/ui/input";
import { Spinner, EmptyState } from "@/components/ui/misc";
import { useUIStore } from "@/store/uiStore";
import { apiError } from "@/lib/api";
import { toast } from "@/store/toastStore";
import type { Lead, Stage } from "@/types";

export function Board() {
  const { data: pipelines, isLoading: plLoading } = usePipelines();
  const [pipelineId, setPipelineId] = useState<number | undefined>();
  useEffect(() => {
    if (!pipelineId && pipelines && pipelines.length) setPipelineId(pipelines[0].id);
  }, [pipelines, pipelineId]);

  const { data: stages } = useStages(pipelineId);
  const { data: leads, isLoading } = useLeads(pipelineId ? { pipeline_id: pipelineId } : {});
  const { data: customFields } = useCustomFields();
  const changeStage = useChangeStage();
  const openDetail = useUIStore((s) => s.openDetail);

  // local optimistic board state keyed by stage id
  const [board, setBoard] = useState<Record<number, Lead[]>>({});
  useEffect(() => {
    if (!leads) return;
    const grouped: Record<number, Lead[]> = {};
    for (const l of leads) {
      const sid = l.stage_id ?? 0;
      (grouped[sid] ??= []).push(l);
    }
    setBoard(grouped);
  }, [leads]);

  const [prompt, setPrompt] = useState<{ leadId: number; stage: Stage } | null>(null);

  const stageList = useMemo(
    () => [...(stages ?? [])].sort((a, b) => a.position - b.position),
    [stages]
  );

  function revert() {
    if (!leads) return;
    const grouped: Record<number, Lead[]> = {};
    for (const l of leads) {
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

  if (plLoading || isLoading) {
    return (
      <div className="flex justify-center py-16">
        <Spinner className="h-6 w-6" />
      </div>
    );
  }

  if (!pipelines || pipelines.length === 0) {
    return <EmptyState title="No pipelines yet. Create one in Pipelines." />;
  }

  return (
    <div className="flex h-full flex-col">
      <div className="mb-4 flex items-center gap-3">
        <Select
          value={pipelineId ?? ""}
          onChange={(e) => setPipelineId(Number(e.target.value))}
          className="w-56"
        >
          {pipelines.map((p) => (
            <option key={p.id} value={p.id}>
              {p.name}
            </option>
          ))}
        </Select>
      </div>

      <DragDropContext onDragEnd={onDragEnd}>
        <div className="flex flex-1 gap-3 overflow-x-auto pb-4">
          {stageList.map((stage) => {
            const items = board[stage.id] ?? [];
            return (
              <Droppable droppableId={String(stage.id)} key={stage.id}>
                {(provided, snapshot) => (
                  <div
                    ref={provided.innerRef}
                    {...provided.droppableProps}
                    className={`flex w-72 shrink-0 flex-col rounded-lg bg-pd-stage p-2 ${
                      snapshot.isDraggingOver ? "ring-2 ring-pd-blue/40" : ""
                    }`}
                  >
                    <div className="mb-2 flex items-center justify-between px-1">
                      <span className="text-sm font-semibold text-pd-text">{stage.name}</span>
                      <span className="rounded bg-white px-1.5 text-xs font-semibold text-pd-muted">
                        {items.length}
                      </span>
                    </div>
                    <div className="flex flex-1 flex-col gap-2">
                      {items.map((lead, i) => (
                        <Draggable draggableId={String(lead.id)} index={i} key={lead.id}>
                          {(p, snap) => (
                            <div
                              ref={p.innerRef}
                              {...p.draggableProps}
                              {...p.dragHandleProps}
                            >
                              <LeadCard
                                lead={lead}
                                customFields={customFields ?? []}
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
