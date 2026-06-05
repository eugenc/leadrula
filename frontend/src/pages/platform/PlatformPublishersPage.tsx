import { useEffect, useState } from "react";
import { format } from "date-fns";
import { ArrowRightLeft, Plus } from "lucide-react";
import {
  useCreatePublisher,
  usePlatformPublishers,
  useSwitchAccount,
} from "@/features/auth/switchHooks";
import { PlatformAccountStatusBadge } from "@/pages/platform/PlatformAccountStatusBadge";
import { PlatformPublisherDetailDrawer } from "@/pages/platform/PlatformPublisherDetailDrawer";
import type { PlatformAccount } from "@/types";
import { PageHeader } from "@/components/layout/PageHeader";
import { PageBody } from "@/components/layout/PageBody";
import { Table, THead, TH, TBody, TR, TD } from "@/components/ui/table";
import { Button } from "@/components/ui/button";
import { Input, Label } from "@/components/ui/input";
import { Spinner, EmptyState } from "@/components/ui/misc";
import { FormDrawer } from "@/components/ui/dialog";
import {
  PlatformAccountPagination,
  PlatformAccountSearchBar,
} from "@/pages/platform/PlatformAccountListControls";
import { toast } from "@/store/toastStore";
import { errorMessage } from "@/lib/api";

const emptyForm = {
  name: "",
  admin_first_name: "",
  admin_last_name: "",
  admin_email: "",
};

export function PlatformPublishersPage() {
  const [search, setSearch] = useState("");
  const [debouncedSearch, setDebouncedSearch] = useState("");
  const [page, setPage] = useState(1);
  const [limit, setLimit] = useState(25);
  const [open, setOpen] = useState(false);
  const [form, setForm] = useState(emptyForm);
  const [selectedPublisher, setSelectedPublisher] = useState<PlatformAccount | null>(null);

  useEffect(() => {
    const t = setTimeout(() => setDebouncedSearch(search.trim()), 300);
    return () => clearTimeout(t);
  }, [search]);

  useEffect(() => {
    setPage(1);
  }, [debouncedSearch, limit]);

  const { data, isLoading } = usePlatformPublishers({
    q: debouncedSearch || undefined,
    page,
    limit,
  });
  const switchAccount = useSwitchAccount();
  const create = useCreatePublisher();

  const rows = data?.items ?? [];
  const total = data?.total ?? 0;
  const hasSearch = debouncedSearch !== "";

  const canSubmit =
    form.name.trim() &&
    form.admin_first_name.trim() &&
    form.admin_last_name.trim() &&
    form.admin_email.trim();

  function submitCreate() {
    create.mutate(
      {
        name: form.name.trim(),
        admin_first_name: form.admin_first_name.trim(),
        admin_last_name: form.admin_last_name.trim(),
        admin_email: form.admin_email.trim(),
      },
      {
        onSuccess: () => {
          toast.success(`Publisher created — invite sent to ${form.admin_email.trim()}`);
          setOpen(false);
          setForm(emptyForm);
        },
        onError: (e) => toast.error(errorMessage(e)),
      }
    );
  }

  return (
    <>
      <PageHeader
        action={
          <Button onClick={() => setOpen(true)}>
            <Plus className="h-4 w-4" /> New Publisher
          </Button>
        }
      />
      <PageBody>
        <PlatformAccountSearchBar search={search} onSearchChange={setSearch} />

        {isLoading ? (
          <Spinner className="h-6 w-6" />
        ) : rows.length === 0 ? (
          <EmptyState title={hasSearch ? "No results." : "No publishers yet."} />
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
                {rows.map((p) => {
                  const suspended = p.operational_status === "suspended";
                  return (
                    <TR
                      key={p.id}
                      className="cursor-pointer"
                      onClick={() => setSelectedPublisher(p)}
                    >
                      <TD className="font-medium text-gray-800">{p.name}</TD>
                      <TD>
                        <PlatformAccountStatusBadge value={p.operational_status ?? "active"} />
                      </TD>
                      <TD className="font-mono text-xs text-gray-500">{p.handler_id}</TD>
                      <TD>{p.timezone}</TD>
                      <TD className="text-gray-500">
                        {p.created_at ? format(new Date(p.created_at), "MMM d, yyyy") : "—"}
                      </TD>
                      <TD>
                        <div className="flex justify-end">
                          <Button
                            size="sm"
                            variant="secondary"
                            disabled={switchAccount.isPending || suspended}
                            title={suspended ? "Account suspended" : undefined}
                            onClick={(e) => {
                              e.stopPropagation();
                              switchAccount.mutate(p.id);
                            }}
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

        <PlatformPublisherDetailDrawer
          publisher={selectedPublisher}
          onClose={() => setSelectedPublisher(null)}
        />

        <FormDrawer
          open={open}
          onClose={() => setOpen(false)}
          title="New Publisher"
          width={480}
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
              <Label>Publisher name</Label>
              <Input value={form.name} onChange={(e) => setForm({ ...form, name: e.target.value })} />
            </div>
            <div className="grid grid-cols-2 gap-2">
              <div>
                <Label>Admin first name</Label>
                <Input
                  value={form.admin_first_name}
                  onChange={(e) => setForm({ ...form, admin_first_name: e.target.value })}
                />
              </div>
              <div>
                <Label>Admin last name</Label>
                <Input
                  value={form.admin_last_name}
                  onChange={(e) => setForm({ ...form, admin_last_name: e.target.value })}
                />
              </div>
            </div>
            <div>
              <Label>Admin email</Label>
              <Input
                type="email"
                value={form.admin_email}
                onChange={(e) => setForm({ ...form, admin_email: e.target.value })}
              />
            </div>
          </div>
        </FormDrawer>
      </PageBody>
    </>
  );
}
