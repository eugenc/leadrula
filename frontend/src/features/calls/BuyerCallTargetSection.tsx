import { useEffect, useState } from "react";
import { Button } from "@/components/ui/button";
import { Input, Label, Select, Textarea } from "@/components/ui/input";
import { SectionLabel } from "@/components/layout/SectionLabel";
import { toast } from "@/store/toastStore";
import { errorMessage } from "@/lib/api";
import { useCallTarget, useSaveCallTarget } from "@/features/calls/hooks";

// Buyers configure where calls land: a static destination number or a dynamic
// RTB endpoint. A target must be saved before the participation can go active.
export function BuyerCallTargetSection({ participationId }: { participationId: number }) {
  const { data, isLoading } = useCallTarget(participationId, "buyer");
  const save = useSaveCallTarget("buyer");

  const [targetType, setTargetType] = useState<"static" | "dynamic">("static");
  const [destination, setDestination] = useState("");
  const [rtbEndpoint, setRtbEndpoint] = useState("");
  const [headersText, setHeadersText] = useState("");

  useEffect(() => {
    if (!data || !data.configured) return;
    setTargetType(data.target_type ?? "static");
    setDestination(data.destination_number ?? "");
    setRtbEndpoint(data.rtb_endpoint ?? "");
    setHeadersText(
      data.rtb_headers && Object.keys(data.rtb_headers).length
        ? JSON.stringify(data.rtb_headers, null, 2)
        : ""
    );
  }, [data]);

  if (isLoading) return <p className="text-sm text-gray-400">Loading…</p>;

  function submit() {
    const body: Record<string, unknown> = { target_type: targetType };
    if (targetType === "static") {
      if (!destination.trim()) {
        toast.error("Destination number is required");
        return;
      }
      body.destination_number = destination.trim();
    } else {
      if (!rtbEndpoint.trim()) {
        toast.error("RTB endpoint is required");
        return;
      }
      body.rtb_endpoint = rtbEndpoint.trim();
      if (headersText.trim()) {
        try {
          body.rtb_headers = JSON.parse(headersText);
        } catch {
          toast.error("RTB headers must be valid JSON");
          return;
        }
      }
    }
    save.mutate(
      { participationId, body },
      {
        onSuccess: () => toast.success("Call target saved"),
        onError: (e) => toast.error(errorMessage(e)),
      }
    );
  }

  return (
    <div className="space-y-3">
      <SectionLabel>Call target</SectionLabel>
      <p className="text-xs text-gray-400">
        Where should calls connect? Configure a target before activating — you receive no leads until then.
      </p>

      <div>
        <Label>Target type</Label>
        <Select value={targetType} onChange={(e) => setTargetType(e.target.value as "static" | "dynamic")}>
          <option value="static">Static — fixed destination number</option>
          <option value="dynamic">Dynamic — RTB endpoint</option>
        </Select>
      </div>

      {targetType === "static" ? (
        <div>
          <Label>Destination number (E.164)</Label>
          <Input value={destination} onChange={(e) => setDestination(e.target.value)} placeholder="+14155550123" />
        </div>
      ) : (
        <>
          <div>
            <Label>RTB endpoint URL</Label>
            <Input
              value={rtbEndpoint}
              onChange={(e) => setRtbEndpoint(e.target.value)}
              placeholder="https://rtb.example.com/ping"
            />
          </div>
          <div>
            <Label>RTB auth headers (JSON, optional)</Label>
            <Textarea
              value={headersText}
              onChange={(e) => setHeadersText(e.target.value)}
              placeholder={`{\n  "Authorization": "Bearer …"\n}`}
              rows={4}
            />
            <p className="mt-1 text-xs text-gray-500">Stored encrypted. Sent on every RTB ping.</p>
          </div>
        </>
      )}

      <Button variant="secondary" disabled={save.isPending} onClick={submit}>
        Save call target
      </Button>
    </div>
  );
}
