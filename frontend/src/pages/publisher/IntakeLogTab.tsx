import { useSearchParams } from "react-router-dom";
import { IntakeLogTable } from "@/features/intake/IntakeLogTable";

export function IntakeLogTab() {
  const [searchParams] = useSearchParams();
  const sourceSlug = searchParams.get("source") ?? undefined;
  return <IntakeLogTable source="publisher" sourceSlug={sourceSlug} />;
}
