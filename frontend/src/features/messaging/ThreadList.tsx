import { useEffect, useState } from "react";
import { Search, Archive, ArchiveRestore } from "lucide-react";
import { cn } from "@/lib/utils";
import { Input } from "@/components/ui/input";
import { useChatStore } from "@/store/chatStore";
import { useThreads } from "./hooks";
import type { Thread } from "./types";

export function ThreadList({ onOpen, archived = false }: { onOpen: (id: string) => void; archived?: boolean }) {
  const [query, setQuery] = useState("");
  const [debounced, setDebounced] = useState("");
  const setView = useChatStore((s) => s.setView);

  useEffect(() => {
    const t = setTimeout(() => setDebounced(query), 300);
    return () => clearTimeout(t);
  }, [query]);

  const { data: threads, isLoading } = useThreads(archived, archived ? "" : debounced);

  return (
    <div className="flex h-full flex-col">
      {!archived && (
        <div className="border-b border-gray-100 p-2">
          <div className="relative">
            <Search className="pointer-events-none absolute left-2.5 top-1/2 h-4 w-4 -translate-y-1/2 text-gray-300" />
            <Input
              value={query}
              onChange={(e) => setQuery(e.target.value)}
              placeholder="Search conversations"
              className="h-8 pl-8 text-sm"
            />
          </div>
        </div>
      )}

      <div className="min-h-0 flex-1 overflow-y-auto">
        {isLoading && <p className="p-4 text-center text-sm text-gray-400">Loading…</p>}
        {!isLoading && (threads?.length ?? 0) === 0 && (
          <p className="p-6 text-center text-sm text-gray-400">
            {archived ? "No archived conversations." : "No conversations yet."}
          </p>
        )}
        {threads?.map((t) => (
          <ThreadRow key={t.id} thread={t} onOpen={() => onOpen(t.id)} />
        ))}
      </div>

      <button
        type="button"
        onClick={() => setView(archived ? "inbox" : "archived")}
        className="flex items-center justify-center gap-1.5 border-t border-gray-100 py-2 text-xs font-medium text-gray-500 hover:bg-gray-50"
      >
        {archived ? (
          <>
            <ArchiveRestore className="h-3.5 w-3.5" /> Back to inbox
          </>
        ) : (
          <>
            <Archive className="h-3.5 w-3.5" /> Archived
          </>
        )}
      </button>
    </div>
  );
}

function ThreadRow({ thread, onOpen }: { thread: Thread; onOpen: () => void }) {
  const last = thread.last_message;
  const preview = last?.deleted_at
    ? "Message deleted"
    : last?.type === "lead_card"
      ? "Shared a lead"
      : last?.type === "attachment"
        ? "Sent an attachment"
        : (last?.body ?? "");
  return (
    <button
      type="button"
      onClick={onOpen}
      className="flex w-full items-start gap-3 border-b border-gray-50 px-3 py-2.5 text-left hover:bg-gray-50"
    >
      <div className="mt-0.5 flex h-9 w-9 shrink-0 items-center justify-center rounded-full bg-jade-50 text-sm font-semibold text-jade-700">
        {initials(thread.display_name)}
      </div>
      <div className="min-w-0 flex-1">
        <div className="flex items-center gap-2">
          <span className="truncate text-sm font-semibold text-gray-800">{thread.display_name}</span>
          {last && (
            <span className="ml-auto shrink-0 text-[11px] text-gray-400">{timeAgo(last.created_at)}</span>
          )}
        </div>
        {thread.context_label && (
          <div className="truncate text-[11px] font-medium text-jade-600">{thread.context_label}</div>
        )}
        <div className="flex items-center gap-2">
          <span className={cn("truncate text-xs", thread.unread_count > 0 ? "font-medium text-gray-700" : "text-gray-400")}>
            {preview || "No messages yet"}
          </span>
          {thread.unread_count > 0 && (
            <span className="ml-auto flex h-4 min-w-4 shrink-0 items-center justify-center rounded-full bg-jade-500 px-1 text-[10px] font-bold text-white">
              {thread.unread_count}
            </span>
          )}
        </div>
      </div>
    </button>
  );
}

function initials(name: string): string {
  return name
    .split(" ")
    .filter(Boolean)
    .slice(0, 2)
    .map((p) => p[0]?.toUpperCase())
    .join("");
}

function timeAgo(iso: string): string {
  const d = new Date(iso);
  const diff = Date.now() - d.getTime();
  const min = Math.floor(diff / 60000);
  if (min < 1) return "now";
  if (min < 60) return `${min}m`;
  const hr = Math.floor(min / 60);
  if (hr < 24) return `${hr}h`;
  const day = Math.floor(hr / 24);
  if (day < 7) return `${day}d`;
  return d.toLocaleDateString(undefined, { month: "short", day: "numeric" });
}
