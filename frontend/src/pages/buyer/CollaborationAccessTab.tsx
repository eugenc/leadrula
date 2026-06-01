import { useState } from "react";
import { Link } from "react-router-dom";
import { usePublishers } from "@/features/admin/hooks";
import { PublisherDetailDrawer } from "@/features/admin/PublisherDetailDrawer";
import { collabBadgeClass, collabLabel } from "@/features/collaboration/collaborationStatus";
import { Table, THead, TH, TBody, TR, TD } from "@/components/ui/table";
import { Spinner, EmptyState } from "@/components/ui/misc";
import { useAuthStore } from "@/store/authStore";
import { cn } from "@/lib/utils";

export function CollaborationAccessTab() {
  const { data: publishers, isLoading } = usePublishers();
  const user = useAuthStore((s) => s.user);
  const isAdmin = user?.role === "admin";
  const [selectedPublisherId, setSelectedPublisherId] = useState<number | null>(null);
  const [selectedLeadCount, setSelectedLeadCount] = useState(0);

  if (isLoading) return <Spinner className="h-6 w-6" />;

  if ((publishers ?? []).length === 0) {
    return (
      <EmptyState
        title="No publishers linked yet."
        subtitle="Link a publisher from the Publishers page before managing collaboration access."
        action={
          <Link to="/b/publishers" className="text-sm font-medium text-jade-600 hover:underline">
            Go to Publishers
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
            <TH>Publisher</TH>
            <TH>Handler ID</TH>
            <TH>Access</TH>
          </tr>
        </THead>
        <TBody>
          {(publishers ?? []).map((p) => (
            <TR
              key={p.id}
              onClick={() => {
                setSelectedPublisherId(p.id);
                setSelectedLeadCount(p.lead_count);
              }}
              className="cursor-pointer"
            >
              <TD className="font-medium text-gray-800">{p.name}</TD>
              <TD className="font-mono text-xs text-gray-500">{p.handler_id}</TD>
              <TD>
                <span
                  className={cn(
                    "rounded-full px-2 py-0.5 text-xs font-medium",
                    collabBadgeClass(p.collaboration_status ?? "none")
                  )}
                >
                  {collabLabel(p.collaboration_status ?? "none")}
                </span>
              </TD>
            </TR>
          ))}
        </TBody>
      </Table>

      <PublisherDetailDrawer
        publisherId={selectedPublisherId}
        leadCount={selectedLeadCount}
        isAdmin={!!isAdmin}
        onClose={() => setSelectedPublisherId(null)}
      />
    </>
  );
}
