import { useState } from "react";
import { useBuyers, useCreateBuyer } from "@/features/admin/hooks";
import { BuyerDetailDrawer } from "@/features/admin/BuyerDetailDrawer";
import { PageHeader } from "@/components/layout/PageHeader";
import { PageBody } from "@/components/layout/PageBody";
import { Table, THead, TH, TBody, TR, TD } from "@/components/ui/table";
import { Button } from "@/components/ui/button";
import { Input, Label, Select } from "@/components/ui/input";
import { Spinner, EmptyState } from "@/components/ui/misc";
import { FormDrawer } from "@/components/ui/dialog";
import { cn, formatMoney } from "@/lib/utils";
import { TIMEZONES } from "@/lib/timezones";
import { toast } from "@/store/toastStore";
import { apiError } from "@/lib/api";
import { Plus } from "lucide-react";

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
  const [selectedBuyerId, setSelectedBuyerId] = useState<number | null>(null);
  const [selectedLeadCount, setSelectedLeadCount] = useState(0);
  const [open, setOpen] = useState(false);
  const [form, setForm] = useState(emptyForm);

  const canSubmit =
    form.name.trim() &&
    form.admin_first_name.trim() &&
    form.admin_last_name.trim() &&
    form.admin_email.trim() &&
    form.starting_balance >= 0;

  function openBuyer(id: number, leadCount: number) {
    setSelectedBuyerId(id);
    setSelectedLeadCount(leadCount);
  }

  return (
    <>
      <PageHeader
        action={
          <Button onClick={() => setOpen(true)}>
            <Plus className="h-4 w-4" /> Add Buyer
          </Button>
        }
      />
      <PageBody>
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
              </tr>
            </THead>
            <TBody>
              {(buyers ?? []).map((b) => (
                <TR key={b.id} onClick={() => openBuyer(b.id, b.lead_count)}>
                  <TD className="font-medium text-gray-800">{b.name}</TD>
                  <TD>{b.lead_count}</TD>
                  <TD className={cn(b.balance < 0 && "font-semibold text-danger")}>{formatMoney(b.balance)}</TD>
                </TR>
              ))}
            </TBody>
          </Table>
        )}

        <BuyerDetailDrawer
          buyerId={selectedBuyerId}
          leadCount={selectedLeadCount}
          onClose={() => setSelectedBuyerId(null)}
        />

        <FormDrawer
          open={open}
          onClose={() => setOpen(false)}
          title="Add Buyer"
          width={560}
          footer={
            <>
              <Button variant="secondary" onClick={() => setOpen(false)}>
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
            </>
          }
        >
          <div className="space-y-2.5">
            <div>
              <div className="mb-2 text-sm font-semibold text-gray-800">Company</div>
              <div className="space-y-2.5">
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
              <div className="mb-2 text-sm font-semibold text-gray-800">Admin contact</div>
              <div className="space-y-2.5">
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
              <div className="mb-2 text-sm font-semibold text-gray-800">Account settings</div>
              <div className="space-y-2.5">
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
          </div>
        </FormDrawer>
      </PageBody>
    </>
  );
}
