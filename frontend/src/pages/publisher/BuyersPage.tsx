import { useState } from "react";
import { useBuyers, useCreateBuyer, useBuyerBilling, useManualInvoice } from "@/features/admin/hooks";
import { PageHeader } from "@/components/layout/PageHeader";
import { Table, THead, TH, TBody, TR, TD } from "@/components/ui/table";
import { Button } from "@/components/ui/button";
import { Input, Label, Select } from "@/components/ui/input";
import { Spinner, EmptyState, Badge } from "@/components/ui/misc";
import { Dialog } from "@/components/ui/dialog";
import { formatMoney } from "@/lib/utils";
import { format } from "date-fns";
import { toast } from "@/store/toastStore";
import { apiError } from "@/lib/api";
import { Plus } from "lucide-react";

const TIMEZONES = [
  "America/Toronto",
  "America/New_York",
  "America/Chicago",
  "America/Denver",
  "America/Los_Angeles",
  "America/Phoenix",
  "America/Anchorage",
  "Pacific/Honolulu",
  "UTC",
] as const;

const emptyForm = {
  name: "",
  website: "",
  admin_first_name: "",
  admin_last_name: "",
  admin_email: "",
  starting_balance: 0,
  timezone: "America/Toronto",
};

export function BuyersPage() {
  const { data: buyers, isLoading } = useBuyers();
  const create = useCreateBuyer();
  const [detail, setDetail] = useState<{ id: number; name: string } | null>(null);
  const [open, setOpen] = useState(false);
  const [form, setForm] = useState(emptyForm);

  const canSubmit =
    form.name.trim() &&
    form.admin_first_name.trim() &&
    form.admin_last_name.trim() &&
    form.admin_email.trim() &&
    form.starting_balance >= 0;

  return (
    <div>
      <PageHeader
        title="Buyers"
        subtitle="Accounts purchasing your leads."
        action={
          <Button onClick={() => setOpen(true)}>
            <Plus className="h-4 w-4" /> Add Buyer
          </Button>
        }
      />
      {isLoading ? (
        <Spinner className="h-6 w-6" />
      ) : (buyers ?? []).length === 0 ? (
        <EmptyState title="No buyers yet." />
      ) : (
        <Table>
          <THead>
            <tr>
              <TH>Buyer</TH>
              <TH>Leads</TH>
              <TH>Balance</TH>
              <TH />
            </tr>
          </THead>
          <TBody>
            {(buyers ?? []).map((b) => (
              <TR key={b.id}>
                <TD className="font-semibold">{b.name}</TD>
                <TD>{b.lead_count}</TD>
                <TD className={b.balance < 0 ? "font-semibold text-pd-red" : ""}>{formatMoney(b.balance)}</TD>
                <TD>
                  <div className="flex justify-end">
                    <Button size="sm" variant="outline" onClick={() => setDetail({ id: b.id, name: b.name })}>
                      View
                    </Button>
                  </div>
                </TD>
              </TR>
            ))}
          </TBody>
        </Table>
      )}
      {detail && <BuyerDetail id={detail.id} name={detail.name} onClose={() => setDetail(null)} />}

      <Dialog open={open} onClose={() => setOpen(false)} title="Add Buyer" className="max-w-lg">
        <div className="space-y-4">
          <div>
            <div className="mb-2 text-sm font-semibold">Company</div>
            <div className="space-y-3">
              <div>
                <Label>Company Name</Label>
                <Input value={form.name} onChange={(e) => setForm({ ...form, name: e.target.value })} />
              </div>
              <div>
                <Label>Website</Label>
                <Input
                  type="url"
                  placeholder="https://example.com"
                  value={form.website}
                  onChange={(e) => setForm({ ...form, website: e.target.value })}
                />
              </div>
            </div>
          </div>

          <div>
            <div className="mb-2 text-sm font-semibold">Admin contact</div>
            <div className="space-y-3">
              <div className="grid grid-cols-2 gap-2">
                <div>
                  <Label>First Name</Label>
                  <Input
                    value={form.admin_first_name}
                    onChange={(e) => setForm({ ...form, admin_first_name: e.target.value })}
                  />
                </div>
                <div>
                  <Label>Last Name</Label>
                  <Input
                    value={form.admin_last_name}
                    onChange={(e) => setForm({ ...form, admin_last_name: e.target.value })}
                  />
                </div>
              </div>
              <div>
                <Label>Email</Label>
                <Input
                  type="email"
                  value={form.admin_email}
                  onChange={(e) => setForm({ ...form, admin_email: e.target.value })}
                />
              </div>
            </div>
          </div>

          <div>
            <div className="mb-2 text-sm font-semibold">Account settings</div>
            <div className="space-y-3">
              <div>
                <Label>Timezone</Label>
                <Select value={form.timezone} onChange={(e) => setForm({ ...form, timezone: e.target.value })}>
                  {TIMEZONES.map((tz) => (
                    <option key={tz} value={tz}>
                      {tz}
                    </option>
                  ))}
                </Select>
              </div>
              <div>
                <Label>Starting Balance</Label>
                <Input
                  type="number"
                  min={0}
                  step={0.01}
                  value={form.starting_balance}
                  onChange={(e) => setForm({ ...form, starting_balance: Number(e.target.value) })}
                />
              </div>
            </div>
          </div>

          <div className="flex justify-end gap-2 pt-2">
            <Button variant="outline" onClick={() => setOpen(false)}>
              Cancel
            </Button>
            <Button
              disabled={!canSubmit || create.isPending}
              onClick={() =>
                create.mutate(
                  {
                    name: form.name.trim(),
                    admin_first_name: form.admin_first_name.trim(),
                    admin_last_name: form.admin_last_name.trim(),
                    admin_email: form.admin_email.trim(),
                    website: form.website.trim() || undefined,
                    starting_balance: form.starting_balance,
                    timezone: form.timezone,
                  },
                  {
                    onSuccess: () => {
                      toast.success(`Buyer created — invite sent to ${form.admin_email.trim()}`);
                      setOpen(false);
                      setForm(emptyForm);
                    },
                    onError: (e) => toast.error(apiError(e).message),
                  }
                )
              }
            >
              Create Buyer
            </Button>
          </div>
        </div>
      </Dialog>
    </div>
  );
}

