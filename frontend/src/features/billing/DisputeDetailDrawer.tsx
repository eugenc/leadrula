import { useState } from "react";
import { FormDrawer } from "@/components/ui/dialog";
import { Button } from "@/components/ui/button";
import { Label, Select, Textarea } from "@/components/ui/input";
import { Badge, Spinner } from "@/components/ui/misc";
import { usePipelines, useStages } from "@/features/leads/hooks";
import {
  useDisputeMessages,
  usePostDisputeMessage,
  useAcceptDispute,
  useRejectDispute,
  useSubmitDisputePlacement,
  openDisputeAttachment,
} from "@/features/admin/hooks";
import { useAuthStore } from "@/store/authStore";
import { toast } from "@/store/toastStore";
import { errorMessage } from "@/lib/api";
import { formatMoney } from "@/lib/utils";
import { format, formatDistanceToNowStrict } from "date-fns";
import { Paperclip } from "lucide-react";
import type { Dispute, DisputeMessage, DisputeParty } from "@/types";

type Scope = "publisher" | "buyer";

function deadlineLabel(d: Dispute): string | null {
  if (d.status !== "open" || !d.response_deadline_at) return null;
  const due = new Date(d.response_deadline_at);
  if (due.getTime() <= Date.now()) return "Deadline passed";
  return `Responds in ${formatDistanceToNowStrict(due)}`;
}

// PipelineStagePicker selects a pipeline + stage on the current account.
function PipelineStagePicker({
  pipelineId,
  stageId,
  onChange,
}: {
  pipelineId: number | undefined;
  stageId: number | undefined;
  onChange: (pipelineId: number | undefined, stageId: number | undefined) => void;
}) {
  const { data: pipelines } = usePipelines();
  const { data: stages } = useStages(pipelineId);
  return (
    <div className="flex flex-col gap-2">
      <div>
        <Label>Pipeline</Label>
        <Select
          value={pipelineId ?? ""}
          onChange={(e) => onChange(Number(e.target.value) || undefined, undefined)}
        >
          <option value="">Select a pipeline…</option>
          {(pipelines ?? []).map((p) => (
            <option key={p.id} value={p.id}>
              {p.name}
            </option>
          ))}
        </Select>
      </div>
      <div>
        <Label>Stage</Label>
        <Select
          value={stageId ?? ""}
          onChange={(e) => onChange(pipelineId, Number(e.target.value) || undefined)}
          disabled={!pipelineId}
        >
          <option value="">Select a stage…</option>
          {(stages ?? []).map((s) => (
            <option key={s.id} value={s.id}>
              {s.name}
            </option>
          ))}
        </Select>
      </div>
    </div>
  );
}

function MessageBubble({ msg, scope }: { msg: DisputeMessage; scope: Scope }) {
  const mine = msg.author_party === scope;
  return (
    <div className={mine ? "flex justify-end" : "flex justify-start"}>
      <div
        className={
          "max-w-[80%] rounded-lg px-3 py-2 text-sm " +
          (mine ? "bg-jade-500 text-white" : "bg-neutral-bg text-gray-800")
        }
      >
        <div className="mb-0.5 text-[11px] font-semibold opacity-80">
          {msg.author_name || (msg.author_party === "publisher" ? "Publisher" : "Buyer")}
          {msg.kind === "reject" && " · rejected"}
          {msg.kind === "system" && " · system"}
        </div>
        {msg.body && <div className="whitespace-pre-wrap">{msg.body}</div>}
        {(msg.attachments ?? []).map((a) => (
          <button
            key={a.id}
            type="button"
            onClick={() =>
              openDisputeAttachment(scope, a.id, a.filename).catch((e) => toast.error(errorMessage(e)))
            }
            className={
              "mt-1 flex items-center gap-1 text-xs underline " +
              (mine ? "text-white/90" : "text-jade-700")
            }
          >
            <Paperclip className="h-3 w-3" />
            {a.filename}
          </button>
        ))}
        <div className="mt-1 text-[10px] opacity-60">
          {format(new Date(msg.created_at), "MMM d, h:mma")}
        </div>
      </div>
    </div>
  );
}

