import { useEffect, useState } from "react";
import { Sheet, DrawerHeader, DrawerBody } from "@/components/ui/dialog";
import { Button } from "@/components/ui/button";
import { Input, Label, Textarea, Select } from "@/components/ui/input";
import { Avatar, Badge, Spinner } from "@/components/ui/misc";
import { SectionLabel } from "@/components/layout/SectionLabel";
import { ActionDot } from "./ActionDot";
import { format, isPast } from "date-fns";
import { cn } from "@/lib/utils";
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
import { formatStatus } from "./leadsListColumns";
import { LeadTagsEditor } from "./LeadTagsEditor";

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
    <Sheet open={!!leadId} onClose={close}>
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

  const overdue = lead.action_at && isPast(new Date(lead.action_at));

  return (
    <div className="flex h-full flex-col">
      <DrawerHeader
        title={`${lead.first_name} ${lead.last_name}`}
        subtitle={`${lead.campaign_name ?? "—"} · ${formatStatus(lead.status)}`}
        onClose={onClose}
      />

      {lead.action_at && (
        <div className="border-b border-gray-100 px-5 py-2">
          <div
            className={cn(
              "flex items-center gap-2 rounded-md border px-2.5 py-1.5",
              overdue
                ? "border-danger-border bg-danger-bg"
                : "border-gray-100 bg-gray-50"
            )}
          >
            <ActionDot actionAt={lead.action_at} variant="dot" />
            <span className={cn("text-xs", overdue ? "font-semibold text-danger" : "text-gray-700")}>
              Action {format(new Date(lead.action_at), "MMM d, h:mma")}
              {overdue && " — overdue"}
            </span>
          </div>
        </div>
      )}

      <div className="flex border-b border-gray-100 px-5">
        {(["details", "notes", "history"] as const).map((t) => (
          <button
            key={t}
            onClick={() => setTab(t)}
            className={cn(
              "-mb-px border-b-2 px-2.5 py-1.5 text-sm font-semibold capitalize transition-colors",
              tab === t ? "border-jade-500 text-jade-700" : "border-transparent text-gray-400"
            )}
          >
            {t}
          </button>
        ))}
      </div>

      <DrawerBody>
        {tab === "details" && (
          <div className="flex flex-col gap-4">
            <div>
              <SectionLabel className="mb-2">Lead</SectionLabel>
              <div className="flex flex-col gap-2.5">
                {user?.role === "admin" ? (
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
                      {(users ?? []).filter((u) => u.status === "active").map((u) => (
                        <option key={u.id} value={u.id}>
                          {u.full_name}
                        </option>
                      ))}
                    </Select>
                  </div>
                ) : (
                  <div>
                    <Label>Assigned To</Label>
                    <div className="mt-1 flex items-center gap-2 text-sm text-gray-700">
                      {lead.assignee_name ? (
                        <>
                          <Avatar
                            name={lead.assignee_name}
                            src={lead.assignee_avatar_url}
                            variant="card"
                          />
                          {lead.assignee_name}
                        </>
                      ) : (
                        "Unassigned"
                      )}
                    </div>
                  </div>
                )}
                <div>
                  <Label>Buyer</Label>
                  <div className="mt-1 text-sm text-gray-700">{lead.buyer_name ?? "—"}</div>
                </div>
              </div>
            </div>
            <LeadTagsEditor leadId={lead.id} tags={lead.tags ?? []} />
            <div>
              <SectionLabel className="mb-2">Contact</SectionLabel>
              <div className="flex flex-col gap-2.5">
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
              </div>
            </div>
            {(customFields ?? []).filter((f) => f.is_active).length > 0 && (
              <div>
                <SectionLabel className="mb-2">Custom Fields</SectionLabel>
                <div className="flex flex-col gap-2.5">
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
                </div>
              </div>
            )}
            <RedistributeBox lead={lead} />
          </div>
        )}
        {tab === "notes" && <NotesTab leadId={lead.id} />}
        {tab === "history" && <HistoryTab leadId={lead.id} />}
      </DrawerBody>
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
    <div className="flex flex-col gap-4">
      <div>
        <Textarea
          value={body}
          onChange={(e) => setBody(e.target.value)}
          placeholder="Add a note…"
        />
        <div className="mt-1.5 flex justify-end">
          <Button
            size="sm"
            disabled={!body.trim()}
            onClick={() => addNote.mutate({ leadId, body }, { onSuccess: () => setBody("") })}
          >
            Add Note
          </Button>
        </div>
      </div>
      <div>
        <SectionLabel className="mb-2">Notes</SectionLabel>
        {(notes ?? []).map((n) => (
          <div key={n.id} className="border-b border-gray-100 py-2 last:border-0">
            <div className="mb-0.5 flex items-center gap-1.5">
              <span className="text-xs font-semibold text-gray-600">{n.author_name || "System"}</span>
              <span className="text-xs text-gray-400">
                {format(new Date(n.created_at), "MMM d, h:mma")}
              </span>
            </div>
            <p className="whitespace-pre-wrap text-sm leading-relaxed text-gray-700">{n.body}</p>
          </div>
        ))}
      </div>
    </div>
  );
}

function HistoryTab({ leadId }: { leadId: number }) {
  const { data: history } = useStageHistory(leadId);
  return (
    <div>
      <SectionLabel className="mb-2">Stage History</SectionLabel>
      {(history ?? []).length === 0 && (
        <p className="text-sm text-gray-400">No stage changes yet.</p>
      )}
      {(history ?? []).map((h) => (
        <div key={h.id} className="flex items-start gap-2.5 py-1.5 text-sm text-gray-500">
          <span className="mt-1.5 h-2 w-2 shrink-0 rounded-full bg-jade-300" />
          <div>
            <div>
              {h.from_stage_name ?? "Created"} → <span className="font-medium">{h.to_stage_name}</span>
            </div>
            <div className="text-xs text-gray-400">
              {h.moved_by_name ?? "System"} · {format(new Date(h.created_at), "MMM d, h:mma")}
              {h.action_at_captured &&
                ` · action ${format(new Date(h.action_at_captured), "MMM d, h:mma")}`}
              {h.disqualification_reason && ` · ${h.disqualification_reason}`}
            </div>
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
    <div className="rounded-md border border-warning-border bg-warning-bg p-2.5">
      <div className="mb-0.5 flex items-center gap-2">
        <Badge variant="returned">Returned</Badge>
        <span className="text-xs font-semibold text-gray-800">Re-distribute this lead</span>
      </div>
      <p className="text-xs text-gray-400">
        Send this returned lead to another buyer from the Contracts page.
      </p>
    </div>
  );
}
