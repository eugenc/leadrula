import { useEffect, useState } from "react";
import {
  DragDropContext,
  Droppable,
  Draggable,
  type DropResult,
} from "@hello-pangea/dnd";
import { usePipelines, useStages } from "@/features/leads/hooks";
import {
  useCreatePipeline,
  useDeletePipeline,
  useUpdatePipeline,
  useCreateStage,
  useUpdateStage,
  useDeleteStage,
  useReorderStages,
} from "@/features/admin/hooks";
import { StageSettingsDrawer } from "@/features/pipelines/StageSettingsDrawer";
import { stageColorDot } from "@/features/pipelines/stageColors";
import { PageBody } from "@/components/layout/PageBody";
import { SectionLabel } from "@/components/layout/SectionLabel";
import { IconButton } from "@/components/layout/IconButton";
import { Button } from "@/components/ui/button";
import { Input, Select } from "@/components/ui/input";
import { STAGE_TYPES } from "@/features/pipelines/stageTypes";
import { Card, Spinner, EmptyState } from "@/components/ui/misc";
import { cn } from "@/lib/utils";
import { Plus, Settings2, Trash2, GripVertical } from "lucide-react";
import { toast } from "@/store/toastStore";
import { errorMessage } from "@/lib/api";
import type { Stage, StageType } from "@/types";

export function PipelinesPage() {
  const { data: pipelines, isLoading } = usePipelines();
  const [selected, setSelected] = useState<number | undefined>();
  useEffect(() => {
    if (!selected && pipelines?.length) setSelected(pipelines[0].id);
  }, [pipelines, selected]);

  const createPipeline = useCreatePipeline();
  const deletePipeline = useDeletePipeline();
  const updatePipeline = useUpdatePipeline();
  const [newName, setNewName] = useState("");

  return (
    <>
      <PageBody>
        {isLoading ? (
          <Spinner className="h-6 w-6" />
        ) : (
          <div className="flex gap-6">
            <Card className="w-64 shrink-0 p-2">
              {(pipelines ?? []).map((p) => (
                <div
                  key={p.id}
                  className={cn(
                    "flex items-center gap-1 rounded-md px-2 py-1.5 text-sm",
                    selected === p.id ? "bg-jade-100 text-jade-700" : "hover:bg-gray-100"
                  )}
                >
                  <Input
                    defaultValue={p.name}
                    className="h-8 flex-1 border-0 bg-transparent px-1 text-sm font-medium shadow-none focus-visible:ring-1"
                    onFocus={() => setSelected(p.id)}
                    onBlur={(e) => {
                      const name = e.target.value.trim();
                      if (!name) {
                        e.target.value = p.name;
                        return;
                      }
                      if (name !== p.name) {
                        updatePipeline.mutate(
                          { id: p.id, name },
                          { onError: (err) => toast.error(errorMessage(err)) }
                        );
                      }
                    }}
                  />
                  <IconButton
                    variant="danger"
                    onClick={() =>
                      deletePipeline.mutate(p.id, {
                        onError: (e) => toast.error(errorMessage(e)),
                      })
                    }
                  >
                    <Trash2 className="h-3.5 w-3.5" />
                  </IconButton>
                </div>
              ))}
              <div className="mt-2 flex gap-1 p-1">
                <Input
                  value={newName}
                  onChange={(e) => setNewName(e.target.value)}
                  placeholder="New pipeline"
                  className="h-8 text-sm"
                />
                <Button
                  size="icon"
                  onClick={() =>
                    newName &&
                    createPipeline.mutate(newName, {
                      onSuccess: () => setNewName(""),
                    })
                  }
                >
                  <Plus className="h-4 w-4" />
                </Button>
              </div>
            </Card>

            <div className="flex-1">
              {selected ? (
                <StagesEditor pipelineId={selected} />
              ) : (
                <EmptyState title="Select a pipeline" />
              )}
            </div>
          </div>
        )}
      </PageBody>
    </>
  );
}

