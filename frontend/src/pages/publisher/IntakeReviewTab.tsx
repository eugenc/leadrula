import { useState } from "react";
import { useIntakeQueue, useRejectQueue } from "@/features/admin/hooks";
import { Table, THead, TH, TBody, TR, TD } from "@/components/ui/table";
import { Button } from "@/components/ui/button";
import { Spinner, EmptyState } from "@/components/ui/misc";
import { format } from "date-fns";
import { toast } from "@/store/toastStore";
import { errorMessage } from "@/lib/api";
import type { QueueItem } from "@/types";
import { QueueItemDrawer, RouteDialog } from "./intakeShared";

export function IntakeReviewTab() {
  const { data, isLoading } = useIntakeQueue();
  const reject = useRejectQueue();
  const [routing, setRouting] = useState<QueueItem | null>(null);
  const [drawerItem, setDrawerItem] = useState<QueueItem | null>(null);

  const queue = data?.items ?? [];

  if (isLoading) return <Spinner className="h-6 w-6" />;

  if ((queue ?? []).length === 0) {
    return <EmptyState title="Nothing pending review." />;
  }

  return (
    <>
      <Table>
        <THead>
          <tr>
            <TH>Name</TH>
            <TH>Phone</TH>
            <TH>Source</TH>
            <TH>Received</TH>
            <TH />
          </tr>
        </THead>
        <TBody>
          {(queue ?? []).map((q) => (
            <TR key={q.id}>
              <TD className="font-medium text-gray-800">
                {q.first_name} {q.last_name}
              </TD>
              <TD>{q.phone ?? "—"}</TD>
              <TD>{q.source ?? "—"}</TD>
              <TD>{format(new Date(q.created_at), "MMM d, h:mma")}</TD>
              <TD>
                <div className="flex justify-end gap-2">
                  {(q.unmapped_keys?.length ?? 0) > 0 && (
                    <Button size="sm" variant="secondary" onClick={() => setDrawerItem(q)}>
                      Map
                    </Button>
                  )}
                  <Button size="sm" onClick={() => setRouting(q)}>
                    Route
                  </Button>
                  <Button
                    size="sm"
                    variant="secondary"
                    onClick={() => reject.mutate(q.id, { onError: (e) => toast.error(errorMessage(e)) })}
                  >
                    Reject
                  </Button>
                </div>
              </TD>
            </TR>
          ))}
        </TBody>
      </Table>

      {routing && <RouteDialog item={routing} onClose={() => setRouting(null)} />}
      {drawerItem && (
        <QueueItemDrawer
          item={drawerItem}
          onClose={() => setDrawerItem(null)}
          onUpdated={setDrawerItem}
          onRoute={() => {
            setRouting(drawerItem);
            setDrawerItem(null);
          }}
          onReject={() => {
            reject.mutate(drawerItem.id, {
              onSuccess: () => {
                toast.success("Lead rejected");
                setDrawerItem(null);
              },
              onError: (e) => toast.error(errorMessage(e)),
            });
          }}
        />
      )}
    </>
  );
}
