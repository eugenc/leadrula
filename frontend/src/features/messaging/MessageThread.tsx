import { useEffect, useRef, useState } from "react";
import {
  Send,
  Paperclip,
  Reply,
  X,
  MoreVertical,
  BellOff,
  Bell,
  Ban,
  Archive,
  ArchiveRestore,
  Pencil,
  Trash2,
  FileText,
  UserPlus,
} from "lucide-react";
import { cn } from "@/lib/utils";
import { errorMessage, getBlob, messagingNs } from "@/lib/api";
import { toast } from "@/store/toastStore";
import { useAuthStore } from "@/store/authStore";
import {
  useThread,
  useMessages,
  useSendMessage,
  useMarkRead,
  useSetMuted,
  useSetArchived,
  useBlockThread,
  useUnblockThread,
  useEditMessage,
  useDeleteMessage,
  useAcceptInvite,
} from "./hooks";
import { sendTyping, useTypingNames } from "./useMessagingSocket";
import { LeadSharePicker } from "./LeadSharePicker";
import type { Message, Thread } from "./types";

export function MessageThread({ threadId, onBack }: { threadId: string; onBack: () => void }) {
  const { data: thread } = useThread(threadId);
  const { data: messages, isLoading } = useMessages(threadId);
  const markRead = useMarkRead();
  const typingNames = useTypingNames(threadId);
  const bottomRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    if (threadId) markRead.mutate(threadId);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [threadId, messages?.length]);

  useEffect(() => {
    bottomRef.current?.scrollIntoView({ block: "end" });
  }, [messages?.length, typingNames.length]);

  return (
    <div className="flex h-full flex-col">
      {thread && <ThreadHeader thread={thread} onBack={onBack} />}
      <div className="min-h-0 flex-1 space-y-2 overflow-y-auto px-3 py-3">
        {isLoading && <p className="text-center text-sm text-gray-400">Loading…</p>}
        {messages?.map((m) => (
          <MessageBubble key={m.id} message={m} onReplyRef={setReplyTargetRef} />
        ))}
        {typingNames.length > 0 && <TypingIndicator names={typingNames} />}
        <div ref={bottomRef} />
      </div>
      {thread && <Composer thread={thread} />}
    </div>
  );
}

// Small module-level bridge so a bubble's Reply button can set the composer target.
let setReplyTargetRef: (m: Message) => void = () => {};

function ThreadHeader({ thread, onBack }: { thread: Thread; onBack: () => void }) {
  const [menu, setMenu] = useState(false);
  const setMuted = useSetMuted();
  const setArchived = useSetArchived();
  const block = useBlockThread();
  const unblock = useUnblockThread();

  const act = (fn: () => void) => {
    setMenu(false);
    fn();
  };

  return (
    <div className="relative flex items-center gap-2 border-b border-gray-100 px-3 py-2">
      <div className="min-w-0 flex-1">
        <div className="truncate text-sm font-semibold text-gray-800">{thread.display_name}</div>
        {thread.context_label && (
          <div className="truncate text-[11px] font-medium text-jade-600">{thread.context_label}</div>
        )}
      </div>
      <button
        type="button"
        aria-label="Thread options"
        onClick={() => setMenu((v) => !v)}
        className="flex h-7 w-7 items-center justify-center rounded-md text-gray-400 hover:bg-gray-100 hover:text-gray-700"
      >
        <MoreVertical className="h-4 w-4" />
      </button>
      {menu && (
        <div className="absolute right-2 top-11 z-10 w-44 rounded-md border border-gray-200 bg-surface-card py-1 shadow-lg">
          <MenuItem
            icon={thread.muted ? <Bell className="h-4 w-4" /> : <BellOff className="h-4 w-4" />}
            label={thread.muted ? "Unmute" : "Mute"}
            onClick={() => act(() => setMuted.mutate({ threadId: thread.id, muted: !thread.muted }))}
          />
          {thread.status === "blocked" ? (
            thread.blocked_by_me && (
              <MenuItem
                icon={<Ban className="h-4 w-4" />}
                label="Unblock"
                onClick={() => act(() => unblock.mutate(thread.id))}
              />
            )
          ) : (
            <MenuItem
              icon={<Ban className="h-4 w-4" />}
              label="Block"
              onClick={() => act(() => block.mutate(thread.id))}
            />
          )}
          <MenuItem
            icon={<Archive className="h-4 w-4" />}
            label="Archive"
            onClick={() => act(() => {
              setArchived.mutate({ threadId: thread.id, archived: true });
              onBack();
            })}
          />
          <MenuItem
            icon={<ArchiveRestore className="h-4 w-4" />}
            label="Unarchive"
            onClick={() => act(() => setArchived.mutate({ threadId: thread.id, archived: false }))}
          />
        </div>
      )}
    </div>
  );
}

