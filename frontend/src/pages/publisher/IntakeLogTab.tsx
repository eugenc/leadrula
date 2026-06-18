import { useSearchParams } from "react-router-dom";
import { IntakeLogTable } from "@/features/intake/IntakeLogTable";
import type { LogTypeFilter } from "@/features/intake/logShared";

const LOG_TYPE_PARAM_VALUES: LogTypeFilter[] = ["all", "routes", "intake", "webhooks", "integrations"];

function parseLogTypeParam(value: string | null): LogTypeFilter | undefined {
  if (value && LOG_TYPE_PARAM_VALUES.includes(value as LogTypeFilter)) {
    return value as LogTypeFilter;
  }
  return undefined;
}

export function IntakeLogTab() {
  const [searchParams] = useSearchParams();
  const sourceSlug = searchParams.get("source") ?? undefined;
  const initialLogType = parseLogTypeParam(searchParams.get("type")) ?? "all";
  return (
    <IntakeLogTable
      source="publisher"
      sourceSlug={sourceSlug}
      initialLogType={initialLogType}
    />
  );
}
