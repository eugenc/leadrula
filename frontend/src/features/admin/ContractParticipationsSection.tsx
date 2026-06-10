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
  useBuyers,
} from "@/features/admin/hooks";
import { formatParticipationStatus } from "@/features/admin/contractOffer";
import type { Contract, ContractParticipation } from "@/types";

export function ContractParticipationsSection({ contract }: { contract: Contract }) {
  const { data: buyers } = useBuyers();
  const add = useAddContractParticipation();
  const invite = useContractInvite();
  const acceptCounter = useAcceptCounter();
  const rejectCounter = useRejectCounter();
  const [buyerId, setBuyerId] = useState(0);
  const [handlerId, setHandlerId] = useState("");

  const parts = contract.participations ?? [];

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
  return (
    <TR>
      <TD className="font-semibold">{part.buyer_name || `#${part.buyer_id}`}</TD>
      <TD>{formatParticipationStatus(part.status)}</TD>
      <TD>{part.delivery || "—"}</TD>
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
      </TD>
    </TR>
  );
}