function MenuItem({ icon, label, onClick }: { icon: React.ReactNode; label: string; onClick: () => void }) {
  return (
    <button
      type="button"
      onClick={onClick}
      className="flex w-full items-center gap-2 px-3 py-1.5 text-left text-sm text-gray-700 hover:bg-gray-50"
    >
      {icon}
      {label}
    </button>
  );
}

function MessageBubble({ message, onReplyRef }: { message: Message; onReplyRef: (m: Message) => void }) {
  const [editing, setEditing] = useState(false);
  const [draft, setDraft] = useState(message.body ?? "");
  const edit = useEditMessage();
  const del = useDeleteMessage();
  const mine = message.mine;

  if (message.type === "system") {
    return <div className="text-center text-[11px] text-gray-400">{message.body}</div>;
  }

  return (
    <div className={cn("group flex flex-col", mine ? "items-end" : "items-start")}>
      {!mine && <span className="mb-0.5 px-1 text-[11px] font-medium text-gray-500">{message.sender_name}</span>}
      <div className={cn("flex items-end gap-1", mine && "flex-row-reverse")}>
        <div
          className={cn(
            "max-w-[240px] rounded-2xl px-3 py-1.5 text-sm",
            mine ? "bg-jade-500 text-white" : "bg-gray-100 text-gray-800"
          )}
        >
          {message.reply_to && (
            <div className={cn("mb-1 border-l-2 pl-2 text-xs", mine ? "border-white/50 text-white/80" : "border-gray-300 text-gray-500")}>
              <div className="font-medium">{message.reply_to.sender_name}</div>
              <div className="line-clamp-2">{message.reply_to.body ?? "Message"}</div>
            </div>
          )}
          {message.deleted_at ? (
            <span className="italic opacity-70">Message deleted</span>
          ) : editing ? (
            <div className="flex flex-col gap-1">
              <textarea
                value={draft}
                onChange={(e) => setDraft(e.target.value)}
                className="w-48 rounded-md p-1 text-sm text-gray-800"
                rows={2}
              />
              <div className="flex gap-1">
                <button
                  type="button"
                  className="rounded bg-white/20 px-2 py-0.5 text-xs"
                  onClick={() =>
                    edit.mutate(
                      { messageId: message.id, body: draft },
                      { onSuccess: () => setEditing(false), onError: (e) => toast.error(errorMessage(e)) }
                    )
                  }
                >
                  Save
                </button>
                <button type="button" className="px-2 py-0.5 text-xs" onClick={() => setEditing(false)}>
                  Cancel
                </button>
              </div>
            </div>
          ) : (
            <>
              {message.lead_card && <LeadCardBubble card={message.lead_card} mine={mine} />}
              {message.body && <span className="whitespace-pre-wrap break-words">{message.body}</span>}
              {message.attachments?.map((a) => (
                <button
                  key={a.id}
                  type="button"
                  onClick={() => openAttachment(a.id, a.filename)}
                  className={cn("mt-1 flex items-center gap-1 text-xs underline", mine ? "text-white" : "text-jade-600")}
                >
                  <FileText className="h-3.5 w-3.5" /> {a.filename}
                </button>
              ))}
              {message.edited_at && <span className="ml-1 text-[10px] opacity-60">(edited)</span>}
            </>
          )}
        </div>
        {!message.deleted_at && !editing && (
          <div className="flex items-center gap-0.5 opacity-0 transition-opacity group-hover:opacity-100">
            <button
              type="button"
              aria-label="Reply"
              onClick={() => onReplyRef(message)}
              className="flex h-6 w-6 items-center justify-center rounded text-gray-400 hover:bg-gray-100 hover:text-gray-700"
            >
              <Reply className="h-3.5 w-3.5" />
            </button>
            {message.can_edit && (
              <button
                type="button"
                aria-label="Edit"
                onClick={() => {
                  setDraft(message.body ?? "");
                  setEditing(true);
                }}
                className="flex h-6 w-6 items-center justify-center rounded text-gray-400 hover:bg-gray-100 hover:text-gray-700"
              >
                <Pencil className="h-3.5 w-3.5" />
              </button>
            )}
            {message.can_delete && (
              <button
                type="button"
                aria-label="Delete"
                onClick={() => del.mutate(message.id, { onError: (e) => toast.error(errorMessage(e)) })}
                className="flex h-6 w-6 items-center justify-center rounded text-gray-400 hover:bg-gray-100 hover:text-danger"
              >
                <Trash2 className="h-3.5 w-3.5" />
              </button>
            )}
          </div>
        )}
      </div>
      <span className="mt-0.5 px-1 text-[10px] text-gray-300">{formatTime(message.created_at)}</span>
    </div>
  );
}

