import { useState } from "react";
import { useUsersList, useInviteUser, useUpdateUser, useDeleteUser } from "@/features/admin/hooks";
import { PageHeader } from "@/components/layout/PageHeader";
import { Table, THead, TH, TBody, TR, TD } from "@/components/ui/table";
import { Button } from "@/components/ui/button";
import { Input, Label, Select } from "@/components/ui/input";
import { Badge, Spinner } from "@/components/ui/misc";
import { Dialog } from "@/components/ui/dialog";
import { Plus, Trash2 } from "lucide-react";
import { toast } from "@/store/toastStore";
import { apiError } from "@/lib/api";

export function UsersPage() {
  const { data: users, isLoading } = useUsersList();
  const invite = useInviteUser();
  const update = useUpdateUser();
  const remove = useDeleteUser();
  const [open, setOpen] = useState(false);
  const [form, setForm] = useState({ email: "", full_name: "", role: "user" });

  return (
    <div>
      <PageHeader
        title="Users"
        subtitle="Members of this account."
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
              <TH />
            </tr>
          </THead>
          <TBody>
            {(users ?? []).map((u) => (
              <TR key={u.id}>
                <TD className="font-semibold">{u.full_name}</TD>
                <TD>{u.email}</TD>
                <TD>
                  <Select
                    value={u.role}
                    className="h-8 w-32"
                    onChange={(e) => update.mutate({ id: u.id, body: { role: e.target.value } })}
                  >
                    <option value="admin">admin</option>
                    <option value="user">user</option>
                    <option value="follower">follower</option>
                  </Select>
                </TD>
                <TD>
                  <Badge variant={u.is_active ? "green" : "muted"}>{u.is_active ? "active" : "inactive"}</Badge>
                </TD>
                <TD>
                  <button
                    onClick={() => remove.mutate(u.id, { onError: (e) => toast.error(apiError(e).message) })}
                    className="text-pd-muted hover:text-pd-red"
                  >
                    <Trash2 className="h-4 w-4" />
                  </button>
                </TD>
              </TR>
            ))}
          </TBody>
        </Table>
      )}

      <Dialog open={open} onClose={() => setOpen(false)} title="Invite User">
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
            <Select value={form.role} onChange={(e) => setForm({ ...form, role: e.target.value })}>
              <option value="admin">admin</option>
              <option value="user">user</option>
              <option value="follower">follower</option>
            </Select>
          </div>
          <div className="flex justify-end gap-2 pt-2">
            <Button variant="outline" onClick={() => setOpen(false)}>
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
                  onError: (e) => toast.error(apiError(e).message),
                })
              }
            >
              Send Invite
            </Button>
          </div>
        </div>
      </Dialog>
    </div>
  );
}
