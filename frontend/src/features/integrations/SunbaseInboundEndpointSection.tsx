import { Copy } from "lucide-react";
import { Button } from "@/components/ui/button";
import { IconButton } from "@/components/layout/IconButton";
import { toast } from "@/store/toastStore";
import type { SunbaseInboundWebhook } from "@/types";

export function SunbaseInboundEndpointSection({
  inbound,
}: {
  inbound: SunbaseInboundWebhook;
}) {
  function copyEndpoint() {
    navigator.clipboard.writeText(inbound.endpoint);
    toast.success("Endpoint copied");
  }

  return (
    <div className="space-y-3 rounded-lg border border-success-border bg-success-bg p-4">
      <div>
        <p className="text-sm font-semibold text-success-fg">Integration connected</p>
        <p className="mt-1 text-sm font-medium text-gray-900">Send lead updates from SunBase</p>
        <p className="mt-1 text-xs text-gray-600">{inbound.setup_hint}</p>
      </div>
      <ol className="list-decimal space-y-1 pl-4 text-xs text-gray-700">
        <li>Copy the endpoint URL below</li>
        <li>In SunBase or Zapier, create a trigger that POSTs JSON when leads are created or updated</li>
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
