import { Sheet, DrawerHeader, DrawerBody } from "@/components/ui/dialog";
import { Badge, Spinner } from "@/components/ui/misc";
import { Button } from "@/components/ui/button";
import { SectionLabel } from "@/components/layout/SectionLabel";
import { formatMoney } from "@/lib/utils";
import { useUIStore } from "@/store/uiStore";
import { RecordingPlayer } from "@/features/calls/RecordingPlayer";
import { formatCallStatus } from "@/features/calls/format";
import { DispositionEditor } from "@/features/calls/DispositionEditor";
import { useCallDetail, useBuyerCallDetail } from "@/features/calls/hooks";
import type { Call } from "@/types";

function fmtTime(iso?: string | null) {
  return iso ? new Date(iso).toLocaleString() : "—";
}

export function CallDetailDrawer({
  callId,
  onClose,
  role = "publisher",
}: {
  callId: number | null;
  onClose: () => void;
  role?: "publisher" | "buyer";
}) {
  const pub = useCallDetail(role === "publisher" ? callId : null);
  const buyer = useBuyerCallDetail(role === "buyer" ? callId : null);
  const { data: call, isLoading } = role === "publisher" ? pub : buyer;
  return (
    <Sheet open={callId != null} onClose={onClose} width={560}>
      {isLoading || !call ? (
        <div className="flex h-full items-center justify-center">
          <Spinner />
        </div>
      ) : (
        <Content call={call} onClose={onClose} role={role} />
      )}
    </Sheet>
  );
}

function Content({ call, onClose, role }: { call: Call; onClose: () => void; role: "publisher" | "buyer" }) {
  const openDetail = useUIStore((s) => s.openDetail);
  const isBuyer = role === "buyer";

  function viewLead() {
    if (!call.lead_id) return;
    onClose();
    openDetail(call.lead_id);
  }

  return (
    <div className="flex h-full flex-col">
      <DrawerHeader
        title={`Call ${call.public_id?.slice(0, 8) ?? call.id}`}
        subtitle={`${formatCallStatus(call.status)} · ${call.billable ? "Billable" : "Not billable"}`}
        onClose={onClose}
      />
      <DrawerBody>
        <div className="space-y-4">
          <div className="grid grid-cols-2 gap-3 text-sm">
            <Field label="Caller" value={call.caller_number ?? "—"} />
            <Field label="State" value={call.caller_state ?? "—"} />
            <Field label="Tracking number" value={call.tracking_number ?? "—"} />
            <Field label="Contract" value={call.contract_name ?? "—"} />
            <Field label="Duration" value={`${call.duration_sec}s`} />
            <Field label="Billable duration" value={`${call.billable_duration_sec}s`} />
            <Field label="Price" value={call.price_cents ? formatMoney(call.price_cents / 100) : "—"} />
            <Field label="Winning buyer" value={call.winner_buyer_name ?? "—"} />
            <Field label="Connected at" value={fmtTime(call.connected_at)} />
            <Field label="Ended at" value={fmtTime(call.ended_at)} />
            {!isBuyer && <Field label="Disposition" value={call.disposition ?? "—"} />}
          </div>

          {isBuyer && call.lead_id && (
            <Button variant="outline" size="sm" onClick={viewLead}>
              View lead
            </Button>
          )}

          {isBuyer && <DispositionEditor call={call} />}

          {call.recording_url && (
            <div>
              <SectionLabel>Recording</SectionLabel>
              <RecordingPlayer role={role} callId={call.id} />
            </div>
          )}

          <div>
            <SectionLabel>Legs</SectionLabel>
            {(call.legs ?? []).length === 0 ? (
              <p className="mt-1 text-sm text-gray-400">No legs.</p>
            ) : (
              <div className="mt-2 space-y-1">
                {(call.legs ?? []).map((l) => (
                  <div key={l.id} className="flex items-center justify-between rounded border border-gray-100 px-3 py-2 text-sm">
                    <span>
                      Tier {l.tier} · {l.buyer_name ?? "—"} → {l.destination_number ?? "—"}
                    </span>
                    <span className="flex items-center gap-2 text-gray-500">
                      {l.billed && <Badge>billed {formatMoney(l.rate)}</Badge>}
                      <Badge>{l.leg_status}</Badge>
                    </span>
                  </div>
                ))}
              </div>
            )}
          </div>

          {role === "publisher" && (
            <div>
              <SectionLabel>RTB pings</SectionLabel>
              {(call.rtb_pings ?? []).length === 0 ? (
                <p className="mt-1 text-sm text-gray-400">No RTB pings.</p>
              ) : (
                <div className="mt-2 space-y-1">
                  {(call.rtb_pings ?? []).map((p) => (
                    <div key={p.id} className="rounded border border-gray-100 px-3 py-2 text-sm">
                      <div className="flex items-center justify-between">
                        <span className="truncate font-mono text-xs">{p.endpoint}</span>
                        <Badge>{p.accepted ? "accepted" : "rejected"}</Badge>
                      </div>
                      {p.bid_amount != null && <div className="text-xs text-gray-500">Bid {formatMoney(p.bid_amount)}</div>}
                      {p.reason && <div className="text-xs text-gray-500">{p.reason}</div>}
                    </div>
                  ))}
                </div>
              )}
            </div>
          )}
        </div>
      </DrawerBody>
    </div>
  );
}

function Field({ label, value }: { label: string; value: string }) {
  return (
    <div>
      <div className="text-xs font-semibold uppercase tracking-wide text-gray-400">{label}</div>
      <div className="mt-0.5 text-gray-700">{value}</div>
    </div>
  );
}
