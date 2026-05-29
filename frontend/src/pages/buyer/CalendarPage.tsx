import { useState } from "react";
import { useCalendar } from "@/features/admin/hooks";
import { PageHeader } from "@/components/layout/PageHeader";
import { Card, Spinner, EmptyState, Badge } from "@/components/ui/misc";
import { useUIStore } from "@/store/uiStore";
import { format, isToday, isSameDay } from "date-fns";
import type { CalendarEvent } from "@/types";

export function CalendarPage() {
  const [scope, setScope] = useState<"me" | "global">("me");
  const { data: events, isLoading } = useCalendar(scope);
  const openDetail = useUIStore((s) => s.openDetail);

  const days = groupByDay(events ?? []);

  return (
    <div>
      <PageHeader
        title="Calendar"
        subtitle="Upcoming actions on your leads."
        action={
          <div className="flex rounded border border-pd-border">
            {(["me", "global"] as const).map((s) => (
              <button
                key={s}
                onClick={() => setScope(s)}
                className={`px-3 py-1.5 text-sm font-semibold capitalize ${
                  scope === s ? "bg-pd-green text-white" : "text-pd-text"
                }`}
              >
                {s === "me" ? "My actions" : "Team"}
              </button>
            ))}
          </div>
        }
      />
      {isLoading ? (
        <Spinner className="h-6 w-6" />
      ) : days.length === 0 ? (
        <EmptyState title="No scheduled actions." />
      ) : (
        <div className="space-y-4">
          {days.map(({ date, items }) => (
            <Card key={date.toISOString()} className="p-4">
              <div className="mb-2 flex items-center gap-2 text-sm font-bold">
                {format(date, "EEEE, MMM d")}
                {isToday(date) && <Badge variant="blue">Today</Badge>}
              </div>
              <div className="space-y-1">
                {items.map((e) => (
                  <button
                    key={e.lead_id}
                    onClick={() => openDetail(e.lead_id)}
                    className="flex w-full items-center justify-between rounded px-2 py-1.5 text-left text-sm hover:bg-pd-stage"
                  >
                    <span className="font-medium">{e.title}</span>
                    <span className={e.overdue ? "font-semibold text-pd-red" : "text-pd-muted"}>
                      {format(new Date(e.action_at), "h:mma")}
                    </span>
                  </button>
                ))}
              </div>
            </Card>
          ))}
        </div>
      )}
    </div>
  );
}

function groupByDay(events: CalendarEvent[]) {
  const sorted = [...events].sort(
    (a, b) => new Date(a.action_at).getTime() - new Date(b.action_at).getTime()
  );
  const groups: { date: Date; items: CalendarEvent[] }[] = [];
  for (const e of sorted) {
    const d = new Date(e.action_at);
    const last = groups[groups.length - 1];
    if (last && isSameDay(last.date, d)) last.items.push(e);
    else groups.push({ date: d, items: [e] });
  }
  return groups;
}
