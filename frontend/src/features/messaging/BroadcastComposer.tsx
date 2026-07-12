import { useMemo, useState } from "react";
import { Megaphone } from "lucide-react";
import { Dialog } from "@/components/ui/dialog";
import { Button } from "@/components/ui/button";
import { Input, Textarea } from "@/components/ui/input";
import { cn } from "@/lib/utils";
import { toast } from "@/store/toastStore";
import { errorMessage } from "@/lib/api";
import { useBroadcast, useBroadcastRecipients } from "./hooks";
import type { BroadcastRecipient } from "./types";

type RecipientTab = "buyer" | "publisher";

function matchesSearch(row: BroadcastRecipient, q: string): boolean {
  if (!q) return true;
  const needle = q.toLowerCase();
  return row.name.toLowerCase().includes(needle) || row.handler_id.toLowerCase().includes(needle);
}

export function BroadcastComposer() {
  const [open, setOpen] = useState(false);
  const [body, setBody] = useState("");
  const [search, setSearch] = useState("");
  const [tab, setTab] = useState<RecipientTab>("buyer");
  const [selected, setSelected] = useState<string[]>([]);
  const { data: recipients, isLoading } = useBroadcastRecipients();
  const broadcast = useBroadcast();

  const tabRows = useMemo(() => {
    const q = search.trim();
    return (recipients ?? []).filter((r) => r.type === tab && matchesSearch(r, q));
  }, [recipients, tab, search]);

  const allTabIds = tabRows.map((r) => r.id);
  const allSelected = allTabIds.length > 0 && allTabIds.every((id) => selected.includes(id));

  const toggle = (id: string) =>
    setSelected((s) => (s.includes(id) ? s.filter((x) => x !== id) : [...s, id]));

  const toggleAllTab = () => {
    if (allSelected) {
      setSelected((s) => s.filter((id) => !allTabIds.includes(id)));
    } else {
      setSelected((s) => [...new Set([...s, ...allTabIds])]);
    }
  };

  const submit = () => {
    if (!body.trim() || selected.length === 0) return;
    broadcast.mutate(
      { body: body.trim(), recipient_account_ids: selected },
      {
        onSuccess: (job) => {
          toast.success(`Broadcast queued to ${job.total_count} recipient${job.total_count === 1 ? "" : "s"}`);
          setBody("");
          setSelected([]);
          setSearch("");
          setOpen(false);
        },
        onError: (e) => toast.error(errorMessage(e)),
      }
    );
  };

  return (
    <>
      <button
        type="button"
        aria-label="New broadcast"
        onClick={() => setOpen(true)}
        className="flex h-7 w-7 items-center justify-center rounded-md text-gray-500 hover:bg-gray-100 hover:text-gray-700"
      >
        <Megaphone className="h-4 w-4" />
      </button>
      <Dialog
        open={open}
        onClose={() => setOpen(false)}
        title="New broadcast"
        subtitle="Choose buyers and partner publishers to message."
        footer={
          <>
            <Button variant="secondary" onClick={() => setOpen(false)}>
              Cancel
            </Button>
            <Button
              onClick={submit}
              disabled={broadcast.isPending || !body.trim() || selected.length === 0}
            >
              Send broadcast
            </Button>
          </>
        }
      >
        <div className="flex flex-col gap-3">
          <div className="flex gap-1">
            <TabBtn label="Buyers" active={tab === "buyer"} onClick={() => setTab("buyer")} />
            <TabBtn label="Publishers" active={tab === "publisher"} onClick={() => setTab("publisher")} />
          </div>
          <Input
            value={search}
            onChange={(e) => setSearch(e.target.value)}
            placeholder="Search name or handler ID…"
          />
          <div className="flex items-center justify-between text-xs text-gray-500">
            <label className="flex cursor-pointer items-center gap-2">
              <input
                type="checkbox"
                checked={allSelected}
                onChange={toggleAllTab}
                disabled={tabRows.length === 0}
              />
              Select all
            </label>
            <span>{selected.length} selected</span>
          </div>
          <div className="max-h-48 overflow-y-auto rounded-md border border-gray-100">
            {isLoading && <p className="p-3 text-center text-xs text-gray-400">Loading recipients…</p>}
            {!isLoading && tabRows.length === 0 && (
              <p className="p-3 text-center text-xs text-gray-400">No {tab === "buyer" ? "buyers" : "publishers"} match.</p>
            )}
            {!isLoading &&
              tabRows.map((r) => (
                <label key={r.id} className="flex cursor-pointer items-center gap-2 px-2 py-1.5 hover:bg-gray-50">
                  <input type="checkbox" checked={selected.includes(r.id)} onChange={() => toggle(r.id)} />
                  <span className="min-w-0 truncate text-sm text-gray-700">{r.name}</span>
                  <span className="ml-auto shrink-0 font-mono text-[11px] text-gray-400">{r.handler_id}</span>
                </label>
              ))}
          </div>
          <Textarea
            value={body}
            onChange={(e) => setBody(e.target.value)}
            placeholder="Write your announcement…"
            rows={4}
          />
        </div>
      </Dialog>
    </>
  );
}

function TabBtn({ label, active, onClick }: { label: string; active: boolean; onClick: () => void }) {
  return (
    <button
      type="button"
      onClick={onClick}
      className={cn(
        "flex-1 rounded-md px-2 py-1 text-xs font-medium transition-colors",
        active ? "bg-jade-50 text-jade-700" : "text-gray-500 hover:bg-gray-100"
      )}
    >
      {label}
    </button>
  );
}
