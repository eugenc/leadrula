import { useState } from "react";
import {
  useBuyers,
  useCreateBuyer,
  usePartnerships,
  useRequestPartnership,
  useAcceptPartnership,
  useRejectPartnership,
} from "@/features/admin/hooks";
import { BuyerDetailDrawer } from "@/features/admin/BuyerDetailDrawer";
import { PageHeader } from "@/components/layout/PageHeader";
import { PageBody } from "@/components/layout/PageBody";
import { Table, THead, TH, TBody, TR, TD } from "@/components/ui/table";
import { Button } from "@/components/ui/button";
import { Input, Label, Select } from "@/components/ui/input";
import { Spinner, EmptyState, Card } from "@/components/ui/misc";
import { FormDrawer } from "@/components/ui/dialog";
import { cn, formatMoney } from "@/lib/utils";
import { TIMEZONES } from "@/lib/timezones";
import { toast } from "@/store/toastStore";
import { errorMessage } from "@/lib/api";
import { Link2, Plus } from "lucide-react";
import type { Partnership } from "@/types";

const emptyForm = {
  name: "",
  website: "",
  admin_first_name: "",
  admin_last_name: "",
  admin_email: "",
  starting_balance: 0,
  timezone: "America/Toronto",
  collaborate_enabled: true,
};

function PendingPartnerships({
  items,
  onAccept,
  onReject,
  accepting,
  rejecting,
}: {
  items: Partnership[];
  onAccept: (id: number) => void;
  onReject: (id: number) => void;
  accepting: boolean;
  rejecting: boolean;
}) {
  const pending = items.filter((p) => p.status === "pending_publisher");
  if (pending.length === 0) return null;

  return (
    <Card className="mb-4 p-4">
      <h2 className="mb-3 text-sm font-semibold text-gray-800">Pending partnership requests</h2>
      <div className="space-y-3">
        {pending.map((p) => (
          <div key={p.id} className="flex flex-wrap items-center justify-between gap-3 rounded-md border border-amber-100 bg-amber-50 px-3 py-2">
            <div>
              <div className="text-sm font-medium text-gray-800">{p.partner_name}</div>
              <div className="text-xs text-gray-500">{p.partner_handler_id}</div>
            </div>
            <div className="flex gap-2">
              <Button size="sm" disabled={accepting} onClick={() => onAccept(p.id)}>
                Accept
              </Button>
              <Button size="sm" variant="secondary" disabled={rejecting} onClick={() => onReject(p.id)}>
                Reject
              </Button>
            </div>
          </div>
        ))}
      </div>
    </Card>
  );
}

export function BuyersPage() {
  const { data: buyers, isLoading } = useBuyers();
  const { data: partnerships } = usePartnerships();
  const create = useCreateBuyer();
  const request = useRequestPartnership();
  const accept = useAcceptPartnership();
  const reject = useRejectPartnership();
  const [selectedBuyerId, setSelectedBuyerId] = useState<number | null>(null);
  const [selectedLeadCount, setSelectedLeadCount] = useState(0);
  const [open, setOpen] = useState(false);
  const [linkOpen, setLinkOpen] = useState(false);
  const [linkHandlerId, setLinkHandlerId] = useState("");
  const [form, setForm] = useState(emptyForm);

  const startingBalance = Number.isNaN(form.starting_balance) ? 0 : form.starting_balance;

  const canSubmit =
    form.name.trim() &&
    form.admin_first_name.trim() &&
    form.admin_last_name.trim() &&
    form.admin_email.trim() &&
    startingBalance >= 0;

  function openBuyer(id: number, leadCount: number) {
    setSelectedBuyerId(id);
    setSelectedLeadCount(leadCount);
  }

  function submitLink() {
    request.mutate(
      { buyer_handler_id: linkHandlerId.trim().toUpperCase() },
      {
        onSuccess: () => {
          toast.success("Partnership request sent");
          setLinkOpen(false);
          setLinkHandlerId("");
        },
        onError: (e) => toast.error(errorMessage(e)),
      }
    );
  }

  return (
    <>
      <PageHeader
        action={
          <div className="flex gap-2">
            <Button variant="secondary" onClick={() => setLinkOpen(true)}>
              <Link2 className="h-4 w-4" /> Link Buyer
            </Button>
            <Button onClick={() => setOpen(true)}>
              <Plus className="h-4 w-4" /> Add Buyer
            </Button>
          </div>
        }
      />
      <PageBody>
        <PendingPartnerships
          items={partnerships ?? []}
          onAccept={(id) =>
            accept.mutate(id, {
              onSuccess: () => toast.success("Partnership accepted"),
              onError: (e) => toast.error(errorMessage(e)),
            })
          }
          onReject={(id) =>
            reject.mutate(id, {
              onSuccess: () => toast.success("Request rejected"),
              onError: (e) => toast.error(errorMessage(e)),
            })
          }
          accepting={accept.isPending}
          rejecting={reject.isPending}
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
                <TH>Handler ID</TH>
                <TH>Leads</TH>
                <TH>Balance</TH>
              </tr>
            </THead>
            <TBody>
              {(buyers ?? []).map((b) => (
                <TR key={b.id} onClick={() => openBuyer(b.id, b.lead_count)}>
                  <TD className="font-medium text-gray-800">{b.name}</TD>
                  <TD className="font-mono text-xs text-gray-500">{b.handler_id}</TD>
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
          open={linkOpen}
          onClose={() => setLinkOpen(false)}
          title="Link Buyer"
          width={420}
          footer={
            <>
              <Button variant="secondary" onClick={() => setLinkOpen(false)}>
                Cancel
              </Button>
              <Button disabled={!linkHandlerId.trim() || request.isPending} onClick={submitLink}>
                Send Request
              </Button>
            </>
          }
        >
          <p className="mb-3 text-sm text-gray-500">
            Enter an existing buyer&apos;s handler ID. They must accept before you can create a contract.
          </p>
          <Label>Buyer handler ID</Label>
          <Input
            placeholder="B-XXXXX"
            value={linkHandlerId}
            onChange={(e) => setLinkHandlerId(e.target.value.toUpperCase())}
          />
        </FormDrawer>

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
                      starting_balance: startingBalance,
                      timezone: form.timezone,
                      collaborate_enabled: form.collaborate_enabled,
                    },
                    {
                      onSuccess: () => {
                        toast.success(`Buyer created — invite sent to ${form.admin_email.trim()}`);
                        setOpen(false);
                        setForm(emptyForm);
                      },
                      onError: (e) => toast.error(errorMessage(e)),
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
                    value={Number.isNaN(form.starting_balance) ? "" : form.starting_balance}
                    onChange={(e) => {
                      const v = e.target.value;
                      setForm({ ...form, starting_balance: v === "" ? NaN : Number(v) });
                    }}
                    onBlur={() => {
                      if (Number.isNaN(form.starting_balance)) {
                        setForm({ ...form, starting_balance: 0 });
                      }
                    }}
                  />
                </div>
                <label className="flex cursor-pointer items-center gap-2 text-sm text-gray-700">
                  <input
                    type="checkbox"
                    checked={form.collaborate_enabled}
                    onChange={(e) => setForm({ ...form, collaborate_enabled: e.target.checked })}
                    className="rounded border-gray-300"
                  />
                  Enable collaboration (publisher admins can log in as buyer admin)
                </label>
              </div>
            </div>
          </div>
        </FormDrawer>
      </PageBody>
    </>
  );
}
