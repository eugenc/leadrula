import { useMemo, useState } from "react";
import { Link } from "react-router-dom";
import { useBuyers, useCollabSummaries } from "@/features/admin/hooks";
import { BuyerDetailDrawer } from "@/features/admin/BuyerDetailDrawer";
import { collabBadgeClass } from "@/features/collaboration/collaborationStatus";
import { Table, THead, TH, TBody, TR, TD } from "@/components/ui/table";
import { Spinner, EmptyState } from "@/components/ui/misc";
import { cn } from "@/lib/utils";

function collabLabelPublisher(status: string) {
  switch (status) {
    case "active":
      return "Active";
    case "pending_buyer":
      return "Awaiting buyer approval";
    case "pending_publisher":
      return "Pending your approval";
    case "revoked":
      return "Revoked";
    default:
      return "None";
  }
}

export function PublisherCollaborationAccessTab() {
  const { data: buyers, isLoading: buyersLoading } = useBuyers();
  const { data: summaries, isLoading: summariesLoading } = useCollabSummaries();
  const [selectedBuyerId, setSelectedBuyerId] = useState<number | null>(null);
  const [selectedLeadCount, setSelectedLeadCount] = useState(0);

  const statusByBuyerId = useMemo(() => {
    const map: Record<number, string> = {};
    for (const s of summaries ?? []) {
      map[s.buyer_id] = s.status;
    }
    return map;
  }, [summaries]);

  const isLoading = buyersLoading || summariesLoading;

  if (isLoading) return <Spinner className="h-6 w-6" />;

  if ((buyers ?? []).length === 0) {
    return (
      <EmptyState
        title="No buyers linked yet."
        subtitle="Link a buyer from the Buyers page before managing collaboration access."
        action={
          <Link to="/p/buyers" className="text-sm font-medium text-jade-600 hover:underline">
            Go to Buyers
          </Link>
        }
      />
    );
  }

  return (
    <>
      <Table>
        <THead>
          <tr>
            <TH>Buyer</TH>
            <TH>Handler ID</TH>
            <TH>Access</TH>
          </tr>
        </THead>
        <TBody>
          {(buyers ?? []).map((b) => {
            const status = statusByBuyerId[b.id] ?? "none";
            return (
              <TR
                key={b.id}
                onClick={() => {
                  setSelectedBuyerId(b.id);
                  setSelectedLeadCount(b.lead_count);
                }}
                className="cursor-pointer"
              >
                <TD className="font-medium text-gray-800">{b.name}</TD>
                <TD className="font-mono text-xs text-gray-500">{b.handler_id}</TD>
                <TD>
                  <span
                    className={cn(
                      "rounded-full px-2 py-0.5 text-xs font-medium",
                      collabBadgeClass(status)
                    )}
                  >
                    {collabLabelPublisher(status)}
                  </span>
                </TD>
              </TR>
            );
          })}
        </TBody>
      </Table>

      <BuyerDetailDrawer
        buyerId={selectedBuyerId}
        leadCount={selectedLeadCount}
        onClose={() => setSelectedBuyerId(null)}
      />
    </>
  );
}
