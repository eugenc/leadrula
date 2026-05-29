import { useEffect, useState } from "react";
import { usePipelines, useStages } from "@/features/leads/hooks";
import {
  useCreatePipeline,
  useDeletePipeline,
  useCreateStage,
  useUpdateStage,
  useDeleteStage,
} from "@/features/admin/hooks";
import { PageHeader } from "@/components/layout/PageHeader";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Card, Switch, Spinner, EmptyState } from "@/components/ui/misc";
import { Plus, Trash2 } from "lucide-react";
import { toast } from "@/store/toastStore";
import { apiError } from "@/lib/api";

export function PipelinesPage() {
  const { data: pipelines, isLoading } = usePipelines();
  const [selected, setSelected] = useState<number | undefined>();
  useEffect(() => {
    if (!selected && pipelines?.length) setSelected(pipelines[0].id);
  }, [pipelines, selected]);

  const createPipeline = useCreatePipeline();
  const deletePipeline = useDeletePipeline();
  const [newName, setNewName] = useState("");

  return (
    <div>
      <PageHeader title="Pipelines" subtitle="Configure pipelines and their stages." />
      {isLoading ? (
        <Spinner className="h-6 w-6" />
      ) : (
        <div className="flex gap-6">
          <Card className="w-64 shrink-0 p-2">
            {(pipelines ?? []).map((p) => (
              <div
                key={p.id}
                className={`flex items-center justify-between rounded px-3 py-2 text-sm ${
                  selected === p.id ? "bg-pd-green/10 text-pd-green" : "hover:bg-pd-stage"
                }`}
              >
                <button className="flex-1 text-left font-medium" onClick={() => setSelected(p.id)}>
                  {p.name}
                </button>
                <button
                  onClick={() =>
                    deletePipeline.mutate(p.id, {
                      onError: (e) => toast.error(apiError(e).message),
                    })
                  }
                  className="text-pd-muted hover:text-pd-red"
                >
                  <Trash2 className="h-3.5 w-3.5" />
                </button>
              </div>
            ))}
            <div className="mt-2 flex gap-1 p-1">
              <Input
                value={newName}
                onChange={(e) => setNewName(e.target.value)}
                placeholder="New pipeline"
                className="h-8"
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

          <div className="flex-1">{selected ? <StagesEditor pipelineId={selected} /> : <EmptyState title="Select a pipeline" />}</div>
        </div>
      )}
    </div>
  );
}

function StagesEditor({ pipelineId }: { pipelineId: number }) {
  const { data: stages } = useStages(pipelineId);
  const createStage = useCreateStage();
  const updateStage = useUpdateStage();
  const deleteStage = useDeleteStage();
  const [name, setName] = useState("");

  return (
    <Card className="p-4">
      <div className="mb-3 grid grid-cols-[1fr_auto_auto_auto] items-center gap-3 text-xs font-semibold uppercase text-pd-muted">
        <span>Stage</span>
        <span>Action prompt</span>
        <span>Disq. prompt</span>
        <span />
      </div>
      <div className="space-y-2">
        {[...(stages ?? [])]
          .sort((a, b) => a.position - b.position)
          .map((s) => (
            <div key={s.id} className="grid grid-cols-[1fr_auto_auto_auto] items-center gap-3">
              <Input
                defaultValue={s.name}
                onBlur={(e) =>
                  e.target.value !== s.name &&
                  updateStage.mutate({ id: s.id, body: { name: e.target.value } })
                }
              />
              <div className="flex justify-center">
                <Switch
                  checked={s.prompt_action_datetime}
                  onChange={(v) => updateStage.mutate({ id: s.id, body: { prompt_action_datetime: v } })}
                />
              </div>
              <div className="flex justify-center">
                <Switch
                  checked={s.prompt_disqualification}
                  onChange={(v) => updateStage.mutate({ id: s.id, body: { prompt_disqualification: v } })}
                />
              </div>
              <button
                onClick={() =>
                  deleteStage.mutate(s.id, { onError: (e) => toast.error(apiError(e).message) })
                }
                className="text-pd-muted hover:text-pd-red"
              >
                <Trash2 className="h-4 w-4" />
              </button>
            </div>
          ))}
      </div>
      <div className="mt-4 flex gap-2">
        <Input value={name} onChange={(e) => setName(e.target.value)} placeholder="New stage name" />
        <Button
          onClick={() =>
            name &&
            createStage.mutate(
              { pipelineId, body: { name } },
              { onSuccess: () => setName("") }
            )
          }
        >
          <Plus className="h-4 w-4" /> Add Stage
        </Button>
      </div>
    </Card>
  );
}
