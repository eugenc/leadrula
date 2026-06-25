import { useState } from "react";
import { X } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Textarea } from "@/components/ui/input";
import { useAddNote } from "./hooks";
import { toast } from "@/store/toastStore";
import { errorMessage } from "@/lib/api";
import type { Lead } from "@/types";

export function LeadNoteSheet({
  lead,
  open,
  onClose,
}: {
  lead: Lead | null;
  open: boolean;
  onClose: () => void;
}) {
  const addNote = useAddNote();
  const [body, setBody] = useState("");

  if (!open || !lead) return null;

  const leadName = `${lead.first_name} ${lead.last_name}`.trim();

  function handleClose() {
    setBody("");
    onClose();
  }

  function handleSubmit() {
    const trimmed = body.trim();
    if (!trimmed) return;
    addNote.mutate(
      { leadId: lead!.id, body: trimmed },
      {
        onSuccess: () => {
          toast.success("Note added");
          handleClose();
        },
        onError: (err) => toast.error(errorMessage(err)),
      }
    );
  }

  return (
    <div className="fixed inset-0 z-[70]">
      <button
        type="button"
        aria-label="Close"
        className="absolute inset-0 bg-[var(--surface-overlay)]"
        onClick={handleClose}
      />
      <div
        className="absolute bottom-0 left-0 right-0 animate-slideUp rounded-t-xl bg-surface-card shadow-xl"
        style={{ paddingBottom: "env(safe-area-inset-bottom, 0px)" }}
      >
        <div className="flex items-center justify-between border-b border-gray-100 px-4 py-3">
          <div>
            <h3 className="text-base font-semibold text-gray-800">Add note</h3>
            <p className="text-xs text-gray-400">{leadName || "Lead"}</p>
          </div>
          <button
            type="button"
            onClick={handleClose}
            className="text-gray-400 hover:text-gray-700"
            aria-label="Close"
          >
            <X className="h-5 w-5" />
          </button>
        </div>
        <div className="p-4">
          <Textarea
            value={body}
            onChange={(e) => setBody(e.target.value)}
            placeholder="Add a note…"
            rows={4}
            autoFocus
          />
          <div className="mt-3 flex justify-end">
            <Button size="sm" disabled={!body.trim() || addNote.isPending} onClick={handleSubmit}>
              Add Note
            </Button>
          </div>
        </div>
      </div>
    </div>
  );
}
