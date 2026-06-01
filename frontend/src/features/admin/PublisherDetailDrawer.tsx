import { useState } from "react";
import { Sheet, DrawerHeader, DrawerBody, DrawerFooter } from "@/components/ui/dialog";
import { Button } from "@/components/ui/button";
import { Input, Label } from "@/components/ui/input";
import { Spinner } from "@/components/ui/misc";
import { toast } from "@/store/toastStore";
import { errorMessage } from "@/lib/api";
import { cn } from "@/lib/utils";
import { collabBadgeClass, collabLabel } from "@/features/collaboration/collaborationStatus";
import {
  usePublisher,
  usePublisherCollaboration,
  usePartnerships,
  useAcceptPartnership,
  useRejectPartnership,
  useInvitePublisherCollaborationForPublisher,
  useAcceptCollaborationForPublisher,
  useRejectCollaborationForPublisher,
  useRevokeCollaborationForPublisher,
} from "@/features/admin/hooks";

function partnershipLabel(status: string) {
  switch (status) {
    case "active":
      return "Active";
    case "pending_buyer":
      return "Pending your approval";
    case "pending_publisher":
      return "Pending publisher approval";
    case "rejected":
      return "Rejected";
    case "revoked":
      return "Revoked";
    default:
      return status;
  }
}

function partnershipBadgeClass(status: string) {
  switch (status) {
    case "active":
      return "bg-green-100 text-green-800";
    case "pending_buyer":
    case "pending_publisher":
      return "bg-amber-100 text-amber-800";
    default:
      return "bg-gray-100 text-gray-600";
  }
}

export function PublisherDetailDrawer({
  publisherId,
  leadCount,
  isAdmin,
  onClose,
}: {
  publisherId: number | null;
  leadCount: number;
  isAdmin: boolean;
  onClose: () => void;
}) {
  return (
    <Sheet open={!!publisherId} onClose={onClose} width={520}>
      {publisherId && (
        <DrawerContent
          publisherId={publisherId}
          leadCount={leadCount}
          isAdmin={isAdmin}
          onClose={onClose}
        />
      )}
    </Sheet>
  );
}