function BuyerDetail({ id, name, onClose }: { id: number; name: string; onClose: () => void }) {
  const { data } = useBuyerBilling(id);
  const invoice = useManualInvoice();
  const [amount, setAmount] = useState(0);
  const [desc, setDesc] = useState("");

  return (
    <Dialog open onClose={onClose} title={name} className="max-w-2xl">
      <div className="space-y-4">
        <div className="flex items-center gap-2">
          <span className="text-sm text-pd-muted">Balance:</span>
          <span className={`text-lg font-bold ${(data?.balance ?? 0) < 0 ? "text-pd-red" : ""}`}>
            {formatMoney(data?.balance)}
          </span>
        </div>

        <div className="rounded border border-pd-border p-3">
          <div className="mb-2 text-sm font-semibold">Manual invoice / adjustment</div>
          <div className="grid grid-cols-[1fr_2fr_auto] items-end gap-2">
            <div>
              <Label>Amount</Label>
              <Input type="number" value={amount} onChange={(e) => setAmount(Number(e.target.value))} />
            </div>
            <div>
              <Label>Description</Label>
              <Input value={desc} onChange={(e) => setDesc(e.target.value)} />
            </div>
            <Button
              disabled={!amount}
              onClick={() =>
                invoice.mutate(
                  { buyer_id: id, amount, description: desc || "manual invoice" },
                  {
                    onSuccess: () => {
                      toast.success("Invoice recorded");
                      setAmount(0);
                      setDesc("");
                    },
                    onError: (e) => toast.error(apiError(e).message),
                  }
                )
              }
            >
              Charge
            </Button>
          </div>
        </div>

        <div className="max-h-72 overflow-y-auto">
          <Table>
            <THead>
              <tr>
                <TH>Type</TH>
                <TH>Amount</TH>
                <TH>Balance</TH>
                <TH>When</TH>
              </tr>
            </THead>
            <TBody>
              {(data?.transactions ?? []).map((t) => (
                <TR key={t.id}>
                  <TD>
                    <Badge variant={t.amount < 0 ? "red" : "green"}>{t.type}</Badge>
                  </TD>
                  <TD className={t.amount < 0 ? "text-pd-red" : "text-pd-green"}>{formatMoney(t.amount)}</TD>
                  <TD>{formatMoney(t.balance_after)}</TD>
                  <TD>{format(new Date(t.created_at), "MMM d, h:mma")}</TD>
                </TR>
              ))}
            </TBody>
          </Table>
        </div>
      </div>
    </Dialog>
  );
}
