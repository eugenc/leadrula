import { useState } from "react";
import { useIntakeQueue, useRouteQueue, useRejectQueue, useBuyers } from "@/features/admin/hooks";
import { PageBody } from "@/components/layout/PageBody";
import { Table, THead, TH, TBody, TR, TD } from "@/components/ui/table";
import { Button } from "@/components/ui/button";
import { Label, Select } from "@/components/ui/input";
import { Spinner, EmptyState } from "@/components/ui/misc";
import { Dialog } from "@/components/ui/dialog";
import { format } from "date-fns";
import { toast } from "@/store/toastStore";
import { apiError } from "@/lib/api";
import type { QueueItem } from "@/types";

export function IntakeQueuePage() {
  const { data: queue, isLoading } = useIntakeQueue();
  const reject = useRejectQueue();
  const [routing, setRouting] = useState<QueueItem | null>(null);

  return (
    <>
      <PageBody>
        {isLoading ? (
          <Spinner className="h-6 w-6" />
        ) : (queue ?? []).length === 0 ? (
          <EmptyState title="The intake queue is empty." />
        ) : (
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
                      <Button size="sm" onClick={() => setRouting(q)}>
                        Route
                      </Button>
                      <Button
                        size="sm"
                        variant="secondary"
                        onClick={() => reject.mutate(q.id, { onError: (e) => toast.error(apiError(e).message) })}
                      >
                        Reject
                      </Button>
                    </div>
                  </TD>
                </TR>
              ))}
            </TBody>
          </Table>
        )}

        {routing && <RouteDialog item={routing} onClose={() => setRouting(null)} />}
      </PageBody>
    </>
  );
}

function RouteDialog({ item, onClose }: { item: QueueItem; onClose: () => void }) {
  const { data: buyers } = useBuyers();
  const route = useRouteQueue();
  const [buyerId, setBuyerId] = useState(0);

  return (
    <Dialog open onClose={onClose} title={`Route ${item.first_name} ${item.last_name}`}>
      <div className="space-y-3">
        <div>
          <Label>Send to buyer</Label>
          <Select value={buyerId} onChange={(e) => setBuyerId(Number(e.target.value))}>
            <option value={0}>Select a buyer…</option>
            {(buyers ?? []).map((b) => (
              <option key={b.id} value={b.id}>
                {b.name}
              </option>
            ))}
          </Select>
          <p className="mt-1 text-xs text-gray-400">
            The lead lands in the buyer's contract pipeline and the buyer is charged the contract rate.
          </p>
        </div>
        <div className="flex justify-end gap-2 pt-2">
          <Button variant="secondary" onClick={onClose}>
            Cancel
          </Button>
          <Button
            disabled={!buyerId}
            onClick={() =>
              route.mutate(
                { id: item.id, body: { buyer_id: buyerId } },
                {
                  onSuccess: () => {
                    toast.success("Lead routed");
                    onClose();
                  },
                  onError: (e) => toast.error(apiError(e).message),
                }
              )
            }
          >
            Route Lead
          </Button>
        </div>
      </div>
    </Dialog>
  );
}