function DrawerContent({
  publisherId,
  leadCount,
  isAdmin,
  onClose,
}: {
  publisherId: number;
  leadCount: number;
  isAdmin: boolean;
  onClose: () => void;
}) {
  const { data: publisher, isLoading } = usePublisher(publisherId);
  const { data: collab } = usePublisherCollaboration(publisherId);
  const { data: partnerships } = usePartnerships();
  const acceptPartnership = useAcceptPartnership();
  const rejectPartnership = useRejectPartnership();
  const invite = useInvitePublisherCollaborationForPublisher();
  const acceptCollab = useAcceptCollaborationForPublisher();
  const rejectCollab = useRejectCollaborationForPublisher();
  const revoke = useRevokeCollaborationForPublisher();
  const [email, setEmail] = useState("");

  const partnership = (partnerships ?? []).find(
    (p) => publisher && p.partner_handler_id === publisher.handler_id
  );
  const partnershipStatus = partnership?.status ?? "active";
  const collabStatus = collab?.status ?? "none";

  if (isLoading || !publisher) {
    return (
      <div className="flex h-full items-center justify-center">
        <Spinner className="h-6 w-6" />
      </div>
    );
  }

  return (
    <div className="flex h-full flex-col">
      <DrawerHeader
        title={publisher.name}
        subtitle={`${publisher.handler_id} · ${leadCount} leads`}
        onClose={onClose}
      />

      <DrawerBody>
        <div className="mb-5 rounded-lg border border-gray-100 bg-gray-50 p-3">
          <div className="mb-2 flex items-center justify-between">
            <span className="text-sm font-semibold text-gray-800">Partnership</span>
            <span
              className={cn(
                "rounded-full px-2 py-0.5 text-xs font-medium",
                partnershipBadgeClass(partnershipStatus)
              )}
            >
              {partnershipLabel(partnershipStatus)}
            </span>
          </div>
          {partnershipStatus === "active" && (
            <p className="text-sm text-gray-500">
              You can receive leads from this publisher once a contract is in place.
            </p>
          )}
          {isAdmin && partnership && partnershipStatus === "pending_buyer" && (
            <div className="mt-2 flex gap-2">
              <Button
                size="sm"
                disabled={acceptPartnership.isPending}
                onClick={() =>
                  acceptPartnership.mutate(partnership.id, {
                    onSuccess: () => toast.success("Partnership accepted"),
                    onError: (e) => toast.error(errorMessage(e)),
                  })
                }
              >
                Accept
              </Button>
              <Button
                size="sm"
                variant="secondary"
                disabled={rejectPartnership.isPending}
                onClick={() =>
                  rejectPartnership.mutate(partnership.id, {
                    onSuccess: () => toast.success("Request rejected"),
                    onError: (e) => toast.error(errorMessage(e)),
                  })
                }
              >
                Reject
              </Button>
            </div>
          )}
        </div>

        <div className="mb-5 rounded-lg border border-gray-100 bg-gray-50 p-3">
          <div className="mb-2 flex items-center justify-between">
            <span className="text-sm font-semibold text-gray-800">Collaboration</span>
            <span
              className={cn(
                "rounded-full px-2 py-0.5 text-xs font-medium",
                collabBadgeClass(collabStatus)
              )}
            >
              {collabLabel(collabStatus)}
            </span>
          </div>

          {isAdmin && collabStatus === "active" && (
            <p className="text-sm text-gray-700">
              <strong>{collab?.publisher_name ?? publisher.name}</strong> can log in and manage this
              account as admin.
            </p>
          )}

          {isAdmin && collabStatus === "pending_buyer" && (
            <div className="flex gap-2">
              <Button
                size="sm"
                disabled={acceptCollab.isPending}
                onClick={() =>
                  acceptCollab.mutate(publisherId, {
                    onSuccess: () => toast.success("Collaboration enabled"),
                    onError: (e) => toast.error(errorMessage(e)),
                  })
                }
              >
                Accept request
              </Button>
              <Button
                size="sm"
                variant="secondary"
                disabled={rejectCollab.isPending}
                onClick={() =>
                  rejectCollab.mutate(publisherId, {
                    onSuccess: () => toast.success("Request rejected"),
                    onError: (e) => toast.error(errorMessage(e)),
                  })
                }
              >
                Reject
              </Button>
            </div>
          )}

          {isAdmin && (collabStatus === "none" || collabStatus === "revoked") && (
            <div className="space-y-2.5">
              <p className="text-sm text-gray-500">
                Invite a publisher admin by email. They must accept before access is granted.
              </p>
              <div>
                <Label>Publisher admin email</Label>
                <Input
                  type="email"
                  value={email}
                  onChange={(e) => setEmail(e.target.value)}
                  placeholder="admin@publisher.com"
                />
              </div>
              <Button
                size="sm"
                disabled={!email.trim() || invite.isPending}
                onClick={() =>
                  invite.mutate(
                    { publisherId, email: email.trim() },
                    {
                      onSuccess: () => {
                        toast.success("Invitation sent");
                        setEmail("");
                      },
                      onError: (e) => toast.error(errorMessage(e)),
                    }
                  )
                }
              >
                Send invitation
              </Button>
            </div>
          )}

          {collabStatus === "pending_publisher" && (
            <p className="text-sm text-gray-600">
              Waiting for{" "}
              <strong>{collab?.target_publisher_user_name ?? "publisher admin"}</strong> to accept your
              invitation.
            </p>
          )}
        </div>

        <div className="flex flex-col gap-2.5">
          <div>
            <div className="mb-2 text-sm font-semibold text-gray-800">Admin</div>
            <div className="space-y-2.5">
              <div>
                <Label>Name</Label>
                <div className="pt-1 text-sm font-medium text-gray-800">
                  {publisher.admin_name || "—"}
                </div>
              </div>
              <div>
                <Label>Email</Label>
                <div className="pt-1 text-sm font-medium text-gray-800">
                  {publisher.admin_email || "—"}
                </div>
              </div>
            </div>
          </div>
          <div>
            <Label>Website</Label>
            <div className="pt-1 text-sm font-medium text-gray-800">{publisher.website || "—"}</div>
          </div>
          <div>
            <Label>Timezone</Label>
            <div className="pt-1 text-sm font-medium text-gray-800">{publisher.timezone || "—"}</div>
          </div>
          <div>
            <Label>Leads</Label>
            <div className="pt-1 text-sm font-medium text-gray-800">{leadCount}</div>
          </div>
        </div>
      </DrawerBody>

      {isAdmin && collabStatus === "active" && (
        <DrawerFooter>
          <Button
            variant="danger"
            disabled={revoke.isPending}
            onClick={() =>
              revoke.mutate(publisherId, {
                onSuccess: () => toast.success("Collaboration revoked"),
                onError: (e) => toast.error(errorMessage(e)),
              })
            }
          >
            Revoke access
          </Button>
        </DrawerFooter>
      )}
    </div>
  );
}
