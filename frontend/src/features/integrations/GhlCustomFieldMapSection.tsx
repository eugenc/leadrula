import { GhlEntityFieldMapSection } from "@/features/integrations/GhlEntityFieldMapSection";
import type { GhlCustomField } from "@/features/integrations/hooks";
import type { OutboundFieldMapEntry } from "@/types";

export function GhlCustomFieldMapSection(props: {
  entries: OutboundFieldMapEntry[];
  onChange: (entries: OutboundFieldMapEntry[]) => void;
  ghlCustomFields: GhlCustomField[];
  ghlCustomFieldsLoading?: boolean;
  webhookMode?: boolean;
}) {
  return (
    <GhlEntityFieldMapSection
      section="contact"
      title="GHL custom field mapping"
      description="Map Leadrula fields to GHL custom fields."
      defaultModel="contact"
      {...props}
    />
  );
}