export function DisputeDetailDrawer({
  scope,
  dispute,
  onClose,
}: {
  scope: Scope;
  dispute: Dispute;
  onClose: () => void;
}) {
  const accountType = useAuthStore((s) => s.user?.account_type);
  const myParty: DisputeParty = accountType === "publisher" ? "publisher" : "buyer";

  const { data: messages, isLoading } = useDisputeMessages(scope, dispute.id);
  const postMessage = usePostDisputeMessage(scope);
  const accept = useAcceptDispute(scope);
  const reject = useRejectDispute(scope);
  const placement = useSubmitDisputePlacement(scope);

  const [composer, setComposer] = useState("");
  const [composerFiles, setComposerFiles] = useState<File[]>([]);
  const [rejectMode, setRejectMode] = useState(false);
  const [rejectBody, setRejectBody] = useState("");
  const [rejectFiles, setRejectFiles] = useState<File[]>([]);
  const [pipelineId, setPipelineId] = useState<number | undefined>();
  const [stageId, setStageId] = useState<number | undefined>();

  const isOpen = dispute.status === "open";
  const myTurn = isOpen && dispute.awaiting_party === myParty;

  // Whether accepting now hands the lead to me (so I must choose where it lands).
  const acceptNeedsPlacement =
    (dispute.initiated_by === "publisher" && myParty === "buyer") ||
    (dispute.initiated_by === "buyer" && myParty === "publisher");

  const needsClosedPlacement =
    !isOpen && dispute.placement_party === myParty && !dispute.placement_completed_at;

  const counterpartyLabel = dispute.initiated_by === "publisher" ? "Publisher" : "Buyer";

  function handleAccept() {
    if (acceptNeedsPlacement && (!pipelineId || !stageId)) {
      toast.error("Choose a pipeline and stage for the lead");
      return;
    }
    accept.mutate(
      { id: dispute.id, pipelineId, stageId },
      {
        onSuccess: () => {
          toast.success("Dispute accepted");
          onClose();
        },
        onError: (e) => toast.error(errorMessage(e)),
      }
    );
  }

  function handleReject() {
    reject.mutate(
      { id: dispute.id, body: rejectBody, files: rejectFiles },
      {
        onSuccess: () => {
          toast.info("Response sent");
          setRejectMode(false);
          setRejectBody("");
          setRejectFiles([]);
        },
        onError: (e) => toast.error(errorMessage(e)),
      }
    );
  }

  function handlePostMessage() {
    if (!composer.trim() && composerFiles.length === 0) return;
    postMessage.mutate(
      { id: dispute.id, body: composer, files: composerFiles },
      {
        onSuccess: () => {
          setComposer("");
          setComposerFiles([]);
        },
        onError: (e) => toast.error(errorMessage(e)),
      }
    );
  }

  function handlePlacement() {
    if (!pipelineId || !stageId) {
      toast.error("Choose a pipeline and stage for the lead");
      return;
    }
    placement.mutate(
      { id: dispute.id, pipelineId, stageId },
      {
        onSuccess: () => {
          toast.success("Lead placed");
          onClose();
        },
        onError: (e) => toast.error(errorMessage(e)),
      }
    );
  }

  const deadline = deadlineLabel(dispute);

  return (
    <FormDrawer
      open
      onClose={onClose}
      title="Dispute"
      subtitle={`${counterpartyLabel}-initiated · ${formatMoney(dispute.amount ?? 0)}`}
      width={520}
    >
      <div className="flex flex-col gap-3">
        <div className="flex flex-wrap items-center gap-2">
          <Badge variant={isOpen ? "pending" : "closed"}>
            {isOpen ? "Open" : dispute.outcome === "withdrawn" ? "Withdrawn" : "Resolved"}
          </Badge>
          {dispute.lead_name && <span className="text-sm text-gray-600">{dispute.lead_name}</span>}
          {deadline && <span className="text-xs text-warning-fg">{deadline}</span>}
        </div>

        <div className="flex max-h-[40vh] flex-col gap-2 overflow-y-auto rounded-md border border-gray-100 p-3">
          {isLoading ? (
            <Spinner className="h-5 w-5" />
          ) : (messages ?? []).length === 0 ? (
            <p className="text-sm text-gray-400">No messages yet.</p>
          ) : (
            (messages ?? []).map((m) => <MessageBubble key={m.id} msg={m} scope={scope} />)
          )}
        </div>

        {/* Plain message composer (always available to both parties). */}
        <div className="flex flex-col gap-1.5">
          <Textarea
            value={composer}
            onChange={(e) => setComposer(e.target.value)}
            placeholder="Write a message…"
            rows={2}
          />
          <div className="flex items-center justify-between">
            <input
              type="file"
              multiple
              className="text-xs"
              onChange={(e) => setComposerFiles(Array.from(e.target.files ?? []))}
            />
            <Button size="sm" variant="secondary" onClick={handlePostMessage} disabled={postMessage.isPending}>
              Send
            </Button>
          </div>
        </div>

        {/* Responder actions while open. */}
        {myTurn && !rejectMode && (
          <div className="flex flex-col gap-2 rounded-md border border-gray-100 p-3">
            {acceptNeedsPlacement && (
              <>
                <p className="text-xs text-gray-500">
                  Accepting moves the lead to your pipeline. Choose where it lands:
                </p>
                <PipelineStagePicker
                  pipelineId={pipelineId}
                  stageId={stageId}
                  onChange={(p, s) => {
                    setPipelineId(p);
                    setStageId(s);
                  }}
                />
              </>
            )}
            <div className="flex justify-end gap-2">
              <Button size="sm" onClick={handleAccept} disabled={accept.isPending}>
                Accept
              </Button>
              <Button size="sm" variant="secondary" onClick={() => setRejectMode(true)}>
                Reject
              </Button>
            </div>
          </div>
        )}

        {myTurn && rejectMode && (
          <div className="flex flex-col gap-2 rounded-md border border-gray-100 p-3">
            <Label>Reason for rejecting</Label>
            <Textarea
              value={rejectBody}
              onChange={(e) => setRejectBody(e.target.value)}
              placeholder="Explain why you reject…"
              rows={3}
            />
            <input
              type="file"
              multiple
              className="text-xs"
              onChange={(e) => setRejectFiles(Array.from(e.target.files ?? []))}
            />
            <div className="flex justify-end gap-2">
              <Button size="sm" variant="secondary" onClick={() => setRejectMode(false)}>
                Cancel
              </Button>
              <Button size="sm" onClick={handleReject} disabled={reject.isPending}>
                Send rejection
              </Button>
            </div>
          </div>
        )}

        {!isOpen && !myTurn && dispute.awaiting_party && dispute.status === "open" && (
          <p className="text-xs text-gray-400">Waiting for the other party to respond.</p>
        )}

        {/* Closed dispute that auto-resolved and now needs me to place the lead. */}
        {needsClosedPlacement && (
          <div className="flex flex-col gap-2 rounded-md border border-warning-border bg-warning-bg p-3">
            <p className="text-xs font-semibold text-warning-fg">
              This dispute closed automatically. Choose where the lead should land.
            </p>
            <PipelineStagePicker
              pipelineId={pipelineId}
              stageId={stageId}
              onChange={(p, s) => {
                setPipelineId(p);
                setStageId(s);
              }}
            />
            <div className="flex justify-end">
              <Button size="sm" onClick={handlePlacement} disabled={placement.isPending}>
                Place lead
              </Button>
            </div>
          </div>
        )}
      </div>
    </FormDrawer>
  );
}
