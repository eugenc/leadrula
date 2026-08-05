import { Copy } from "lucide-react";
import { Button } from "@/components/ui/button";
import { IconButton } from "@/components/layout/IconButton";
import { toast } from "@/store/toastStore";
import type { SunbaseInboundWebhook } from "@/types";

const GHL_STAGE_SYNC_WEBHOOK_BODY = `{
  "contact_id": "{{contact.id}}",
  "pipeline_id": "{{opportunity.pipeline_id}}",
  "pipelineStageId": "{{opportunity.pipeline_stage_id}}"
}`;

export function GhlInboundEndpointSection({
  inbound,
  inboundStageSyncEnabled,
  syncPipelineName,
}: {
  inbound: SunbaseInboundWebhook;
  inboundStageSyncEnabled?: boolean;
  syncPipelineName?: string;
}) {
  function copyEndpoint() {
    navigator.clipboard.writeText(inbound.endpoint);
    toast.success("Endpoint copied");
  }

  function copyWebhookBody() {
    navigator.clipboard.writeText(GHL_STAGE_SYNC_WEBHOOK_BODY);
    toast.success("Webhook body copied");
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
          <div className="mt-2 space-y-1 rounded-md border border-amber-200 bg-amber-50 p-2 text-xs text-amber-900">
            <p className="font-medium">Stage sync requires these fields in the GHL webhook body:</p>
            <ul className="list-disc pl-4">
              <li><code className="font-mono">contact_id</code> or <code className="font-mono">contactId</code></li>
              <li><code className="font-mono">pipelineId</code></li>
              <li><code className="font-mono">pipelineStageId</code></li>
            </ul>
            <p>Use a GHL Workflow triggered on <strong>Opportunity Stage Changed</strong> and map those values into the webhook action.</p>
            <p className="mt-1">Do not use display names like <code className="font-mono">pipeline_name</code> or <code className="font-mono">pipleline_stage</code> — send GHL IDs instead.</p>
            <div className="mt-2 space-y-1">
              <p className="font-medium">Example webhook body (copy into your GHL workflow):</p>
              <pre className="overflow-x-auto rounded border border-amber-300 bg-white p-2 font-mono text-[11px]">{GHL_STAGE_SYNC_WEBHOOK_BODY}</pre>
              <Button size="sm" variant="secondary" onClick={copyWebhookBody}>
                <Copy className="h-3.5 w-3.5" /> Copy webhook body
              </Button>
            </div>
          </div>
        )}
        {inboundStageSyncEnabled && syncPipelineName && (
          <p className="mt-2 text-xs font-medium text-success-fg">
            Pipeline stage auto-sync is on for {syncPipelineName}.
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
