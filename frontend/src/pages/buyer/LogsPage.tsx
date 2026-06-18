import { useSearchParams } from "react-router-dom";
import { PageBody } from "@/components/layout/PageBody";
import { IntakeLogTable } from "@/features/intake/IntakeLogTable";
import type { LogTypeFilter } from "@/features/intake/logShared";

const LOG_TYPE_PARAM_VALUES: LogTypeFilter[] = ["all", "routes", "intake", "webhooks", "integrations"];

function parseLogTypeParam(value: string | null): LogTypeFilter | undefined {
  if (value && LOG_TYPE_PARAM_VALUES.includes(value as LogTypeFilter)) {
    return value as LogTypeFilter;
  }
  return undefined;
}

export function LogsPage() {
  const [searchParams] = useSearchParams();
  const initialLogType = parseLogTypeParam(searchParams.get("type")) ?? "all";
  return (
    <PageBody>
      <IntakeLogTable source="buyer" initialLogType={initialLogType} />
    </PageBody>
  );
}
