import { useEffect, useState } from "react";
import { format } from "date-fns";
import { Button } from "@/components/ui/button";
import { Input, Label, Select } from "@/components/ui/input";
import { FormDrawer } from "@/components/ui/dialog";
import { toast } from "@/store/toastStore";
import { errorMessage } from "@/lib/api";
import { useLeads, usePipelines, useStages } from "@/features/leads/hooks";
import { useBookAppointment, useBuyerBookAppointment } from "@/features/appointments/hooks";
import type { AppointmentFreeSlot } from "@/types";

export function BookAppointmentDrawer({
  open,
  onClose,
  onBooked,
  contractId,
  slot,
  mode = "publisher",
}: {
  open: boolean;
  onClose: () => void;
  onBooked?: () => void;
  contractId: number;
  slot: AppointmentFreeSlot | null;
  mode?: "publisher" | "buyer";
}) {
  const isBuyer = mode === "buyer";
  const publisherBook = useBookAppointment();
  const buyerBook = useBuyerBookAppointment();
  const book = isBuyer ? buyerBook : publisherBook;
  const [modeLead, setModeLead] = useState<"existing" | "new">("existing");
  const [deliveryMode, setDeliveryMode] = useState<"" | "contract" | "publisher_pipeline">("");
  const [search, setSearch] = useState("");
  const [debounced, setDebounced] = useState("");
  const [leadId, setLeadId] = useState<number | null>(null);
  const [firstName, setFirstName] = useState("");
  const [lastName, setLastName] = useState("");
  const [phone, setPhone] = useState("");
  const [email, setEmail] = useState("");
  const [source, setSource] = useState("");
  const [pipelineId, setPipelineId] = useState(0);
  const [stageId, setStageId] = useState(0);

  const { data: leadRes } = useLeads({ q: debounced || undefined, limit: 20 });
  const leads = leadRes?.items ?? [];
  const { data: pipelines } = usePipelines();
  const { data: stages } = useStages(pipelineId || undefined);

  useEffect(() => {
    const t = setTimeout(() => setDebounced(search.trim()), 300);
    return () => clearTimeout(t);
  }, [search]);

  useEffect(() => {
    if (!open) {
      setDeliveryMode(isBuyer ? "contract" : "");
      setLeadId(null);
      setModeLead("existing");
    } else if (isBuyer) {
      setDeliveryMode("contract");
    }
  }, [open, isBuyer]);

  function submit() {
    if (!slot) return;
    const effectiveDelivery = isBuyer ? "contract" : deliveryMode;
    if (!effectiveDelivery) {
      toast.error("Choose a delivery mode");
      return;
    }
    const body: Record<string, unknown> = {
      contract_id: contractId,
      buyer_slot_id: slot.buyer_slot_id,
      slot_start: slot.slot_start,
      delivery_mode: effectiveDelivery,
    };
    if (modeLead === "existing") {
      if (!leadId) {
        toast.error("Select a lead");
        return;
      }
      body.lead_id = leadId;
    } else {
      body.first_name = firstName;
      body.last_name = lastName;
      body.phone = phone;
      body.email = email;
      body.source = source;
    }
    if (!isBuyer && effectiveDelivery === "publisher_pipeline") {
      if (!pipelineId || !stageId) {
        toast.error("Select publisher pipeline and stage");
        return;
      }
      body.publisher_pipeline_id = pipelineId;
      body.publisher_stage_id = stageId;
    }
    book.mutate(body, {
      onSuccess: () => {
        toast.success("Appointment booked");
        onClose();
        onBooked?.();
      },
      onError: (e) => toast.error(errorMessage(e)),
    });
  }

  return (
    <FormDrawer open={open} onClose={onClose} title="Book appointment">
      {slot && (
        <p className="mb-4 text-sm text-gray-600">
          {format(new Date(slot.slot_start), "EEEE, MMM d · h:mm a")} · {slot.duration_min} min ·{" "}
          {slot.remaining_capacity} free
        </p>
      )}

      <div className="mb-4 flex gap-2">
        <button
          type="button"
          className={`rounded px-3 py-1 text-sm font-semibold ${modeLead === "existing" ? "bg-jade-500 text-white" : "bg-gray-100"}`}
          onClick={() => setModeLead("existing")}
        >
          Existing lead
        </button>
        <button
          type="button"
          className={`rounded px-3 py-1 text-sm font-semibold ${modeLead === "new" ? "bg-jade-500 text-white" : "bg-gray-100"}`}
          onClick={() => setModeLead("new")}
        >
          New lead
        </button>
      </div>

      {modeLead === "existing" ? (
        <div className="space-y-2">
          <Label>Search leads</Label>
          <Input value={search} onChange={(e) => setSearch(e.target.value)} placeholder="Name, phone, email…" />
          <div className="max-h-40 overflow-y-auto rounded border border-gray-100">
            {leads.map((l) => (
              <button
                key={l.id}
                type="button"
                onClick={() => setLeadId(l.id)}
                className={`block w-full px-3 py-2 text-left text-sm hover:bg-jade-50 ${
                  leadId === l.id ? "bg-jade-100 font-semibold" : ""
                }`}
              >
                {[l.first_name, l.last_name].filter(Boolean).join(" ") || "—"} · {l.phone || l.email || "no contact"}
              </button>
            ))}
          </div>
        </div>
      ) : (
        <div className="grid grid-cols-2 gap-2">
          <div>
            <Label>First name</Label>
            <Input value={firstName} onChange={(e) => setFirstName(e.target.value)} />
          </div>
          <div>
            <Label>Last name</Label>
            <Input value={lastName} onChange={(e) => setLastName(e.target.value)} />
          </div>
          <div>
            <Label>Phone</Label>
            <Input value={phone} onChange={(e) => setPhone(e.target.value)} />
          </div>
          <div>
            <Label>Email</Label>
            <Input value={email} onChange={(e) => setEmail(e.target.value)} />
          </div>
          <div className="col-span-2">
            <Label>Source (optional)</Label>
            <Input value={source} onChange={(e) => setSource(e.target.value)} />
          </div>
        </div>
      )}

      {!isBuyer && (
        <>
          <div className="mt-4 space-y-2">
            <Label>Delivery</Label>
            <label className="flex items-center gap-2 text-sm">
              <input
                type="radio"
                name="delivery"
                checked={deliveryMode === "contract"}
                onChange={() => setDeliveryMode("contract")}
              />
              Deliver via contract
            </label>
            <label className="flex items-center gap-2 text-sm">
              <input
                type="radio"
                name="delivery"
                checked={deliveryMode === "publisher_pipeline"}
                onChange={() => setDeliveryMode("publisher_pipeline")}
              />
              Place on publisher pipeline
            </label>
          </div>

          {deliveryMode === "publisher_pipeline" && (
            <div className="mt-3 grid grid-cols-2 gap-2">
              <div>
                <Label>Pipeline</Label>
                <Select value={pipelineId} onChange={(e) => setPipelineId(Number(e.target.value))}>
                  <option value={0}>Select…</option>
                  {(pipelines ?? []).map((p) => (
                    <option key={p.id} value={p.id}>
                      {p.name}
                    </option>
                  ))}
                </Select>
              </div>
              <div>
                <Label>Stage</Label>
                <Select value={stageId} onChange={(e) => setStageId(Number(e.target.value))}>
                  <option value={0}>Select…</option>
                  {(stages ?? []).map((s) => (
                    <option key={s.id} value={s.id}>
                      {s.name}
                    </option>
                  ))}
                </Select>
              </div>
            </div>
          )}
        </>
      )}

      <div className="mt-6 flex justify-end gap-2">
        <Button variant="secondary" onClick={onClose}>
          Cancel
        </Button>
        <Button onClick={submit} disabled={book.isPending || !slot}>
          Book
        </Button>
      </div>
    </FormDrawer>
  );
}
