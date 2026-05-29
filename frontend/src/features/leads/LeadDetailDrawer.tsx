import { useEffect, useState } from "react";
import { Sheet } from "@/components/ui/dialog";
import { Button } from "@/components/ui/button";
import { Input, Label, Textarea, Select } from "@/components/ui/input";
import { Avatar, Badge, Spinner } from "@/components/ui/misc";
import { X } from "lucide-react";
import { format } from "date-fns";
import { useUIStore } from "@/store/uiStore";
import { useAuthStore } from "@/store/authStore";
import { toast } from "@/store/toastStore";
import {
  useLead,
  useNotes,
  useAddNote,
  useStageHistory,
  useUpdateLead,
  useUsers,
  useCustomFields,
} from "./hooks";
import type { Lead } from "@/types";

const BUILTINS: { key: keyof Lead; label: string }[] = [
  { key: "first_name", label: "First Name" },
  { key: "last_name", label: "Last Name" },
  { key: "phone", label: "Phone" },
  { key: "email", label: "Email" },
  { key: "address", label: "Address" },
  { key: "city", label: "City" },
  { key: "state", label: "State" },
  { key: "zip", label: "Zip" },
];

export function LeadDetailDrawer() {
  const leadId = useUIStore((s) => s.detailLeadId);
  const close = useUIStore((s) => s.closeDetail);
  const { data: lead, isLoading } = useLead(leadId);

  return (
    <Sheet open={!!leadId} onClose={close} width={520}>
      {isLoading || !lead ? (
        <div className="flex justify-center py-20">
          <Spinner className="h-6 w-6" />
        </div>
      ) : (
        <DrawerContent lead={lead} onClose={close} />
      )}
    </Sheet>
  );
}

function DrawerContent({ lead, onClose }: { lead: Lead; onClose: () => void }) {
  const user = useAuthStore((s) => s.user);
  const [tab, setTab] = useState<"details" | "notes" | "history">("details");
  const update = useUpdateLead();
  const { data: users } = useUsers();
  const { data: customFields } = useCustomFields();

  const [fields, setFields] = useState<Record<string, string>>({});
  useEffect(() => {
    const f: Record<string, string> = {};
    for (const b of BUILTINS) f[b.key as string] = (lead[b.key] as string) ?? "";
    setFields(f);
  }, [lead]);

  function saveField(key: string) {
    update.mutate(
      { leadId: lead.id, body: { fields: { [key]: fields[key] } } },
      { onSuccess: () => toast.success("Saved"), onError: () => toast.error("Save failed") }
    );
  }

  return (
    <div className="flex h-full flex-col">
      <div className="flex items-center justify-between border-b border-pd-border px-5 py-3">
        <div>
          <div className="text-base font-bold">
            {lead.first_name} {lead.last_name}
          </div>
          <div className="text-xs text-pd-muted">
            {lead.campaign_name ?? "—"} · <span className="capitalize">{lead.status}</span>
          </div>
        </div>
        <button onClick={onClose} className="text-pd-muted hover:text-pd-text">
          <X className="h-4 w-4" />
        </button>
      </div>

      <div className="flex border-b border-pd-border px-5">
        {(["details", "notes", "history"] as const).map((t) => (
          <button
            key={t}
            onClick={() => setTab(t)}
            className={`-mb-px border-b-2 px-3 py-2 text-sm font-semibold capitalize ${
              tab === t ? "border-pd-green text-pd-green" : "border-transparent text-pd-muted"
            }`}
          >
            {t}
          </button>
        ))}
      </div>

      <div className="flex-1 overflow-y-auto p-5">
        {tab === "details" && (
          <div className="space-y-4">
            {user?.role === "admin" && (
              <div>
                <Label>Assigned To</Label>
                <Select
                  value={lead.assigned_user_id ?? ""}
                  onChange={(e) =>
                    update.mutate({
                      leadId: lead.id,
                      body: e.target.value
                        ? { assigned_user_id: Number(e.target.value) }
                        : { clear_assignee: true },
                    })
                  }
                >
                  <option value="">Unassigned</option>
                  {(users ?? []).map((u) => (
                    <option key={u.id} value={u.id}>
                      {u.full_name}
                    </option>
                  ))}
                </Select>
              </div>
            )}
            {BUILTINS.map((b) => (
              <div key={b.key as string}>
                <Label>{b.label}</Label>
                <Input
                  value={fields[b.key as string] ?? ""}
                  onChange={(e) => setFields((f) => ({ ...f, [b.key as string]: e.target.value }))}
                  onBlur={() => saveField(b.key as string)}
                />
              </div>
            ))}
            {(customFields ?? [])
              .filter((f) => f.is_active)
              .map((f) => (
                <div key={f.id}>
                  <Label>{f.name}</Label>
                  <CustomFieldValue
                    leadId={lead.id}
                    fieldId={f.id}
                    type={f.type}
                    options={f.options}
                    value={lead.custom_values?.[String(f.id)]}
                  />
                </div>
              ))}
            <RedistributeBox lead={lead} />
          </div>
        )}
        {tab === "notes" && <NotesTab leadId={lead.id} />}
        {tab === "history" && <HistoryTab leadId={lead.id} />}
      </div>
    </div>
  );
}

