import { useEffect, useState } from "react";
import { Dialog } from "@/components/ui/dialog";
import { Button } from "@/components/ui/button";
import { Label, Select } from "@/components/ui/input";
import type { Stage } from "@/types";
import { useStageDisqReasons } from "./hooks";
import { DatetimeFieldInput } from "./DatetimeFieldInput";

export interface PromptResult {
  action_at?: string | null;
  disqualification_reason_id?: number | null;
}

export function StagePromptModal({
  open,
  stage,
  initialActionAt,
  onCancel,
  onConfirm,
}: {
  open: boolean;
  stage: Stage | null;
  initialActionAt?: string;
  onCancel: () => void;
  onConfirm: (r: PromptResult) => void;
}) {
  const { data: reasons } = useStageDisqReasons(stage?.id ?? null);
  const [actionAt, setActionAt] = useState("");
  const [reasonId, setReasonId] = useState("");

  useEffect(() => {
    if (!open) return;
    setActionAt(initialActionAt ?? "");
    setReasonId("");
  }, [open, stage?.id, initialActionAt]);

  if (!stage) return null;

  const needAction = stage.stage_type === "action";
  const needDisq = stage.stage_type === "disqualification";
  const valid = (!needAction || actionAt) && (!needDisq || reasonId);

  return (
    <Dialog
      open={open}
      onClose={onCancel}
      title={`Move to "${stage.name}"`}
      subtitle="Complete the required fields to move this lead."
      className="max-w-[420px]"
      footer={
        <>
          <Button variant="secondary" onClick={onCancel}>
            Cancel
          </Button>
          <Button
            disabled={!valid}
            onClick={() =>
              onConfirm({
                action_at: needAction && actionAt ? new Date(actionAt).toISOString() : undefined,
                disqualification_reason_id: needDisq && reasonId ? Number(reasonId) : undefined,
              })
            }
          >
            Confirm Move
          </Button>
        </>
      }
    >
      <div className="space-y-4">
        {needAction && (
          <div>
            <Label>Action Date &amp; Time</Label>
            <DatetimeFieldInput layout="inline" value={actionAt} onChange={setActionAt} />
            <p className="mt-1 text-xs text-gray-400">When should the next action happen?</p>
          </div>
        )}
        {needDisq && (
          <div>
            <Label>Disqualification Reason</Label>
            <Select value={reasonId} onChange={(e) => setReasonId(e.target.value)}>
              <option value="">Select a reason…</option>
              {(reasons ?? [])
                .filter((r) => r.is_active)
                .map((r) => (
                  <option key={r.id} value={r.id}>
                    {r.label}
                  </option>
                ))}
            </Select>
          </div>
        )}
      </div>
    </Dialog>
  );
}
