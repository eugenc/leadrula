import { useEffect, useState } from "react";
import { Button } from "@/components/ui/button";
import { Input, Label } from "@/components/ui/input";
import { toast } from "@/store/toastStore";
import { errorMessage } from "@/lib/api";
import { useCallTarget, useSaveCallTarget } from "@/features/calls/hooks";

// Publisher-owned routing knobs for a buyer's call participation: priority tier,
// simuldial weight, rate override, and caps. Also shows the buyer's saved
// destination so the publisher has full visibility.
export function PublisherCallTargetEditor({ participationId }: { participationId: number }) {
  const { data, isLoading } = useCallTarget(participationId, "publisher");
  const save = useSaveCallTarget("publisher");

  const [priority, setPriority] = useState(1);
  const [weight, setWeight] = useState(0);
  const [rateOverride, setRateOverride] = useState("");
  const [dailyCap, setDailyCap] = useState("");
  const [monthlyCap, setMonthlyCap] = useState("");
  const [concurrencyCap, setConcurrencyCap] = useState("");

  useEffect(() => {
    if (!data) return;
    setPriority(data.priority || 1);
    setWeight(data.weight || 0);
    setRateOverride(data.rate_override != null ? String(data.rate_override) : "");
    setDailyCap(data.daily_cap != null ? String(data.daily_cap) : "");
    setMonthlyCap(data.monthly_cap != null ? String(data.monthly_cap) : "");
    setConcurrencyCap(data.concurrency_cap != null ? String(data.concurrency_cap) : "");
  }, [data]);

  if (isLoading) return <p className="px-3 py-2 text-sm text-gray-400">Loading…</p>;

  const numOrNull = (s: string) => (s.trim() === "" ? null : Number(s));

  function submit() {
    save.mutate(
      {
        participationId,
        body: {
          priority,
          weight,
          rate_override: numOrNull(rateOverride),
          daily_cap: numOrNull(dailyCap),
          monthly_cap: numOrNull(monthlyCap),
          concurrency_cap: numOrNull(concurrencyCap),
        },
      },
      {
        onSuccess: () => toast.success("Routing saved"),
        onError: (e) => toast.error(errorMessage(e)),
      }
    );
  }

  const destination =
    data?.target_type === "dynamic"
      ? data.rtb_endpoint || "—"
      : data?.destination_number || "—";

  return (
    <div className="space-y-3 bg-gray-50/70 px-3 py-3">
      <div className="rounded border border-gray-100 bg-white px-3 py-2 text-xs text-gray-600">
        <span className="font-semibold">Buyer target:</span>{" "}
        {data?.configured ? (
          <span>
            {data.target_type === "dynamic" ? "RTB " : "Number "} {destination}
          </span>
        ) : (
          <span className="text-amber-700">Not configured by buyer yet</span>
        )}
      </div>

      <div className="grid grid-cols-3 gap-2">
        <div>
          <Label>Priority tier</Label>
          <Input type="number" value={priority} onChange={(e) => setPriority(Number(e.target.value))} />
        </div>
        <div>
          <Label>Weight (% in tier)</Label>
          <Input type="number" value={weight} onChange={(e) => setWeight(Number(e.target.value))} />
        </div>
        <div>
          <Label>Rate override ($)</Label>
          <Input
            type="number"
            step="0.01"
            value={rateOverride}
            onChange={(e) => setRateOverride(e.target.value)}
            placeholder="contract base"
          />
        </div>
        <div>
          <Label>Daily cap</Label>
          <Input type="number" value={dailyCap} onChange={(e) => setDailyCap(e.target.value)} placeholder="∞" />
        </div>
        <div>
          <Label>Monthly cap</Label>
          <Input type="number" value={monthlyCap} onChange={(e) => setMonthlyCap(e.target.value)} placeholder="∞" />
        </div>
        <div>
          <Label>Concurrency cap</Label>
          <Input
            type="number"
            value={concurrencyCap}
            onChange={(e) => setConcurrencyCap(e.target.value)}
            placeholder="∞"
          />
        </div>
      </div>

      <Button className="h-8 px-2 text-xs" variant="secondary" disabled={save.isPending} onClick={submit}>
        Save routing
      </Button>
    </div>
  );
}
