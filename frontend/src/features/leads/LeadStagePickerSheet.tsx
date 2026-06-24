import { useState } from "react";
import { X } from "lucide-react";
import { useChangeStage, useStages } from "./hooks";
import { StagePromptModal, type PromptResult } from "./StagePromptModal";
import {
  initialActionAtForStageMove,
  stageNeedsPrompt,
  stagePromptMissingError,
} from "@/features/pipelines/stageTypes";
import { apiError, errorMessage } from "@/lib/api";
import { toast } from "@/store/toastStore";
import { cn } from "@/lib/utils";
import { stageColorDot } from "@/features/pipelines/stageColors";
import type { Lead, Stage } from "@/types";

export function LeadStagePickerSheet({
  lead,
  open,
  onClose,
}: {
  lead: Lead | null;
  open: boolean;
  onClose: () => void;
}) {
  const { data: stages } = useStages(lead?.pipeline_id ?? undefined);
  const changeStage = useChangeStage();
  const [prompt, setPrompt] = useState<{ stage: Stage; initialActionAt: string } | null>(null);

  if (!open || !lead) return null;

  function commit(stage: Stage, extra?: PromptResult) {
    changeStage.mutate(
      { leadId: lead!.id, payload: { stage_id: stage.id, ...extra } },
      {
        onSuccess: () => {
          setPrompt(null);
          toast.success("Stage updated");
          onClose();
        },
        onError: (err) => {
          const e = apiError(err);
          if (stagePromptMissingError(e.code, e.message, stage.stage_type)) {
            const fromStageType = stages?.find((s) => s.id === lead?.stage_id)?.stage_type;
            setPrompt({
              stage,
              initialActionAt: initialActionAtForStageMove(
                fromStageType,
                stage.stage_type,
                lead?.action_at
              ),
            });
          } else {
            toast.error(errorMessage(err));
          }
        },
      }
    );
  }

  function pickStage(stage: Stage) {
    if (stage.id === lead?.stage_id) {
      onClose();
      return;
    }
    if (stageNeedsPrompt(stage.stage_type)) {
      const fromStageType = stages?.find((s) => s.id === lead?.stage_id)?.stage_type;
      setPrompt({
        stage,
        initialActionAt: initialActionAtForStageMove(
          fromStageType,
          stage.stage_type,
          lead?.action_at
        ),
      });
      return;
    }
    commit(stage);
  }

  return (
    <>
      <div className="fixed inset-0 z-[70]">
        <button
          type="button"
          aria-label="Close"
          className="absolute inset-0 bg-[var(--surface-overlay)]"
          onClick={onClose}
        />
        <div
          className="absolute bottom-0 left-0 right-0 max-h-[70vh] animate-slideUp overflow-y-auto rounded-t-xl bg-surface-card shadow-xl"
          style={{ paddingBottom: "env(safe-area-inset-bottom, 0px)" }}
        >
          <div className="flex items-center justify-between border-b border-gray-100 px-4 py-3">
            <div>
              <h3 className="text-base font-semibold text-gray-800">Change stage</h3>
              <p className="text-xs text-gray-400">
                {lead.pipeline_name ?? "Pipeline"} ·{" "}
                {`${lead.first_name} ${lead.last_name}`.trim()}
              </p>
            </div>
            <button
              type="button"
              onClick={onClose}
              className="text-gray-400 hover:text-gray-700"
              aria-label="Close"
            >
              <X className="h-5 w-5" />
            </button>
          </div>
          {!lead.pipeline_id ? (
            <p className="px-4 py-6 text-center text-sm text-gray-400">
              This lead has no pipeline. Open the lead to set a pipeline first.
            </p>
          ) : (stages ?? []).length === 0 ? (
            <p className="px-4 py-6 text-center text-sm text-gray-400">No stages available.</p>
          ) : (
            <ul className="divide-y divide-gray-50 p-2">
              {(stages ?? []).map((stage) => (
                <li key={stage.id}>
                  <button
                    type="button"
                    disabled={changeStage.isPending}
                    onClick={() => pickStage(stage)}
                    className={cn(
                      "flex w-full items-center gap-3 rounded-md px-3 py-3 text-left text-sm hover:bg-gray-50",
                      stage.id === lead.stage_id && "bg-jade-50 font-medium text-jade-700"
                    )}
                  >
                    <span className={cn("h-2.5 w-2.5 shrink-0 rounded-full", stageColorDot(stage.color))} />
                    {stage.name}
                    {stage.id === lead.stage_id && (
                      <span className="ml-auto text-xs text-jade-600">Current</span>
                    )}
                  </button>
                </li>
              ))}
            </ul>
          )}
        </div>
      </div>
      <StagePromptModal
        key={prompt ? `${lead.id}-${prompt.stage.id}` : "closed"}
        open={!!prompt}
        stage={prompt?.stage ?? null}
        initialActionAt={prompt?.initialActionAt}
        onCancel={() => setPrompt(null)}
        onConfirm={(r) => {
          if (prompt) commit(prompt.stage, r);
        }}
      />
    </>
  );
}