function LeadCardBubble({ card, mine }: { card: { name: string; phone?: string; city?: string; state?: string }; mine: boolean }) {
  return (
    <div className={cn("mb-1 rounded-lg border p-2", mine ? "border-white/30 bg-white/10" : "border-gray-200 bg-white")}>
      <div className="text-xs font-semibold">{card.name || "Lead"}</div>
      {card.phone && <div className="text-[11px] opacity-80">{card.phone}</div>}
      {(card.city || card.state) && (
        <div className="text-[11px] opacity-80">{[card.city, card.state].filter(Boolean).join(", ")}</div>
      )}
    </div>
  );
}

function TypingIndicator({ names }: { names: string[] }) {
  const label =
    names.length === 1
      ? `${names[0]} is typing…`
      : names.length === 2
        ? `${names[0]} and ${names[1]} are typing…`
        : "Several people are typing…";
  return <div className="px-1 text-[11px] italic text-gray-400">{label}</div>;
}

function Composer({ thread }: { thread: Thread }) {
  const [text, setText] = useState("");
  const [reply, setReply] = useState<Message | null>(null);
  const [files, setFiles] = useState<File[]>([]);
  const [showLeadPicker, setShowLeadPicker] = useState(false);
  const send = useSendMessage();
  const acceptInvite = useAcceptInvite();
  const fileRef = useRef<HTMLInputElement>(null);
  const typingSent = useRef(false);

  useEffect(() => {
    setReplyTargetRef = (m: Message) => setReply(m);
  }, []);

  const pendingInvite = thread.members?.some((m) => m.invite_status === "pending");

  const onType = (v: string) => {
    setText(v);
    if (!typingSent.current && v.length > 0) {
      typingSent.current = true;
      sendTyping(thread.id, true);
      setTimeout(() => {
        typingSent.current = false;
      }, 2000);
    }
  };

  const submit = (leadId?: string) => {
    if (!text.trim() && files.length === 0 && !leadId) return;
    send.mutate(
      { threadId: thread.id, body: text, replyToId: reply?.id, leadId, files: files.length ? files : undefined },
      {
        onSuccess: () => {
          setText("");
          setReply(null);
          setFiles([]);
          sendTyping(thread.id, false);
        },
        onError: (e) => toast.error(errorMessage(e)),
      }
    );
  };

  if (thread.status === "blocked") {
    return <div className="border-t border-gray-100 p-3 text-center text-xs text-gray-400">This conversation is blocked.</div>;
  }
  if (thread.status === "pending") {
    return (
      <div className="border-t border-gray-100 p-3 text-center text-xs text-gray-400">
        Waiting for your connect request to be accepted.
      </div>
    );
  }
  if (pendingInvite) {
    return (
      <div className="flex items-center justify-between gap-2 border-t border-gray-100 p-3">
        <span className="text-xs text-gray-500">You have a pending invite to this group.</span>
        <button
          type="button"
          onClick={() => acceptInvite.mutate(thread.id)}
          className="flex items-center gap-1 rounded-md bg-jade-500 px-2.5 py-1 text-xs font-semibold text-white hover:bg-jade-600"
        >
          <UserPlus className="h-3.5 w-3.5" /> Accept
        </button>
      </div>
    );
  }

  return (
    <div className="border-t border-gray-100 p-2">
      {reply && (
        <div className="mb-1.5 flex items-start gap-2 rounded-md bg-gray-50 px-2 py-1 text-xs">
          <div className="min-w-0 flex-1">
            <div className="font-medium text-gray-600">Replying to {reply.sender_name}</div>
            <div className="line-clamp-1 text-gray-400">{reply.body ?? "Message"}</div>
          </div>
          <button type="button" onClick={() => setReply(null)} aria-label="Cancel reply">
            <X className="h-3.5 w-3.5 text-gray-400" />
          </button>
        </div>
      )}
      {files.length > 0 && (
        <div className="mb-1.5 flex flex-wrap gap-1">
          {files.map((f, i) => (
            <span key={i} className="flex items-center gap-1 rounded bg-gray-100 px-1.5 py-0.5 text-[11px] text-gray-600">
              {f.name}
              <button type="button" onClick={() => setFiles(files.filter((_, j) => j !== i))} aria-label="Remove">
                <X className="h-3 w-3" />
              </button>
            </span>
          ))}
        </div>
      )}
      <div className="flex items-end gap-1.5">
        <button
          type="button"
          aria-label="Attach file"
          onClick={() => fileRef.current?.click()}
          className="flex h-8 w-8 shrink-0 items-center justify-center rounded-md text-gray-400 hover:bg-gray-100 hover:text-gray-700"
        >
          <Paperclip className="h-4 w-4" />
        </button>
        <button
          type="button"
          aria-label="Share a lead"
          onClick={() => setShowLeadPicker(true)}
          className="flex h-8 w-8 shrink-0 items-center justify-center rounded-md text-gray-400 hover:bg-gray-100 hover:text-gray-700"
        >
          <FileText className="h-4 w-4" />
        </button>
        <input
          ref={fileRef}
          type="file"
          multiple
          className="hidden"
          onChange={(e) => setFiles([...files, ...Array.from(e.target.files ?? [])])}
        />
        <textarea
          value={text}
          onChange={(e) => onType(e.target.value)}
          onBlur={() => sendTyping(thread.id, false)}
          onKeyDown={(e) => {
            if (e.key === "Enter" && !e.shiftKey) {
              e.preventDefault();
              submit();
            }
          }}
          placeholder="Type a message…"
          rows={1}
          className="max-h-24 min-h-[32px] flex-1 resize-none rounded-md border border-gray-200 px-2.5 py-1.5 text-sm outline-none focus:border-jade-500"
        />
        <button
          type="button"
          aria-label="Send"
          onClick={() => submit()}
          disabled={send.isPending}
          className="flex h-8 w-8 shrink-0 items-center justify-center rounded-md bg-jade-500 text-white hover:bg-jade-600 disabled:opacity-40"
        >
          <Send className="h-4 w-4" />
        </button>
      </div>
      {showLeadPicker && (
        <LeadSharePicker
          onClose={() => setShowLeadPicker(false)}
          onPick={(leadId) => {
            setShowLeadPicker(false);
            submit(leadId);
          }}
        />
      )}
    </div>
  );
}

async function openAttachment(id: string, filename: string) {
  try {
    const blob = await getBlob(`${messagingNs()}/messages/attachments/${id}`);
    const url = URL.createObjectURL(blob);
    const a = document.createElement("a");
    a.href = url;
    a.download = filename;
    a.click();
    setTimeout(() => URL.revokeObjectURL(url), 5000);
  } catch (e) {
    toast.error(errorMessage(e));
  }
}

function formatTime(iso: string): string {
  return new Date(iso).toLocaleTimeString(undefined, { hour: "numeric", minute: "2-digit" });
}
