import { Badge, Spinner } from "@/components/ui/misc";
import { SectionLabel } from "@/components/layout/SectionLabel";
import { formatMoney } from "@/lib/utils";
import { RecordingPlayer } from "@/features/calls/RecordingPlayer";
import { formatCallStatus } from "@/features/calls/format";
import { DispositionEditor } from "@/features/calls/DispositionEditor";
import { useLeadCall } from "@/features/calls/hooks";

function fmtTime(iso?: string | null) {
  return iso ? new Date(iso).toLocaleString() : "—";
}

export function LeadCallTab({ leadId, accountType }: { leadId: number; accountType?: string }) {
  const { data: call, isLoading } = useLeadCall(leadId, accountType);

  if (isLoading) {
    return (
      <div className="flex justify-center py-8">
        <Spinner />
      </div>
    );
  }
  if (!call) return <p className="text-sm text-gray-400">No call linked to this lead.</p>;

  const isBuyer = accountType === "buyer";

  return (
    <div className="flex flex-col gap-4">
      <div className="grid grid-cols-2 gap-3 text-sm">
        <Info label="Status" value={formatCallStatus(call.status)} />
        <Info label="Billable" value={call.billable ? "Yes" : "No"} />
        {!isBuyer && <Info label="Caller" value={call.caller_number ?? "—"} />}
        <Info label="Duration" value={`${call.duration_sec}s`} />
        <Info label="Price" value={call.price_cents ? formatMoney(call.price_cents / 100) : "—"} />
        <Info label="Connected" value={fmtTime(call.connected_at)} />
      </div>

      {call.recording_url && (
        <div>
          <SectionLabel>Recording</SectionLabel>
          <RecordingPlayer role={isBuyer ? "buyer" : "publisher"} callId={call.id} />
        </div>
      )}

      {isBuyer ? (
        <DispositionEditor call={call} />
      ) : (
        <div>
          <SectionLabel>Disposition</SectionLabel>
          <p className="mt-1 text-sm text-gray-700">{call.disposition ?? "—"}</p>
          {call.disposition_note && <p className="mt-1 text-sm text-gray-500">{call.disposition_note}</p>}
        </div>
      )}

      {!isBuyer && (call.legs ?? []).length > 0 && (
        <div>
          <SectionLabel>Legs</SectionLabel>
          <div className="mt-2 space-y-1">
            {(call.legs ?? []).map((l) => (
              <div key={l.id} className="flex items-center justify-between rounded border border-gray-100 px-3 py-2 text-sm">
                <span>
                  Tier {l.tier} · {l.buyer_name ?? "—"}
                </span>
                <Badge>{l.leg_status}</Badge>
              </div>
            ))}
          </div>
        </div>
      )}
    </div>
  );
}

function Info({ label, value }: { label: string; value: React.ReactNode }) {
  return (
    <div>
      <div className="text-xs font-semibold uppercase tracking-wide text-gray-400">{label}</div>
      <div className="mt-0.5 text-gray-700">{value}</div>
    </div>
  );
}
