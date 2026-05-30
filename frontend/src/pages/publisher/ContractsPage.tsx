import { useState } from "react";
import {
  useContracts,
  useCreateContract,
  useDeleteContract,
  useBuyers,
  useBuyerPipelines,
} from "@/features/admin/hooks";
import { usePipelines, useStages } from "@/features/leads/hooks";
import { PageHeader } from "@/components/layout/PageHeader";
import { PageBody } from "@/components/layout/PageBody";
import { IconButton } from "@/components/layout/IconButton";
import { Table, THead, TH, TBody, TR, TD } from "@/components/ui/table";
import { Button } from "@/components/ui/button";
import { Input, Label, Select } from "@/components/ui/input";
import { Spinner, EmptyState } from "@/components/ui/misc";
import { Dialog } from "@/components/ui/dialog";
import { Plus, Trash2 } from "lucide-react";
import { formatMoney } from "@/lib/utils";
import { toast } from "@/store/toastStore";
import { apiError } from "@/lib/api";
import { ContractDetailDrawer } from "@/features/admin/ContractDetailDrawer";
import { ContractStatusBadge } from "@/features/admin/contractStatus";
import type { Contract } from "@/types";

export function ContractsPage() {
  const { data: contracts, isLoading } = useContracts();
  const remove = useDeleteContract();
  const [open, setOpen] = useState(false);
  const [selected, setSelected] = useState<Contract | null>(null);

  return (
    <>
      <PageHeader
        action={
          <Button onClick={() => setOpen(true)}>
            <Plus className="h-4 w-4" /> New Contract
          </Button>
        }
      />
      <PageBody>
      {isLoading ? (
        <Spinner className="h-6 w-6" />
      ) : (contracts ?? []).length === 0 ? (
        <EmptyState title="No contracts yet." />
      ) : (
        <Table>
          <THead>
            <tr>
              <TH>Buyer</TH>
              <TH>Name</TH>
              <TH>Rate / Lead</TH>
              <TH>Status</TH>
              <TH />
            </tr>
          </THead>
          <TBody>
            {(contracts ?? []).map((c) => (
              <TR key={c.id} onClick={() => setSelected(c)}>
                <TD className="font-semibold">{c.buyer_name}</TD>
                <TD>{c.name}</TD>
                <TD>{formatMoney(c.rate_per_lead)}</TD>
                <TD>
                  <ContractStatusBadge status={c.status} />
                </TD>
                <TD>
                  <div className="flex justify-end">
                    <IconButton
                      variant="danger"
                      onClick={(e) => {
                        e.stopPropagation();
                        remove.mutate(c.id, { onError: (err) => toast.error(apiError(err).message) });
                      }}
                    >
                      <Trash2 className="h-4 w-4" />
                    </IconButton>
                  </div>
                </TD>
              </TR>
            ))}
          </TBody>
        </Table>
      )}

      {open && <CreateContractDialog onClose={() => setOpen(false)} />}
      <ContractDetailDrawer contract={selected} onClose={() => setSelected(null)} />
      </PageBody>
    </>
  );
}

function CreateContractDialog({ onClose }: { onClose: () => void }) {
  const { data: buyers } = useBuyers();
  const { data: pubPipelines } = usePipelines();
  const create = useCreateContract();
  const [form, setForm] = useState({
    buyer_id: 0,
    name: "Contract",
    source_pipeline_id: 0,
    source_stage_id: 0,
    buyer_pipeline_id: 0,
    return_stage_id: 0,
    rate_per_lead: 25,
  });
  const { data: sourceStages } = useStages(form.source_pipeline_id || undefined);
  const { data: buyerPipelines } = useBuyerPipelines(form.buyer_id || null);

  function set<K extends keyof typeof form>(k: K, v: (typeof form)[K]) {
    setForm((f) => ({ ...f, [k]: v }));
  }

  return (
    <Dialog open onClose={onClose} title="New Contract">
      <div className="space-y-3">
        <div>
          <Label>Buyer</Label>
          <Select value={form.buyer_id} onChange={(e) => set("buyer_id", Number(e.target.value))}>
            <option value={0}>Select…</option>
            {(buyers ?? []).map((b) => (
              <option key={b.id} value={b.id}>
                {b.name}
              </option>
            ))}
          </Select>
        </div>
        <div>
          <Label>Name</Label>
          <Input value={form.name} onChange={(e) => set("name", e.target.value)} />
        </div>
        <div className="grid grid-cols-2 gap-3">
          <div>
            <Label>Source pipeline (yours)</Label>
            <Select value={form.source_pipeline_id} onChange={(e) => set("source_pipeline_id", Number(e.target.value))}>
              <option value={0}>Select…</option>
              {(pubPipelines ?? []).map((p) => (
                <option key={p.id} value={p.id}>
                  {p.name}
                </option>
              ))}
            </Select>
          </div>
          <div>
            <Label>Source stage (distribute from)</Label>
            <Select value={form.source_stage_id} onChange={(e) => set("source_stage_id", Number(e.target.value))}>
              <option value={0}>Select…</option>
              {(sourceStages ?? []).map((s) => (
                <option key={s.id} value={s.id}>
                  {s.name}
                </option>
              ))}
            </Select>
          </div>
        </div>
        <div className="grid grid-cols-2 gap-3">
          <div>
            <Label>Buyer pipeline</Label>
            <Select value={form.buyer_pipeline_id} onChange={(e) => set("buyer_pipeline_id", Number(e.target.value))}>
              <option value={0}>Select…</option>
              {(buyerPipelines ?? []).map((p) => (
                <option key={p.id} value={p.id}>
                  {p.name}
                </option>
              ))}
            </Select>
          </div>
          <div>
            <Label>Return stage (yours)</Label>
            <Select value={form.return_stage_id} onChange={(e) => set("return_stage_id", Number(e.target.value))}>
              <option value={0}>Select…</option>
              {(sourceStages ?? []).map((s) => (
                <option key={s.id} value={s.id}>
                  {s.name}
                </option>
              ))}
            </Select>
          </div>
        </div>
        <div>
          <Label>Rate per lead (USD)</Label>
          <Input
            type="number"
            value={form.rate_per_lead}
            onChange={(e) => set("rate_per_lead", Number(e.target.value))}
          />
        </div>
        <div className="flex justify-end gap-2 pt-2">
          <Button variant="secondary" onClick={onClose}>
            Cancel
          </Button>
          <Button
            disabled={!form.buyer_id || !form.source_stage_id || !form.buyer_pipeline_id || !form.return_stage_id}
            onClick={() =>
              create.mutate(form, {
                onSuccess: () => {
                  toast.success("Contract created");
                  onClose();
                },
                onError: (e) => toast.error(apiError(e).message),
              })
            }
          >
            Create
          </Button>
        </div>
      </div>
    </Dialog>
  );
}
