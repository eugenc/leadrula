import { Copy } from "lucide-react";
import { IconButton } from "@/components/layout/IconButton";
import { toast } from "@/store/toastStore";
import { SunbaseFieldMapSection } from "@/features/integrations/SunbaseFieldMapSection";
import type { OutboundFieldMapEntry } from "@/types";

type Props = {
  ingestEndpoint: string;
  exampleCurl?: string;
  connectionPublicId: string;
  sourceSlug?: string;
  callSourceSlug?: string;
  fieldMap: OutboundFieldMapEntry[];
  onFieldMapChange: (map: OutboundFieldMapEntry[]) => void;
  onSave: () => void;
  saving?: boolean;
};

export function VoiceUniConnectionSettings({
  ingestEndpoint,
  exampleCurl,
  connectionPublicId,
  sourceSlug,
  callSourceSlug,
  fieldMap,
  onFieldMapChange,
  onSave,
  saving,
}: Props) {
  function copyText(text: string, label: string) {
    navigator.clipboard.writeText(text).then(
      () => toast.success(label),
      () => toast.error("Could not copy")
    );
  }

  return (
    <div className="space-y-6">
      <div className="space-y-3 rounded-lg border border-success-border bg-success-bg p-4">
        <div>
          <p className="text-sm font-semibold text-success-fg">VoiceUni ingest endpoint</p>
          <p className="mt-1 text-xs text-gray-600">
            VoiceUni pushes leads here with a publisher API key (<code className="text-xs">leads:write</code>).
            Include <code className="text-xs">connection_id</code> ({connectionPublicId}) when multiple connections exist.
          </p>
        </div>
        <div className="flex items-center gap-1.5 rounded-md border border-gray-200 bg-gray-0 px-3 py-2 font-mono text-xs text-gray-800">
          <span className="select-all break-all flex-1">{ingestEndpoint}</span>
          <IconButton aria-label="Copy ingest URL" onClick={() => copyText(ingestEndpoint, "Ingest URL copied")}>
            <Copy className="h-3.5 w-3.5" />
          </IconButton>
        </div>
        {exampleCurl && (
          <pre className="overflow-x-auto rounded-md border border-gray-200 bg-gray-50 p-3 text-xs text-gray-800">{exampleCurl}</pre>
        )}
      </div>

      {(sourceSlug || callSourceSlug) && (
        <div className="rounded-lg border border-gray-200 p-4 text-sm text-gray-700 space-y-1">
          {sourceSlug && (
            <p>
              Lead source slug: <code className="text-xs">{sourceSlug}</code>
            </p>
          )}
          {callSourceSlug && (
            <p>
              Call preload source (configure a call source with this slug for{" "}
              <code className="text-xs">POST /api/v1/calls/preload</code>):{" "}
              <code className="text-xs">{callSourceSlug}</code>
            </p>
          )}
        </div>
      )}

      <div>
        <p className="text-sm font-semibold text-gray-900">Inbound field map</p>
        <p className="mt-1 text-xs text-gray-600">Map VoiceUni JSON keys to LeadRula lead fields.</p>
      </div>
      <SunbaseFieldMapSection entries={fieldMap} onChange={onFieldMapChange} />

      <button
        type="button"
        className="inline-flex items-center rounded-md bg-primary px-3 py-2 text-sm font-medium text-white disabled:opacity-50"
        disabled={saving}
        onClick={onSave}
      >
        {saving ? "Saving…" : "Save settings"}
      </button>
    </div>
  );
}
