import { useState } from "react";
import { useUsersList, useInviteUser, useResendInvite } from "@/features/admin/hooks";
import { UserDetailDrawer } from "@/features/admin/UserDetailDrawer";
import { PageHeader } from "@/components/layout/PageHeader";
import { IconButton } from "@/components/layout/IconButton";
import { Table, THead, TH, TBody, TR, TD } from "@/components/ui/table";
import { Button } from "@/components/ui/button";
import { Input, Label, Select } from "@/components/ui/input";
import { Badge, Spinner } from "@/components/ui/misc";
import { FormDrawer } from "@/components/ui/dialog";
import { Mail, Plus } from "lucide-react";
import { toast } from "@/store/toastStore";
import { errorMessage } from "@/lib/api";
import { formatRole } from "@/lib/utils";
import type { Role, UserRow } from "@/types";

const ROLES: { value: Role; label: string }[] = [
  { value: "admin", label: "Admin" },
  { value: "user", label: "User" },
  { value: "follower", label: "Follower" },
];

function rowKey(u: UserRow) {
  return u.status === "pending" ? `invite-${u.invite_id}` : `user-${u.id}`;
}

function statusBadge(u: UserRow) {
  if (u.status === "pending") {
    return <Badge variant="pending">Pending</Badge>;
  }
  if (u.status === "active") {
    return <Badge variant="distributed">Active</Badge>;
  }
  return <Badge variant="closed">Inactive</Badge>;
}

export function UsersPage() {
  const { data: users, isLoading } = useUsersList();
  const invite = useInviteUser();
  const resend = useResendInvite();
  const [selectedUser, setSelectedUser] = useState<UserRow | null>(null);
  const [resendingId, setResendingId] = useState<number | null>(null);
  const [open, setOpen] = useState(false);
  const [form, setForm] = useState({ email: "", full_name: "", role: "user" as Role });

  return (
    <>
      <PageHeader
        action={
          <Button onClick={() => setOpen(true)}>
            <Plus className="h-4 w-4" /> Invite User
          </Button>
        }
      />
        {isLoading ? (
          <Spinner className="h-6 w-6" />
        ) : (
          <Table>
            <THead>
              <tr>
                <TH>Name</TH>
                <TH>Email</TH>
                <TH>Role</TH>
                <TH>Status</TH>
                <TH className="min-w-0 w-12" />
              </tr>
            </THead>
            <TBody>
              {(users ?? []).map((u) => (
                <TR key={rowKey(u)} onClick={() => setSelectedUser(u)}>
                  <TD className="font-medium text-gray-800">{u.full_name || "—"}</TD>
                  <TD>{u.email}</TD>
                  <TD>{formatRole(u.role)}</TD>
                  <TD>{statusBadge(u)}</TD>
                  <TD>
                    {u.status === "pending" && (
                      <div className="flex justify-end" onClick={(e) => e.stopPropagation()}>
                        <IconButton
                          aria-label="Resend invitation"
                          disabled={resend.isPending && resendingId === u.invite_id}
                          onClick={() => {
                            setResendingId(u.invite_id);
                            resend.mutate(u.invite_id, {
                              onSuccess: () => toast.success("Invite resent"),
                              onError: (e) => toast.error(errorMessage(e)),
                              onSettled: () => setResendingId(null),
                            });
                          }}
                        >
                          <Mail className="h-4 w-4" />
                        </IconButton>
                      </div>
                    )}
                  </TD>
                </TR>
              ))}
            </TBody>
          </Table>
        )}

        <UserDetailDrawer user={selectedUser} onClose={() => setSelectedUser(null)} />

        <FormDrawer
          open={open}
          onClose={() => setOpen(false)}
          title="Invite User"
          footer={
            <>
              <Button variant="secondary" onClick={() => setOpen(false)}>
                Cancel
              </Button>
              <Button
                disabled={!form.email || !form.full_name}
                onClick={() =>
                  invite.mutate(form, {
                    onSuccess: () => {
                      toast.success("Invite sent");
                      setOpen(false);
                      setForm({ email: "", full_name: "", role: "user" });
                    },
                    onError: (e) => toast.error(errorMessage(e)),
                  })
                }
              >
                Send Invite
              </Button>
            </>
          }
        >
          <div className="space-y-3">
            <div>
              <Label>Full Name</Label>
              <Input value={form.full_name} onChange={(e) => setForm({ ...form, full_name: e.target.value })} />
            </div>
            <div>
              <Label>Email</Label>
              <Input type="email" value={form.email} onChange={(e) => setForm({ ...form, email: e.target.value })} />
            </div>
            <div>
              <Label>Role</Label>
              <Select value={form.role} onChange={(e) => setForm({ ...form, role: e.target.value as Role })}>
                {ROLES.map((r) => (
                  <option key={r.value} value={r.value}>
                    {r.label}
                  </option>
                ))}
              </Select>
            </div>
          </div>
        </FormDrawer>
    </>
  );
}