function StagesEditor({ pipelineId }: { pipelineId: number }) {
  const { data: stages } = useStages(pipelineId);
  const createStage = useCreateStage();
  const updateStage = useUpdateStage();
  const deleteStage = useDeleteStage();
  const reorderStages = useReorderStages();
  const [name, setName] = useState("");
  const [newStageType, setNewStageType] = useState<StageType>("standard");
  const [settingsStage, setSettingsStage] = useState<Stage | null>(null);

  const sorted = [...(stages ?? [])].sort((a, b) => a.position - b.position);

  function onDragEnd(result: DropResult) {
    if (!result.destination || result.destination.index === result.source.index) return;
    const next = [...sorted];
    const [moved] = next.splice(result.source.index, 1);
    next.splice(result.destination.index, 0, moved!);
    reorderStages.mutate(
      { pipelineId, orderedStageIds: next.map((s) => s.id) },
      { onError: (err) => toast.error(errorMessage(err)) }
    );
  }

  return (
    <>
      <Card className="p-4">
        <div className="mb-3 grid grid-cols-[auto_auto_1fr_9rem_auto_auto] items-center gap-3">
          <span />
          <SectionLabel className="col-span-1"> </SectionLabel>
          <SectionLabel>Stage</SectionLabel>
          <SectionLabel>Type</SectionLabel>
          <SectionLabel>Settings</SectionLabel>
          <span />
        </div>
        <DragDropContext onDragEnd={onDragEnd}>
          <Droppable droppableId="pipeline-stages">
            {(provided) => (
              <div ref={provided.innerRef} {...provided.droppableProps} className="space-y-2">
                {sorted.map((s, index) => (
                  <Draggable key={s.id} draggableId={String(s.id)} index={index}>
                    {(drag, snapshot) => (
                      <div
                        ref={drag.innerRef}
                        {...drag.draggableProps}
                        className={cn(
                          "grid grid-cols-[auto_auto_1fr_9rem_auto_auto] items-center gap-3 rounded-md",
                          snapshot.isDragging ? "bg-jade-50 shadow-sm" : "hover:bg-gray-100"
                        )}
                      >
                        <span
                          {...drag.dragHandleProps}
                          className="cursor-grab px-1 text-gray-400 hover:text-gray-600"
                        >
                          <GripVertical className="h-4 w-4" />
                        </span>
                        <span
                          className={cn("ml-1 h-3 w-3 shrink-0 rounded-full", stageColorDot(s.color))}
                        />
                        <Input
                          defaultValue={s.name}
                          className="h-8 text-sm"
                          onBlur={(e) =>
                            e.target.value !== s.name &&
                            updateStage.mutate({ id: s.id, body: { name: e.target.value } })
                          }
                        />
                        <Select
                          value={s.stage_type}
                          className="h-8 text-sm"
                          onChange={(e) => {
                            const stage_type = e.target.value as StageType;
                            if (stage_type !== s.stage_type) {
                              updateStage.mutate(
                                { id: s.id, body: { stage_type } },
                                { onError: (err) => toast.error(errorMessage(err)) }
                              );
                            }
                          }}
                        >
                          {STAGE_TYPES.map((t) => (
                            <option key={t.value} value={t.value}>
                              {t.label}
                            </option>
                          ))}
                        </Select>
                        <Button
                          size="sm"
                          variant="outline"
                          onClick={() => setSettingsStage(s)}
                          aria-label={`Settings for ${s.name}`}
                        >
                          <Settings2 className="h-3.5 w-3.5" />
                        </Button>
                        <IconButton
                          variant="danger"
                          onClick={() =>
                            deleteStage.mutate(s.id, { onError: (e) => toast.error(errorMessage(e)) })
                          }
                        >
                          <Trash2 className="h-4 w-4" />
                        </IconButton>
                      </div>
                    )}
                  </Draggable>
                ))}
                {provided.placeholder}
              </div>
            )}
          </Droppable>
        </DragDropContext>
        <div className="mt-4 grid grid-cols-[auto_auto_1fr_9rem_auto_auto] items-center gap-3">
          <span />
          <span />
          <Input
            value={name}
            onChange={(e) => setName(e.target.value)}
            placeholder="New stage name"
            className="h-8 text-sm"
          />
          <Select
            value={newStageType}
            className="h-8 text-sm"
            onChange={(e) => setNewStageType(e.target.value as StageType)}
          >
            {STAGE_TYPES.map((t) => (
              <option key={t.value} value={t.value}>
                {t.label}
              </option>
            ))}
          </Select>
          <Button
            className="col-span-2"
            disabled={!name.trim()}
            onClick={() =>
              createStage.mutate(
                { pipelineId, body: { name: name.trim(), stage_type: newStageType } },
                { onSuccess: () => setName("") }
              )
            }
          >
            <Plus className="h-4 w-4" /> New
          </Button>
        </div>
      </Card>

      <StageSettingsDrawer
        stage={settingsStage ? sorted.find((s) => s.id === settingsStage.id) ?? settingsStage : null}
        pipelineId={pipelineId}
        open={!!settingsStage}
        onClose={() => setSettingsStage(null)}
      />
    </>
  );
}
