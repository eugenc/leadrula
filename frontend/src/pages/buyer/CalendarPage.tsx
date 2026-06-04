import { useState } from "react";
import { useCalendar } from "@/features/admin/hooks";
import { PageHeader } from "@/components/layout/PageHeader";
import { PageBody } from "@/components/layout/PageBody";
import { Card, Spinner, EmptyState, Badge } from "@/components/ui/misc";
import { cn } from "@/lib/utils";
import { useUIStore } from "@/store/uiStore";
import { format, isToday, isSameDay } from "date-fns";
import type { CalendarEvent } from "@/types";

export function CalendarPage() {
  const [scope, setScope] = useState<"me" | "global">("me");
  const { data: events, isLoading } = useCalendar(scope);
  const openDetail = useUIStore((s) => s.openDetail);

  const days = groupByDay(events ?? []);

  return (
    <>
      <PageHeader
        action={
          <div className="flex overflow-hidden rounded-md border border-gray-200">
            {(["me", "global"] as const).map((s) => (
              <button
                key={s}
                onClick={() => setScope(s)}
                className={cn(
                  "px-3 py-1.5 text-base font-semibold capitalize transition-colors",
                  scope === s ? "bg-jade-500 text-white" : "text-gray-700 hover:bg-gray-100"
                )}
              >
                {s === "me" ? "My actions" : "Team"}
              </button>
            ))}
          </div>
        }
      />
      <PageBody>
        {isLoading ? (
          <Spinner className="h-6 w-6" />
        ) : days.length === 0 ? (
          <EmptyState title="No scheduled actions." />
        ) : (
          <div className="space-y-4">
            {days.map(({ date, items }) => (
              <Card key={date.toISOString()} className="p-4">
                <div className="mb-2 flex items-center gap-2 text-sm font-bold text-gray-800">
                  {format(date, "EEEE, MMM d")}
                  {isToday(date) && <Badge variant="review">Today</Badge>}
                </div>
                <div className="space-y-1">
                  {items.map((e) => (
                    <button
                      key={e.lead_id}
                      onClick={() => openDetail(e.lead_id)}
                      className="flex w-full items-center justify-between rounded-md px-2 py-1.5 text-left text-sm hover:bg-jade-50"
                    >
                      <span className="font-medium text-gray-800">{e.title}</span>
                      <span className={e.overdue ? "font-semibold text-danger" : "text-gray-400"}>
                        {format(new Date(e.action_at), "h:mma")}
                      </span>
                    </button>
                  ))}
                </div>
              </Card>
            ))}
          </div>
        )}
      </PageBody>
    </>
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
