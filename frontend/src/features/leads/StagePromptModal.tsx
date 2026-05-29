import { useState } from "react";
import { Dialog } from "@/components/ui/dialog";
import { Button } from "@/components/ui/button";
import { Label, Select } from "@/components/ui/input";
import type { Stage } from "@/types";
import { useDisqReasons } from "./hooks";

export interface PromptResult {
  action_at?: string | null;
  disqualification_reason_id?: number | null;
}

export function StagePromptModal({
  open,
  stage,
  onCancel,
  onConfirm,
}: {
  open: boolean;
  stage: Stage | null;
  onCancel: () => void;
  onConfirm: (r: PromptResult) => void;
}) {
  const { data: reasons } = useDisqReasons();
  const [actionAt, setActionAt] = useState("");
  const [reasonId, setReasonId] = useState("");

  if (!stage) return null;

  const needAction = stage.prompt_action_datetime;
  const needDisq = stage.prompt_disqualification;
  const valid = (!needAction || actionAt) && (!needDisq || reasonId);

  return (
    <Dialog open={open} onClose={onCancel} title={`Move to "${stage.name}"`}>
      <div className="space-y-4">
        {needAction && (
          <div>
            <Label>Action Date &amp; Time</Label>
            <input
              type="datetime-local"
              value={actionAt}
              onChange={(e) => setActionAt(e.target.value)}
              className="h-9 w-full rounded border border-pd-border px-3 text-sm"
            />
            <p className="mt-1 text-xs text-pd-muted">When should the next action happen?</p>
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
        <div className="flex justify-end gap-2 pt-2">
          <Button variant="outline" onClick={onCancel}>
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
        </div>
      </div>
    </Dialog>
  );
}
