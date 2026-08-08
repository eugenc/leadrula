import { useSearchParams } from "react-router-dom";
import { IntakeLogTable } from "@/features/intake/IntakeLogTable";
import { parseLogExpandParam, parseLogIntParam, parseLogTypeParam } from "@/features/intake/logShared";

export function IntakeLogTab() {
  const [searchParams] = useSearchParams();
  const sourceSlug = searchParams.get("source") ?? undefined;
  const initialLogType = parseLogTypeParam(searchParams.get("type")) ?? "all";
  const initialLeadId = parseLogIntParam(searchParams.get("lead_id"));
  const initialWebhookId = parseLogIntParam(searchParams.get("webhook_id"));
  const initialExpandedKey = parseLogExpandParam(searchParams.get("expand"));
  return (
    <IntakeLogTable
      source="publisher"
      sourceSlug={sourceSlug}
      initialLogType={initialLogType}
      initialLeadId={initialLeadId}
      initialWebhookId={initialWebhookId}
      initialExpandedKey={initialExpandedKey}
    />
  );
}
