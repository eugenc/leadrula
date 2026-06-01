import { useState } from "react";
import {
  usePublishers,
  usePartnerships,
  useRequestPartnership,
  useAcceptPartnership,
  useRejectPartnership,
} from "@/features/admin/hooks";
import { PublisherDetailDrawer } from "@/features/admin/PublisherDetailDrawer";
import { PageHeader } from "@/components/layout/PageHeader";
import { PageBody } from "@/components/layout/PageBody";
import { Table, THead, TH, TBody, TR, TD } from "@/components/ui/table";
import { Button } from "@/components/ui/button";
import { Input, Label } from "@/components/ui/input";
import { Spinner, EmptyState, Card } from "@/components/ui/misc";
import { FormDrawer } from "@/components/ui/dialog";
import { toast } from "@/store/toastStore";
import { errorMessage } from "@/lib/api";
import { useAuthStore } from "@/store/authStore";
import { Link2 } from "lucide-react";
import type { Partnership } from "@/types";

function PendingPartnerships({
  items,
  onAccept,
  onReject,
  accepting,
  rejecting,
}: {
  items: Partnership[];
  onAccept: (id: number) => void;
  onReject: (id: number) => void;
  accepting: boolean;
  rejecting: boolean;
}) {
  const pending = items.filter((p) => p.status === "pending_buyer");
  if (pending.length === 0) return null;

  return (
    <Card className="mb-4 p-4">
      <h2 className="mb-3 text-sm font-semibold text-gray-800">Pending partnership requests</h2>
      <div className="space-y-3">
        {pending.map((p) => (
          <div
            key={p.id}
            className="flex flex-wrap items-center justify-between gap-3 rounded-md border border-amber-100 bg-amber-50 px-3 py-2"
          >
            <div>
              <div className="text-sm font-medium text-gray-800">{p.partner_name}</div>
              <div className="text-xs text-gray-500">{p.partner_handler_id}</div>
            </div>
            <div className="flex gap-2">
              <Button size="sm" disabled={accepting} onClick={() => onAccept(p.id)}>
                Accept
              </Button>
              <Button size="sm" variant="secondary" disabled={rejecting} onClick={() => onReject(p.id)}>
                Reject
              </Button>
            </div>
          </div>
        ))}
      </div>
    </Card>
  );
}

export function PublishersPage() {
  const { data: publishers, isLoading } = usePublishers();
  const { data: partnerships } = usePartnerships();
  const request = useRequestPartnership();
  const accept = useAcceptPartnership();
  const reject = useRejectPartnership();
  const user = useAuthStore((s) => s.user);
  const isAdmin = user?.role === "admin";
  const [selectedPublisherId, setSelectedPublisherId] = useState<number | null>(null);
  const [selectedLeadCount, setSelectedLeadCount] = useState(0);
  const [linkOpen, setLinkOpen] = useState(false);
  const [linkHandlerId, setLinkHandlerId] = useState("");

  function openPublisher(id: number, leadCount: number) {
    setSelectedPublisherId(id);
    setSelectedLeadCount(leadCount);
  }

  function submitLink() {
    request.mutate(
      { publisher_handler_id: linkHandlerId.trim().toUpperCase() },
      {
        onSuccess: () => {
          toast.success("Partnership request sent");
          setLinkOpen(false);
          setLinkHandlerId("");
        },
        onError: (e) => toast.error(errorMessage(e)),
      }
    );
  }

  return (
    <>
      <PageHeader
        action={
          isAdmin ? (
            <Button variant="secondary" onClick={() => setLinkOpen(true)}>
              <Link2 className="h-4 w-4" /> Link Publisher
            </Button>
          ) : undefined
        }
      />
      <PageBody>
        {isAdmin && (
          <PendingPartnerships
            items={partnerships ?? []}
            onAccept={(id) =>
              accept.mutate(id, {
                onSuccess: () => toast.success("Partnership accepted"),
                onError: (e) => toast.error(errorMessage(e)),
              })
            }
            onReject={(id) =>
              reject.mutate(id, {
                onSuccess: () => toast.success("Request rejected"),
                onError: (e) => toast.error(errorMessage(e)),
              })
            }
            accepting={accept.isPending}
            rejecting={reject.isPending}
          />
        )}

        {isLoading ? (
          <Spinner className="h-6 w-6" />
        ) : (publishers ?? []).length === 0 ? (
          <EmptyState title="No publishers yet." />
        ) : (
          <Table>
            <THead>
              <tr>
                <TH>Publisher</TH>
                <TH>Handler ID</TH>
                <TH>Leads</TH>
                <TH>Status</TH>
              </tr>
            </THead>
            <TBody>
              {(publishers ?? []).map((p) => (
                <TR key={p.id} onClick={() => openPublisher(p.id, p.lead_count)}>
                  <TD className="font-medium text-gray-800">{p.name}</TD>
                  <TD className="font-mono text-xs text-gray-500">{p.handler_id}</TD>
                  <TD>{p.lead_count}</TD>
                  <TD>
                    <span className="rounded-full bg-green-100 px-2 py-0.5 text-xs font-medium text-green-800">
                      Active
                    </span>
                  </TD>
                </TR>
              ))}
            </TBody>
          </Table>
        )}

        <PublisherDetailDrawer
          publisherId={selectedPublisherId}
          leadCount={selectedLeadCount}
          isAdmin={!!isAdmin}
          onClose={() => setSelectedPublisherId(null)}
        />

        <FormDrawer
          open={linkOpen}
          onClose={() => setLinkOpen(false)}
          title="Link Publisher"
          width={420}
          footer={
            <>
              <Button variant="secondary" onClick={() => setLinkOpen(false)}>
                Cancel
              </Button>
              <Button disabled={!linkHandlerId.trim() || request.isPending} onClick={submitLink}>
                Send Request
              </Button>
            </>
          }
        >
          <p className="mb-3 text-sm text-gray-500">
            Enter an existing publisher&apos;s handler ID. They must accept before you can create a
            contract.
          </p>
          <Label>Publisher handler ID</Label>
          <Input
            placeholder="P-XXXXX"
            value={linkHandlerId}
            onChange={(e) => setLinkHandlerId(e.target.value.toUpperCase())}
          />
        </FormDrawer>
      </PageBody>
    </>
  );
}
