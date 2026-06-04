import { useEffect, useState } from "react";
import { format } from "date-fns";
import { ArrowRightLeft, Plus } from "lucide-react";
import {
  useCreatePlatformBuyer,
  usePlatformBuyers,
  useSwitchAccount,
  useUpdateBuyerStatus,
} from "@/features/auth/switchHooks";
import { PlatformAccountStatusCell } from "@/pages/platform/PlatformAccountStatusCell";
import type { AccountOperationalStatus } from "@/types";
import { PageHeader } from "@/components/layout/PageHeader";
import { PageBody } from "@/components/layout/PageBody";
import { Table, THead, TH, TBody, TR, TD } from "@/components/ui/table";
import { Button } from "@/components/ui/button";
import { Input, Label, Select } from "@/components/ui/input";
import { Spinner, EmptyState } from "@/components/ui/misc";
import { FormDrawer } from "@/components/ui/dialog";
import {
  PlatformAccountPagination,
  PlatformAccountSearchBar,
} from "@/pages/platform/PlatformAccountListControls";
import { TIMEZONES } from "@/lib/timezones";
import { toast } from "@/store/toastStore";
import { errorMessage, isInviteEmailError } from "@/lib/api";
import { queryClient } from "@/lib/queryClient";

const emptyForm = {
  name: "",
  website: "",
  admin_first_name: "",
  admin_last_name: "",
  admin_email: "",
  starting_balance: 0,
  timezone: "America/Toronto",
};

export function PlatformBuyersPage() {
  const [search, setSearch] = useState("");
  const [debouncedSearch, setDebouncedSearch] = useState("");
  const [page, setPage] = useState(1);
  const [limit, setLimit] = useState(25);
  const [open, setOpen] = useState(false);
  const [form, setForm] = useState(emptyForm);

  useEffect(() => {
    const t = setTimeout(() => setDebouncedSearch(search.trim()), 300);
    return () => clearTimeout(t);
  }, [search]);

  useEffect(() => {
    setPage(1);
  }, [debouncedSearch, limit]);

  const { data, isLoading } = usePlatformBuyers({
    q: debouncedSearch || undefined,
    page,
    limit,
  });
  const switchAccount = useSwitchAccount();
  const updateStatus = useUpdateBuyerStatus();
  const create = useCreatePlatformBuyer();

  const rows = data?.items ?? [];
  const total = data?.total ?? 0;
  const hasSearch = debouncedSearch !== "";

  const startingBalance = Number.isNaN(form.starting_balance) ? 0 : form.starting_balance;

  const canSubmit =
    form.name.trim() &&
    form.admin_first_name.trim() &&
    form.admin_last_name.trim() &&
    form.admin_email.trim() &&
    startingBalance >= 0;

  function submitCreate() {
    create.mutate(
      {
        name: form.name.trim(),
        admin_first_name: form.admin_first_name.trim(),
        admin_last_name: form.admin_last_name.trim(),
        admin_email: form.admin_email.trim(),
        website: form.website.trim() || undefined,
        starting_balance: startingBalance,
        timezone: form.timezone,
      },
      {
        onSuccess: () => {
          toast.success(`Buyer created — invite sent to ${form.admin_email.trim()}`);
          setOpen(false);
          setForm(emptyForm);
        },
        onError: (e) => {
          if (isInviteEmailError(e)) {
            void queryClient.invalidateQueries({ queryKey: ["platform-buyers"] });
            toast.error(
              "Buyer created, but invitation email failed — resend from buyer details when available"
            );
            setOpen(false);
            setForm(emptyForm);
            return;
          }
          toast.error(errorMessage(e));
        },
      }
    );
  }

  return (
    <>
      <PageHeader
        action={
          <Button onClick={() => setOpen(true)}>
            <Plus className="h-4 w-4" /> New Buyer
          </Button>
        }
      />
      <PageBody>
        <PlatformAccountSearchBar search={search} onSearchChange={setSearch} />

        {isLoading ? (
          <Spinner className="h-6 w-6" />
        ) : rows.length === 0 ? (
          <EmptyState title={hasSearch ? "No results." : "No buyers yet."} />
        ) : (
          <>
            <Table>
              <THead>
                <tr>
                  <TH>Name</TH>
                  <TH>Status</TH>
                  <TH>Handler ID</TH>
                  <TH>Timezone</TH>
                  <TH>Created</TH>
                  <TH />
                </tr>
              </THead>
              <TBody>
                {rows.map((b) => {
                  const suspended = b.operational_status === "suspended";
                  return (
                    <TR key={b.id}>
                      <TD className="font-medium text-gray-800">{b.name}</TD>
                      <TD>
                        <PlatformAccountStatusCell
                          value={b.operational_status ?? "active"}
                          disabled={updateStatus.isPending}
                          onChange={(status: AccountOperationalStatus) =>
                            updateStatus.mutate(
                              { id: b.id, operational_status: status },
                              {
                                onSuccess: () => toast.success("Status updated"),
                                onError: (e) => toast.error(errorMessage(e)),
                              }
                            )
                          }
                        />
                      </TD>
                      <TD className="font-mono text-xs text-gray-500">{b.handler_id}</TD>
                      <TD>{b.timezone}</TD>
                      <TD className="text-gray-500">
                        {b.created_at ? format(new Date(b.created_at), "MMM d, yyyy") : "—"}
                      </TD>
                      <TD>
                        <div className="flex justify-end">
                          <Button
                            size="sm"
                            variant="secondary"
                            disabled={switchAccount.isPending || suspended}
                            title={suspended ? "Account suspended" : undefined}
                            onClick={() => switchAccount.mutate(b.id)}
                          >
                            <ArrowRightLeft className="h-3.5 w-3.5" /> Open
                          </Button>
                        </div>
                      </TD>
                    </TR>
                  );
                })}
              </TBody>
            </Table>

            <PlatformAccountPagination
              page={page}
              limit={limit}
              total={total}
              onPageChange={setPage}
              onLimitChange={setLimit}
            />
          </>
        )}

        <FormDrawer
          open={open}
          onClose={() => setOpen(false)}
          title="New Buyer"
          width={560}
          footer={
            <>
              <Button variant="secondary" onClick={() => setOpen(false)}>
                Cancel
              </Button>
              <Button disabled={!canSubmit || create.isPending} onClick={submitCreate}>
                Create & invite admin
              </Button>
            </>
          }
        >
          <div className="space-y-2.5">
            <div>
              <div className="mb-2 text-sm font-semibold text-gray-800">Company</div>
              <div className="space-y-2.5">
                <div>
                  <Label>Company name</Label>
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
                    <Label>First name</Label>
                    <Input
                      value={form.admin_first_name}
                      onChange={(e) => setForm({ ...form, admin_first_name: e.target.value })}
                    />
                  </div>
                  <div>
                    <Label>Last name</Label>
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
                  <Label>Starting balance</Label>
                  <Input
                    type="number"
                    min={0}
                    step="0.01"
                    value={form.starting_balance}
                    onChange={(e) =>
                      setForm({ ...form, starting_balance: parseFloat(e.target.value) || 0 })
                    }
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
