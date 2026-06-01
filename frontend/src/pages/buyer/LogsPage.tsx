import { PageBody } from "@/components/layout/PageBody";
import { IntakeLogTable } from "@/features/intake/IntakeLogTable";

export function LogsPage() {
  return (
    <PageBody>
      <IntakeLogTable source="buyer" readOnly />
    </PageBody>
  );
}
