import { useState } from "react";
import { Button } from "@/components/ui/button";
import { Input, Label, Select } from "@/components/ui/input";
import { Table, THead, TH, TBody, TR, TD } from "@/components/ui/table";
import { SectionLabel } from "@/components/layout/SectionLabel";
import { toast } from "@/store/toastStore";
import { errorMessage } from "@/lib/api";
import {
  useAddContractParticipation,
  useContractInvite,
  useAcceptCounter,
  useRejectCounter,
  useReinviteParticipation,
  useBuyers,
} from "@/features/admin/hooks";
import { formatParticipationStatus } from "@/features/admin/contractOffer";
import { usePipelines, useStages } from "@/features/leads/hooks";
import type { Contract, ContractParticipation } from "@/types";

function stageName(stages: { id: number; name: string }[] | undefined, id?: number | null) {
  if (!id) return "—";
  return stages?.find((s) => s.id === id)?.name ?? `#${id}`;
}

export function ContractParticipationsSection({ contract }: { contract: Contract }) {
  const { data: buyers } = useBuyers();
  const { data: pubPipelines } = usePipelines();
  const { data: pubStages } = useStages(contract.source_pipeline_id ?? undefined);
  const add = useAddContractParticipation();
  const invite = useContractInvite();
  const acceptCounter = useAcceptCounter();
  const rejectCounter = useRejectCounter();
  const [buyerId, setBuyerId] = useState(0);
  const [handlerId, setHandlerId] = useState("");

  const parts = contract.participations ?? [];
  const pubPipelineName = pubPipelines?.find((p) => p.id === contract.source_pipeline_id)?.name;

  function addBuyer() {
    const body: Record<string, unknown> = {};
    if (buyerId) body.buyer_id = buyerId;
    else if (handlerId.trim()) body.buyer_handler_id = handlerId.trim().toUpperCase();
    else {
      toast.error("Select a buyer or enter a handler ID");
      return;
    }
    add.mutate(
      { contractId: contract.id, body },
      {
        onSuccess: () => {
          toast.success("Buyer invited");
          setBuyerId(0);
          setHandlerId("");
        },
        onError: (e) => toast.error(errorMessage(e)),
      }
    );
  }

  function copyInvite() {
    invite.mutate(contract.id, {
      onSuccess: (info) => {
        const url = `${window.location.origin}/b/contract-invite/${info.token}`;
        void navigator.clipboard.writeText(url).then(() => toast.success("Invite link copied"));
      },
      onError: (e) => toast.error(errorMessage(e)),
    });
  }

  return (
    <div className="space-y-4">
      <SectionLabel>Buyers</SectionLabel>
      <p className="text-xs text-gray-400">Add buyers to this open contract. Each buyer accepts with their own delivery and compensation.</p>

      {(contract.source_pipeline_id || contract.return_stage_id) && (
        <div className="rounded-lg border border-gray-100 bg-gray-50 px-3 py-2 text-xs text-gray-600">
          <p>
            <span className="font-semibold">Distribute from:</span>{" "}
            {pubPipelineName ?? "—"} / {stageName(pubStages, contract.source_stage_id)}
          </p>
          <p>
            <span className="font-semibold">Return destination:</span>{" "}
            {stageName(pubStages, contract.return_stage_id)}
          </p>
        </div>
      )}

      <div className="grid grid-cols-2 gap-3">
        <div>
          <Label>Buyer</Label>
          <Select value={buyerId} onChange={(e) => setBuyerId(Number(e.target.value))}>
            <option value={0}>Select…</option>
            {(buyers ?? []).map((b) => (
              <option key={b.id} value={b.id}>
                {b.name}
              </option>
            ))}
          </Select>
        </div>
        <div>
          <Label>Or handler ID</Label>
          <Input value={handlerId} onChange={(e) => setHandlerId(e.target.value)} placeholder="B-XXXXX" />
        </div>
      </div>
      <div className="flex gap-2">
        <Button variant="secondary" disabled={add.isPending} onClick={addBuyer}>
          Add buyer
        </Button>
        <Button variant="secondary" disabled={invite.isPending} onClick={copyInvite}>
          Copy invite link
        </Button>
      </div>

      {parts.length > 0 && (
        <Table>
          <THead>
            <tr>
              <TH>Buyer</TH>
              <TH>Status</TH>
              <TH>Delivery</TH>
              <TH>Distribution</TH>
              <TH />
            </tr>
          </THead>
          <TBody>
            {parts.map((p) => (
              <ParticipationRow
                key={p.id}
                part={p}
                onAcceptCounter={() =>
                  acceptCounter.mutate(p.id, {
                    onSuccess: () => toast.success("Counter accepted — new contract created"),
                    onError: (e) => toast.error(errorMessage(e)),
                  })
                }
                onRejectCounter={() =>
                  rejectCounter.mutate(p.id, {
                    onSuccess: () => toast.success("Counter rejected"),
                    onError: (e) => toast.error(errorMessage(e)),
                  })
                }
                counterLoading={acceptCounter.isPending || rejectCounter.isPending}
              />
            ))}
          </TBody>
        </Table>
      )}
    </div>
  );
}

function ParticipationRow({
  part,
  onAcceptCounter,
  onRejectCounter,
  counterLoading,
}: {
  part: ContractParticipation;
  onAcceptCounter: () => void;
  onRejectCounter: () => void;
  counterLoading: boolean;
}) {
  const reinvite = useReinviteParticipation();
  const { data: pipelines } = usePipelines();
  const { data: stages } = useStages(part.buyer_pipeline_id ?? undefined);
  const pipelineName = pipelines?.find((p) => p.id === part.buyer_pipeline_id)?.name;

  const distribution =
    part.delivery === "leads_pipeline" && part.buyer_pipeline_id
      ? `${pipelineName ?? "Pipeline"} → ${stageName(stages, part.buyer_target_stage_id)}`
      : part.delivery === "leads"
        ? "Lead inbox"
        : part.delivery || "—";

  return (
    <TR>
      <TD className="font-semibold">{part.buyer_name || `#${part.buyer_id}`}</TD>
      <TD>{formatParticipationStatus(part.status)}</TD>
      <TD>{part.delivery || "—"}</TD>
      <TD className="text-sm text-gray-600">{part.status === "active" ? distribution : "—"}</TD>
      <TD>
        {part.status === "counter_pending" && (
          <div className="flex justify-end gap-2">
            <Button className="h-8 px-2 text-xs" disabled={counterLoading} onClick={onAcceptCounter}>
              Accept counter
            </Button>
            <Button className="h-8 px-2 text-xs" variant="secondary" disabled={counterLoading} onClick={onRejectCounter}>
              Reject
            </Button>
          </div>
        )}
        {(part.status === "withdrawn" || part.status === "declined") && (
          <div className="flex justify-end">
            <Button
              className="h-8 px-2 text-xs"
              variant="secondary"
              disabled={reinvite.isPending}
              onClick={() => {
                if (
                  !window.confirm(
                    "Send a new contract invitation to this buyer? They will need to accept again."
                  )
                ) {
                  return;
                }
                reinvite.mutate(part.id, {
                  onSuccess: () => toast.success("Invitation resent"),
                  onError: (e) => toast.error(errorMessage(e)),
                });
              }}
            >
              Resend invite
            </Button>
          </div>
        )}
      </TD>
    </TR>
  );
}
