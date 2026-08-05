import { Copy } from "lucide-react";
import { Button } from "@/components/ui/button";
import { IconButton } from "@/components/layout/IconButton";
import { toast } from "@/store/toastStore";
import type { SunbaseInboundWebhook } from "@/types";

export function GhlInboundEndpointSection({
  inbound,
  inboundStageSyncEnabled,
}: {
  inbound: SunbaseInboundWebhook;
  inboundStageSyncEnabled?: boolean;
}) {
  function copyEndpoint() {
    navigator.clipboard.writeText(inbound.endpoint);
    toast.success("Endpoint copied");
  }

  return (
    <div className="space-y-3 rounded-lg border border-success-border bg-success-bg p-4">
      <div>
        <p className="text-sm font-semibold text-success-fg">Integration connected</p>
        <p className="mt-1 text-sm font-medium text-gray-900">Receive updates from GoHighLevel</p>
        <p className="mt-1 text-xs text-gray-600">
          {inbound.setup_hint ||
            "In GHL, add this URL to a Workflow or Custom Webhook trigger for Contact, Opportunity, or Appointment events."}
        </p>
        {inboundStageSyncEnabled && (
          <p className="mt-2 text-xs text-gray-700">
            Stage sync reads <code className="font-mono">pipleline_stage</code>,{" "}
            <code className="font-mono">pipeline_id</code>, and <code className="font-mono">contactId</code> from
            GHL&apos;s default webhook payload. No custom JSON needed.
          </p>
        )}
      </div>
      <ol className="list-decimal space-y-1 pl-4 text-xs text-gray-700">
        <li>Copy the endpoint URL below</li>
        <li>In GoHighLevel, create a Workflow or automation webhook action</li>
        <li>Paste this URL as the webhook destination</li>
      </ol>
      <div className="flex items-center gap-1.5 rounded-md border border-gray-200 bg-gray-0 px-3 py-2 font-mono text-xs text-gray-800">
        <span className="select-all break-all flex-1">{inbound.endpoint}</span>
        <IconButton aria-label="Copy endpoint" onClick={copyEndpoint}>
          <Copy className="h-3.5 w-3.5" />
        </IconButton>
      </div>
      <Button size="sm" variant="secondary" onClick={copyEndpoint}>
        <Copy className="h-3.5 w-3.5" /> Copy endpoint
      </Button>
    </div>
  );
}
