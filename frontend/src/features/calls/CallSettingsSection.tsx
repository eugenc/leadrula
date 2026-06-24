import { useEffect, useState } from "react";
import { Button } from "@/components/ui/button";
import { Input, Label, Select } from "@/components/ui/input";
import { Switch } from "@/components/ui/misc";
import { SectionLabel } from "@/components/layout/SectionLabel";
import { toast } from "@/store/toastStore";
import { errorMessage } from "@/lib/api";
import { useCallSettings, useSaveCallSettings } from "@/features/calls/hooks";
import type { CallSettings } from "@/types";

const GEO_MODES: { value: CallSettings["caller_geo_mode"]; label: string }[] = [
  { value: "none", label: "None" },
  { value: "area_code", label: "US area-code heuristic" },
  { value: "twilio_lookup", label: "Twilio Lookup (paid)" },
];

export function CallSettingsSection({ contractId }: { contractId: number }) {
  const { data, isLoading } = useCallSettings(contractId);
  const save = useSaveCallSettings();
  const [form, setForm] = useState<CallSettings | null>(null);

  useEffect(() => {
    if (data) setForm(data);
  }, [data]);

  if (isLoading || !form) {
    return <p className="text-sm text-gray-400">Loading call settings…</p>;
  }

  function set<K extends keyof CallSettings>(key: K, value: CallSettings[K]) {
    setForm((f) => (f ? { ...f, [key]: value } : f));
  }

  function submit() {
    if (!form) return;
    save.mutate(
      { contractId, body: { ...form, contract_id: contractId } },
      {
        onSuccess: () => toast.success("Call settings saved"),
        onError: (e) => toast.error(errorMessage(e)),
      }
    );
  }

  return (
    <div className="space-y-4 rounded-lg border border-gray-100 bg-gray-50/60 p-4">
      <SectionLabel>Call settings</SectionLabel>
      <p className="text-xs text-gray-400">
        Routing and billing rules for inbound calls on this contract.
      </p>

      <div className="grid grid-cols-2 gap-3">
        <div>
          <Label>Billable duration threshold (sec)</Label>
          <Input
            type="number"
            value={form.duration_threshold_sec}
            onChange={(e) => set("duration_threshold_sec", Number(e.target.value))}
          />
        </div>
        <div>
          <Label>Tier timeout (sec)</Label>
          <Input
            type="number"
            value={form.tier_timeout_sec}
            onChange={(e) => set("tier_timeout_sec", Number(e.target.value))}
          />
        </div>
        <div>
          <Label>Duplicate window (hours)</Label>
          <Input
            type="number"
            value={form.duplicate_window_hours}
            onChange={(e) => set("duplicate_window_hours", Number(e.target.value))}
          />
        </div>
        <div>
          <Label>Caller geo mode</Label>
          <Select
            value={form.caller_geo_mode}
            onChange={(e) => set("caller_geo_mode", e.target.value as CallSettings["caller_geo_mode"])}
          >
            {GEO_MODES.map((m) => (
              <option key={m.value} value={m.value}>
                {m.label}
              </option>
            ))}
          </Select>
        </div>
        <div>
          <Label>Vertical</Label>
          <Input value={form.vertical} onChange={(e) => set("vertical", e.target.value)} placeholder="e.g. Insurance" />
        </div>
        <div>
          <Label>Allowed states (comma-separated)</Label>
          <Input
            value={(form.allowed_states ?? []).join(", ")}
            onChange={(e) =>
              set(
                "allowed_states",
                e.target.value
                  .split(",")
                  .map((s) => s.trim().toUpperCase())
                  .filter(Boolean)
              )
            }
            placeholder="CA, TX, NY"
          />
        </div>
      </div>

      <div className="space-y-2">
        <ToggleRow
          label="Record calls"
          hint="Capture call recordings; buyers see them only after a billable connect."
          checked={form.recording_enabled}
          onChange={(v) => set("recording_enabled", v)}
        />
        <ToggleRow
          label="Expose caller ID to buyer"
          hint="Pass the real caller number to the buyer leg."
          checked={form.expose_caller_id}
          onChange={(v) => set("expose_caller_id", v)}
        />
        <ToggleRow
          label="Mask caller ID"
          hint="Dial buyers using the tracking number instead of the caller's number."
          checked={form.mask_caller_id}
          onChange={(v) => set("mask_caller_id", v)}
        />
        <ToggleRow
          label="Pass inbound payload to RTB"
          hint="Include preload payload fields in RTB pings."
          checked={form.pass_inbound_payload}
          onChange={(v) => set("pass_inbound_payload", v)}
        />
      </div>

      <Button variant="secondary" disabled={save.isPending} onClick={submit}>
        Save call settings
      </Button>
    </div>
  );
}

function ToggleRow({
  label,
  hint,
  checked,
  onChange,
}: {
  label: string;
  hint: string;
  checked: boolean;
  onChange: (v: boolean) => void;
}) {
  return (
    <div className="flex items-center justify-between">
      <div>
        <Label>{label}</Label>
        <p className="text-xs text-gray-500">{hint}</p>
      </div>
      <Switch checked={checked} onChange={onChange} />
    </div>
  );
}
