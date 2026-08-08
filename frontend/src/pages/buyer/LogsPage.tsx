import { useSearchParams } from "react-router-dom";
import { PageBody } from "@/components/layout/PageBody";
import { IntakeLogTable } from "@/features/intake/IntakeLogTable";
import { parseLogExpandParam, parseLogIntParam, parseLogTypeParam } from "@/features/intake/logShared";

export function LogsPage() {
  const [searchParams] = useSearchParams();
  const initialLogType = parseLogTypeParam(searchParams.get("type")) ?? "all";
  const initialLeadId = parseLogIntParam(searchParams.get("lead_id"));
  const initialWebhookId = parseLogIntParam(searchParams.get("webhook_id"));
  const initialExpandedKey = parseLogExpandParam(searchParams.get("expand"));
  return (
    <PageBody>
      <IntakeLogTable
        source="buyer"
        initialLogType={initialLogType}
        initialLeadId={initialLeadId}
        initialWebhookId={initialWebhookId}
        initialExpandedKey={initialExpandedKey}
      />
    </PageBody>
  );
}
