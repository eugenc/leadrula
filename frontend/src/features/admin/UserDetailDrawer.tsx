import { useEffect, useMemo, useState } from "react";
import { Sheet, DrawerHeader, DrawerBody, DrawerFooter } from "@/components/ui/dialog";
import { Button } from "@/components/ui/button";
import { Input, Label, Select } from "@/components/ui/input";
import { Badge } from "@/components/ui/misc";
import { KeyRound, Mail } from "lucide-react";
import { toast } from "@/store/toastStore";
import { errorMessage } from "@/lib/api";
import { cn, formatRole } from "@/lib/utils";
import { deltaFromEffective, isValidLeadVisibility, presetForRole } from "@/lib/permissions";
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
import { PermissionsEditor, effectiveFromUserRow } from "@/features/admin/PermissionsEditor";
import { useAuthStore } from "@/store/authStore";
import type { Role, UserRow } from "@/types";

const ROLES: { value: Role; label: string }[] = [
  { value: "admin", label: "Admin" },
  { value: "user", label: "User" },
  { value: "follower", label: "Follower" },
];

type DrawerTab = "profile" | "permissions";

function statusBadge(user: UserRow) {
  if (user.status === "pending") {
    return <Badge variant="pending">Pending</Badge>;
  }
  if (user.status === "expired") {
    return <Badge variant="overdue">Invite expired</Badge>;
  }
  if (user.status === "active") {
    return <Badge variant="distributed">Active</Badge>;
  }
  return <Badge variant="closed">Inactive</Badge>;
}

function statusSubtitle(user: UserRow) {
  if (user.status === "pending") return "Pending invite";
  if (user.status === "expired") return "Invite expired — resend to issue a new link";
  if (user.status === "active") return "Active member";
  return "Inactive member";
}

function isInviteStatus(status: UserRow["status"]) {
  return status === "pending" || status === "expired";
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
  const accountType = useAuthStore((s) => s.user?.account_type ?? "publisher");
  const update = useUpdateUser();
  const updateInvite = useUpdateInvite();
  const remove = useDeleteUser();
  const removeInvite = useDeleteInvite();
  const resend = useResendInvite();
  const resetPassword = useRequestPasswordReset();
  const uploadAvatar = useUploadUserAvatar();

  const [tab, setTab] = useState<DrawerTab>("profile");
  const [fullName, setFullName] = useState(user.full_name);
  const [email, setEmail] = useState(user.email);
  const [role, setRole] = useState(user.role);
  const initialEffective = useMemo(
    () => effectiveFromUserRow(user.role, accountType, user),
    [user, accountType]
  );
  const [effective, setEffective] = useState(initialEffective);
  const [permissionsDirty, setPermissionsDirty] = useState(false);

  useEffect(() => {
    setFullName(user.full_name);
    setEmail(user.email);
    setRole(user.role);
    setEffective(effectiveFromUserRow(user.role, accountType, user));
    setPermissionsDirty(false);
    setTab("profile");
  }, [user, accountType]);

  function onRoleChange(next: Role) {
    if (next !== role && permissionsDirty) {
      if (!window.confirm("Changing role will reset custom permissions to the new role defaults. Continue?")) {
        return;
      }
    }
    setRole(next);
    setEffective(presetForRole(next, accountType));
    setPermissionsDirty(false);
  }

  const trimmedName = fullName.trim();
  const trimmedEmail = email.trim();
  const profileUnchanged =
    trimmedName === user.full_name && trimmedEmail === user.email && role === user.role;
  const unchanged = profileUnchanged && !permissionsDirty;
  const invalid = !trimmedName || !trimmedEmail;
  const saving = update.isPending || updateInvite.isPending;

  function save() {
    if (!isValidLeadVisibility(role, effective.lead_scope)) {
      toast.error("Select at least one lead visibility option");
      return;
    }

    const body: Record<string, unknown> = {};
    if (trimmedName !== user.full_name) body.full_name = trimmedName;
    if (trimmedEmail !== user.email) body.email = trimmedEmail;
    if (role !== user.role) body.role = role;
    if (permissionsDirty || role !== user.role) {
      body.permissions = deltaFromEffective(role, accountType, effective);
    }
    if (Object.keys(body).length === 0) return;

    const onSuccess = () => {
      toast.success("Saved");
      onClose();
    };
    const onError = (e: unknown) => toast.error(errorMessage(e));

    if (isInviteStatus(user.status)) {
      updateInvite.mutate({ id: user.invite_id, body }, { onSuccess, onError });
    } else {
      update.mutate({ id: user.id, body }, { onSuccess, onError });
    }
  }

  function revoke() {
    const onSuccess = () => {
      toast.success(isInviteStatus(user.status) ? "Invite revoked" : "User deactivated");
      onClose();
    };
    const onError = (e: unknown) => toast.error(errorMessage(e));

    if (isInviteStatus(user.status)) {
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

      <div className="flex overflow-x-auto border-b border-gray-100 px-5 py-[10px]">
        {(
          [
            { id: "profile" as const, label: "Profile" },
            { id: "permissions" as const, label: "Permissions" },
          ] as const
        ).map(({ id, label }) => (
          <button
            key={id}
            type="button"
            onClick={() => setTab(id)}
            className={cn(
              "-mb-px shrink-0 border-b-2 px-2.5 py-1.5 text-base font-semibold transition-colors",
              tab === id
                ? "border-jade-500 text-jade-700"
                : "border-transparent text-gray-400 hover:text-gray-600"
            )}
          >
            {label}
          </button>
        ))}
      </div>

      <DrawerBody>
        {tab === "profile" && (
          <div className="flex flex-col gap-4">
            {user.status !== "pending" && user.status !== "expired" && (
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
              <Select value={role} onChange={(e) => onRoleChange(e.target.value as Role)}>
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
        )}

        {tab === "permissions" && (
          <PermissionsEditor
            role={role}
            accountType={accountType}
            effective={effective}
            onChange={(next) => {
              setEffective(next);
              setPermissionsDirty(true);
            }}
          />
        )}
      </DrawerBody>

      <DrawerFooter className="flex flex-col gap-2">
        <Button disabled={unchanged || invalid || saving} onClick={save}>
          Save
        </Button>
        {isInviteStatus(user.status) && (
          <Button
            variant="secondary"
            disabled={resend.isPending}
            onClick={() =>
              resend.mutate(user.invite_id, {
                onSuccess: () => toast.success("Invite resent"),
                onError: (e) => toast.error(errorMessage(e)),
              })
            }
          >
            <Mail className="mr-2 h-4 w-4" />
            Resend invite
          </Button>
        )}
        {!isInviteStatus(user.status) && (
          <Button
            variant="secondary"
            disabled={resetPassword.isPending}
            onClick={() =>
              resetPassword.mutate(user.email, {
                onSuccess: () => toast.success("Password reset email sent"),
                onError: (e) => toast.error(errorMessage(e)),
              })
            }
          >
            <KeyRound className="mr-2 h-4 w-4" />
            Send password reset
          </Button>
        )}
        <Button
          variant="danger"
          disabled={user.status === "inactive" || remove.isPending || removeInvite.isPending}
          onClick={revoke}
        >
          {isInviteStatus(user.status) ? "Revoke invite" : "Deactivate user"}
        </Button>
      </DrawerFooter>
    </div>
  );
}
