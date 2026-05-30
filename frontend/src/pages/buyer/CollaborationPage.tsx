import { useState } from "react";
import { Link } from "react-router-dom";
import { PageBody } from "@/components/layout/PageBody";
import { PageHeader } from "@/components/layout/PageHeader";
import { Card, Spinner } from "@/components/ui/misc";
import { Button } from "@/components/ui/button";
import { Input, Label } from "@/components/ui/input";
import {
  useAcceptCollaborationBuyer,
  useBuyerCollabStatus,
  useInvitePublisherCollaboration,
  useRejectCollaborationBuyer,
  useRevokeCollaboration,
} from "@/features/admin/hooks";
import { toast } from "@/store/toastStore";
import { apiError } from "@/lib/api";
import { cn } from "@/lib/utils";

function statusLabel(status: string) {
  switch (status) {
    case "active":
      return "Active";
    case "pending_buyer":
      return "Pending your approval";
    case "pending_publisher":
      return "Pending publisher approval";
    case "revoked":
      return "Revoked";
    default:
      return "None";
  }
}

export function CollaborationPage() {
  const { data: collab, isLoading } = useBuyerCollabStatus();
  const invite = useInvitePublisherCollaboration();
  const accept = useAcceptCollaborationBuyer();
  const reject = useRejectCollaborationBuyer();
  const revoke = useRevokeCollaboration();
  const [email, setEmail] = useState("");

  if (isLoading) {
    return (
      <PageBody>
        <Spinner className="h-6 w-6" />
      </PageBody>
    );
  }

  const status = collab?.status ?? "none";

  return (
    <>
      <PageHeader title="Collaboration" />
      <PageBody className="max-w-2xl space-y-4">
        <Card className="p-5">
          <div className="mb-4 flex items-center justify-between">
            <div>
              <h2 className="text-sm font-semibold text-gray-800">Publisher access</h2>
              <p className="text-sm text-gray-500">
                Allow your publisher to log in and manage this account as an admin.
              </p>
            </div>
            <span
              className={cn(
                "rounded-full px-2.5 py-0.5 text-xs font-medium",
                status === "active" && "bg-green-100 text-green-800",
                status.startsWith("pending") && "bg-amber-100 text-amber-800",
                status === "revoked" && "bg-gray-100 text-gray-600",
                status === "none" && "bg-gray-100 text-gray-600"
              )}
            >
              {statusLabel(status)}
            </span>
          </div>

          {status === "active" && (
            <div className="space-y-3">
              <p className="text-sm text-gray-700">
                <strong>{collab?.publisher_name}</strong> can impersonate this account as buyer admin.
              </p>
              <Button
                variant="danger"
                disabled={revoke.isPending}
                onClick={() =>
                  revoke.mutate(undefined, {
                    onSuccess: () => toast.success("Collaboration revoked"),
                    onError: (e) => toast.error(apiError(e).message),
                  })
                }
              >
                Revoke access
              </Button>
            </div>
          )}

          {status === "pending_buyer" && (
            <div className="flex gap-2">
              <Button
                disabled={accept.isPending}
                onClick={() =>
                  accept.mutate(undefined, {
                    onSuccess: () => toast.success("Collaboration enabled"),
                    onError: (e) => toast.error(apiError(e).message),
                  })
                }
              >
                Accept request
              </Button>
              <Button
                variant="secondary"
                disabled={reject.isPending}
                onClick={() =>
                  reject.mutate(undefined, {
                    onSuccess: () => toast.success("Request rejected"),
                    onError: (e) => toast.error(apiError(e).message),
                  })
                }
              >
                Reject
              </Button>
            </div>
          )}

          {(status === "none" || status === "revoked") && (
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
                disabled={!email.trim() || invite.isPending}
                onClick={() =>
                  invite.mutate(email.trim(), {
                    onSuccess: () => {
                      toast.success("Invitation sent");
                      setEmail("");
                    },
                    onError: (e) => toast.error(apiError(e).message),
                  })
                }
              >
                Send invitation
              </Button>
            </div>
          )}

          {status === "pending_publisher" && (
            <p className="text-sm text-gray-600">
              Waiting for <strong>{collab?.target_publisher_user_name ?? "publisher admin"}</strong> to accept
              your invitation.
            </p>
          )}
        </Card>

        {(collab?.audit_log?.length ?? 0) > 0 && (
          <Card className="p-5">
            <h2 className="mb-3 text-sm font-semibold text-gray-800">Recent activity</h2>
            <ul className="space-y-2">
              {collab!.audit_log!.map((e) => (
                <li key={e.id} className="border-b border-gray-100 pb-2 text-sm last:border-0">
                  <span className="font-medium text-gray-700">{e.event_type.replace(/_/g, " ")}</span>
                  {e.actor_name ? <span className="text-gray-500"> — {e.actor_name}</span> : null}
                  <span className="block text-xs text-gray-400">
                    {new Date(e.created_at).toLocaleString()}
                  </span>
                </li>
              ))}
            </ul>
          </Card>
        )}

        <p className="text-sm text-gray-400">
          <Link to="/b/settings" className="text-jade-600 hover:underline">
            Back to profile settings
          </Link>
        </p>
      </PageBody>
    </>
  );
}