function CustomFieldValue({
  leadId,
  fieldId,
  type,
  options,
  value,
}: {
  leadId: number;
  fieldId: number;
  type: string;
  options: string[];
  value: unknown;
}) {
  const update = useUpdateLead();
  const [val, setVal] = useState(value == null ? "" : typeof value === "string" ? value : JSON.stringify(value));

  function save(next: unknown) {
    update.mutate({ leadId, body: { custom_values: { [String(fieldId)]: next } } });
  }

  if (type === "dropdown") {
    return (
      <Select
        value={val}
        onChange={(e) => {
          setVal(e.target.value);
          save(e.target.value);
        }}
      >
        <option value="">—</option>
        {(options ?? []).map((o) => (
          <option key={o} value={o}>
            {o}
          </option>
        ))}
      </Select>
    );
  }
  return (
    <Input
      value={val}
      type={type === "number" ? "number" : type === "date" ? "date" : "text"}
      onChange={(e) => setVal(e.target.value)}
      onBlur={() => save(type === "number" ? Number(val) : val)}
    />
  );
}

function NotesTab({ leadId }: { leadId: number }) {
  const { data: notes } = useNotes(leadId);
  const addNote = useAddNote();
  const [body, setBody] = useState("");
  return (
    <div className="space-y-3">
      <div>
        <Textarea value={body} onChange={(e) => setBody(e.target.value)} placeholder="Add a note…" />
        <div className="mt-2 flex justify-end">
          <Button
            size="sm"
            disabled={!body.trim()}
            onClick={() =>
              addNote.mutate(
                { leadId, body },
                { onSuccess: () => setBody("") }
              )
            }
          >
            Add Note
          </Button>
        </div>
      </div>
      <div className="space-y-2">
        {(notes ?? []).map((n) => (
          <div key={n.id} className="rounded border border-pd-border p-3">
            <div className="mb-1 flex items-center gap-2">
              <Avatar name={n.author_name || "?"} className="h-6 w-6 text-[10px]" />
              <span className="text-sm font-semibold">{n.author_name || "System"}</span>
              <span className="text-xs text-pd-muted">
                {format(new Date(n.created_at), "MMM d, h:mma")}
              </span>
            </div>
            <p className="whitespace-pre-wrap text-sm">{n.body}</p>
          </div>
        ))}
      </div>
    </div>
  );
}

function HistoryTab({ leadId }: { leadId: number }) {
  const { data: history } = useStageHistory(leadId);
  return (
    <div className="space-y-2">
      {(history ?? []).length === 0 && <p className="text-sm text-pd-muted">No stage changes yet.</p>}
      {(history ?? []).map((h) => (
        <div key={h.id} className="rounded border border-pd-border p-3 text-sm">
          <div className="flex items-center gap-2">
            <span className="text-pd-muted">{h.from_stage_name ?? "Created"}</span>
            <span>→</span>
            <span className="font-semibold">{h.to_stage_name}</span>
          </div>
          <div className="mt-1 text-xs text-pd-muted">
            {h.moved_by_name ?? "System"} · {format(new Date(h.created_at), "MMM d, h:mma")}
            {h.action_at_captured &&
              ` · action ${format(new Date(h.action_at_captured), "MMM d, h:mma")}`}
            {h.disqualification_reason && ` · ${h.disqualification_reason}`}
          </div>
        </div>
      ))}
    </div>
  );
}

function RedistributeBox({ lead }: { lead: Lead }) {
  const user = useAuthStore((s) => s.user);
  if (user?.account_type !== "publisher" || lead.status !== "returned") return null;
  return (
    <div className="rounded border border-pd-amber/40 bg-pd-amber/10 p-3">
      <div className="mb-1 flex items-center gap-2">
        <Badge variant="amber">Returned</Badge>
        <span className="text-sm font-semibold">Re-distribute this lead</span>
      </div>
      <p className="text-xs text-pd-muted">
        Send this returned lead to another buyer from the Contracts page.
      </p>
    </div>
  );
}
