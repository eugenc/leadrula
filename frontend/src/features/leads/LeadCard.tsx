import type { CustomField, Lead } from "@/types";
import { Avatar, Badge } from "@/components/ui/misc";
import { Phone, Clock } from "lucide-react";
import { cn } from "@/lib/utils";
import { format, isPast } from "date-fns";

function renderValue(v: unknown): string {
  if (v == null) return "";
  if (typeof v === "string") return v;
  return JSON.stringify(v);
}

export function LeadCard({
  lead,
  customFields,
  onClick,
  dragging,
}: {
  lead: Lead;
  customFields: CustomField[];
  onClick: () => void;
  dragging?: boolean;
}) {
  // show the first 1-2 active custom fields by position
  const shown = [...customFields]
    .filter((f) => f.is_active)
    .sort((a, b) => a.position - b.position)
    .slice(0, 2)
    .map((f) => ({ field: f, value: renderValue(lead.custom_values?.[String(f.id)]) }))
    .filter((x) => x.value);

  const overdue = lead.action_at && isPast(new Date(lead.action_at));

  return (
    <div
      onClick={onClick}
      className={cn(
        "cursor-pointer rounded border border-pd-border bg-white p-3 shadow-sm transition-shadow hover:shadow-md",
        dragging && "rotate-1 shadow-lg"
      )}
    >
      <div className="mb-1 flex items-start justify-between gap-2">
        <span className="font-semibold text-pd-text">
          {lead.first_name} {lead.last_name}
        </span>
        {lead.status === "returned" && <Badge variant="amber">Returned</Badge>}
      </div>
      {lead.phone && (
        <div className="mb-1 flex items-center gap-1.5 text-xs text-pd-muted">
          <Phone className="h-3 w-3" /> {lead.phone}
        </div>
      )}
      {shown.map((x) => (
        <div key={x.field.id} className="text-xs text-pd-muted">
          <span className="font-medium">{x.field.name}:</span> {x.value}
        </div>
      ))}
      <div className="mt-2 flex items-center justify-between">
        {lead.action_at ? (
          <span
            className={cn(
              "flex items-center gap-1 text-xs",
              overdue ? "font-semibold text-pd-red" : "text-pd-muted"
            )}
          >
            <Clock className="h-3 w-3" />
            {format(new Date(lead.action_at), "MMM d, h:mma")}
          </span>
        ) : (
          <span />
        )}
        {lead.assigned_user_id && <Avatar name={`U${lead.assigned_user_id}`} className="h-6 w-6 text-[10px]" />}
      </div>
    </div>
  );
}
