import { useEffect, useState } from "react";
import { Button } from "@/components/ui/button";
import { Label, Select, Textarea } from "@/components/ui/input";
import { SectionLabel } from "@/components/layout/SectionLabel";
import { toast } from "@/store/toastStore";
import { errorMessage } from "@/lib/api";
import { useSetCallDisposition } from "@/features/calls/hooks";
import type { Call } from "@/types";

const DISPOSITIONS = [
  { value: "", label: "—" },
  { value: "converted", label: "Converted" },
  { value: "not_interested", label: "Not interested" },
  { value: "callback", label: "Callback" },
  { value: "wrong_number", label: "Wrong number" },
  { value: "no_answer", label: "No answer" },
];

export function DispositionEditor({ call, onSaved }: { call: Call; onSaved?: () => void }) {
  const save = useSetCallDisposition();
  const [disposition, setDisposition] = useState(call.disposition ?? "");
  const [note, setNote] = useState(call.disposition_note ?? "");

  useEffect(() => {
    setDisposition(call.disposition ?? "");
    setNote(call.disposition_note ?? "");
  }, [call.id, call.disposition, call.disposition_note]);

  function submit() {
    if (!disposition) {
      toast.error("Select a disposition");
      return;
    }
    save.mutate(
      { callId: call.id, disposition, note },
      {
        onSuccess: () => {
          toast.success("Disposition saved");
          onSaved?.();
        },
        onError: (e) => toast.error(errorMessage(e)),
      }
    );
  }

  return (
    <div>
      <SectionLabel>Disposition</SectionLabel>
      <div className="mt-2 space-y-2">
        <div>
          <Label>Outcome</Label>
          <Select value={disposition} onChange={(e) => setDisposition(e.target.value)}>
            {DISPOSITIONS.map((d) => (
              <option key={d.value} value={d.value}>
                {d.label}
              </option>
            ))}
          </Select>
        </div>
        <div>
          <Label>Note (optional)</Label>
          <Textarea value={note} onChange={(e) => setNote(e.target.value)} rows={3} />
        </div>
        <Button variant="secondary" disabled={save.isPending} onClick={submit}>
          Save disposition
        </Button>
      </div>
    </div>
  );
}
