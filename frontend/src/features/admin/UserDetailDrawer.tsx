import { useEffect, useState } from "react";
import { Sheet, DrawerHeader, DrawerBody, DrawerFooter } from "@/components/ui/dialog";
import { Button } from "@/components/ui/button";
import { Input, Label, Select } from "@/components/ui/input";
import { Badge } from "@/components/ui/misc";
import { KeyRound, Mail } from "lucide-react";
import { toast } from "@/store/toastStore";
import { apiError } from "@/lib/api";
import { formatRole } from "@/lib/utils";
import {
  useUpdateUser,
  useUpdateInvite,
  useDeleteUser,
  useDeleteInvite,
  useResendInvite,
  useRequestPasswordReset,
  useUploadUserAvatar,
} from "@/features/admin/hooks";
import { AvatarUpload, uploadError } from "@/features/admin/AvatarUpload";
import type { Role, UserRow } from "@/types";

const ROLES: { value: Role; label: string }[] = [
  { value: "admin", label: "Admin" },
  { value: "user", label: "User" },
  { value: "follower", label: "Follower" },
];

function statusBadge(user: UserRow) {
  if (user.status === "pending") {
    return <Badge variant="pending">Pending</Badge>;
  }
  if (user.status === "active") {
    return <Badge variant="distributed">Active</Badge>;
  }
  return <Badge variant="closed">Inactive</Badge>;
}

function statusSubtitle(user: UserRow) {
  if (user.status === "pending") return "Pending invite";
  if (user.status === "active") return "Active member";
  return "Inactive member";
}

export function UserDetailDrawer({
  user,
  onClose,
}: {
  user: UserRow | null;
  onClose: () => void;
}) {
  return (
    <Sheet open={!!user} onClose={onClose}>
      {user && <DrawerContent user={user} onClose={onClose} />}
    </Sheet>
  );
}

function DrawerContent({ user, onClose }: { user: UserRow; onClose: () => void }) {
  const update = useUpdateUser();
  const updateInvite = useUpdateInvite();
  const remove = useDeleteUser();
  const removeInvite = useDeleteInvite();
  const resend = useResendInvite();
  const resetPassword = useRequestPasswordReset();
  const uploadAvatar = useUploadUserAvatar();

  const [fullName, setFullName] = useState(user.full_name);
  const [email, setEmail] = useState(user.email);
  const [role, setRole] = useState(user.role);

  useEffect(() => {
    setFullName(user.full_name);
    setEmail(user.email);
    setRole(user.role);
  }, [user]);

  const trimmedName = fullName.trim();
  const trimmedEmail = email.trim();
  const unchanged =
    trimmedName === user.full_name && trimmedEmail === user.email && role === user.role;
  const invalid = !trimmedName || !trimmedEmail;
  const saving = update.isPending || updateInvite.isPending;

  function save() {
    const body: Record<string, string> = {};
    if (trimmedName !== user.full_name) body.full_name = trimmedName;
    if (trimmedEmail !== user.email) body.email = trimmedEmail;
    if (role !== user.role) body.role = role;
    if (Object.keys(body).length === 0) return;

    const onSuccess = () => {
      toast.success("Saved");
      onClose();
    };
    const onError = (e: unknown) => toast.error(apiError(e).message);

    if (user.status === "pending") {
      updateInvite.mutate({ id: user.invite_id, body }, { onSuccess, onError });
    } else {
      update.mutate({ id: user.id, body }, { onSuccess, onError });
    }
  }

  function revoke() {
    const onSuccess = () => {
      toast.success(user.status === "pending" ? "Invite revoked" : "User removed");
      onClose();
    };
    const onError = (e: unknown) => toast.error(apiError(e).message);

    if (user.status === "pending") {
      removeInvite.mutate(user.invite_id, { onSuccess, onError });
    } else {
      remove.mutate(user.id, { onSuccess, onError });
    }
  }

  return (
    <div className="flex h-full flex-col">
      <DrawerHeader
        title={user.full_name || user.email}
        subtitle={`${statusSubtitle(user)} · ${formatRole(user.role)}`}
        onClose={onClose}
      />

      <DrawerBody>
        <div className="flex flex-col gap-2.5">
          {user.status !== "pending" && (
            <AvatarUpload
              name={user.full_name || user.email}
              src={user.avatar_url}
              uploading={uploadAvatar.isPending}
              onSelect={(file) =>
                uploadAvatar.mutate(
                  { id: user.id, file },
                  {
                    onSuccess: () => toast.success("Photo updated"),
                    onError: uploadError,
                  }
                )
              }
            />
          )}
          <div>
            <Label>Full Name</Label>
            <Input value={fullName} onChange={(e) => setFullName(e.target.value)} />
          </div>
          <div>
            <Label>Email</Label>
            <Input type="email" value={email} onChange={(e) => setEmail(e.target.value)} />
          </div>
          <div>
            <Label>Role</Label>
            <Select value={role} onChange={(e) => setRole(e.target.value as Role)}>
              {ROLES.map((r) => (
                <option key={r.value} value={r.value}>
                  {r.label}
                </option>
              ))}
            </Select>
          </div>
          <div>
            <Label>Status</Label>
            <div className="pt-1">{statusBadge(user)}</div>
          </div>
        </div>
      </DrawerBody>

      <DrawerFooter className="flex flex-col gap-2">
        <Button disabled={unchanged || invalid || saving} onClick={save}>
          Save
        </Button>
        {user.status === "pending" && (
          <Button
            variant="secondary"
            disabled={resend.isPending}
            onClick={() =>
              resend.mutate(user.invite_id, {
                onSuccess: () => toast.success("Invite resent"),
                onError: (e) => toast.error(apiError(e).message),
              })
            }
          >
            <Mail className="mr-2 h-4 w-4" />
            Resend invite
          </Button>
        )}
        {user.status !== "pending" && (
          <Button
            variant="secondary"
            disabled={resetPassword.isPending}
            onClick={() =>
              resetPassword.mutate(user.email, {
                onSuccess: () => toast.success("Password reset email sent"),
                onError: (e) => toast.error(apiError(e).message),
              })
            }
          >
            <KeyRound className="mr-2 h-4 w-4" />
            Send password reset
          </Button>
        )}
        <Button
          variant="danger"
          disabled={remove.isPending || removeInvite.isPending}
          onClick={revoke}
        >
          {user.status === "pending" ? "Revoke invite" : "Remove user"}
        </Button>
      </DrawerFooter>
    </div>
  );
}
