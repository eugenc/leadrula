import { useState } from "react";
import {
  useContracts,
  useCreateContract,
  useDeleteContract,
  useBuyers,
  usePartnerPublishers,
  useLinkPublisherPartnership,
} from "@/features/admin/hooks";
import { PageHeader } from "@/components/layout/PageHeader";
import { PageBody } from "@/components/layout/PageBody";
import { IconButton } from "@/components/layout/IconButton";
import { Table, THead, TH, TBody, TR, TD } from "@/components/ui/table";
import { Button } from "@/components/ui/button";
import { Input, Label, Select, Textarea } from "@/components/ui/input";
import { Spinner, EmptyState } from "@/components/ui/misc";
import { FormDrawer } from "@/components/ui/dialog";
import { SectionLabel } from "@/components/layout/SectionLabel";
import { Plus, Trash2 } from "lucide-react";
import { formatMoney } from "@/lib/utils";
import { toast } from "@/store/toastStore";
import { errorMessage } from "@/lib/api";
import { ContractDetailDrawer } from "@/features/admin/ContractDetailDrawer";
import { ContractStatusBadge } from "@/features/admin/contractStatus";
import { formatContractCap } from "@/features/admin/contractCap";
import {
  CreateContractCompensationList,
  compensationDraftToBody,
  compensationsValid,
  emptyCompensationDraft,
  type CompensationDraft,
} from "@/features/admin/CreateContractCompensationList";
import {
  ContractLeadCriteriaSection,
  emptyLeadCriteria,
} from "@/features/admin/ContractLeadCriteriaSection";
import type { ContractLeadCriteria } from "@/types";
import {
  CONTRACT_LEAD_TYPES,
  formatContractLeadType,
  isContractLeadType,
} from "@/features/admin/contractLeadType";
import {
  CONTRACT_TYPES,
  counterpartyLabel,
  formatBuyerWithType,
  formatContractType,
  isContractType,
} from "@/features/admin/contractType";
import type { Contract } from "@/types";

export function ContractsPage() {
  const { data: contracts, isLoading, isError, error } = useContracts();
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
      ) : isError ? (
        <EmptyState title="Could not load contracts." subtitle={errorMessage(error)} />
      ) : (contracts ?? []).length === 0 ? (
        <EmptyState title="No contracts yet." />
      ) : (
        <Table>
          <THead>
            <tr>
              <TH>Type</TH>
              <TH>Buyer</TH>
              <TH>Name</TH>
              <TH>Lead Type</TH>
              <TH>Cap</TH>
              <TH>Rate / Lead</TH>
              <TH>Distributed</TH>
              <TH>Status</TH>
              <TH />
            </tr>
          </THead>
          <TBody>
            {(contracts ?? []).map((c) => (
              <TR key={c.id} onClick={() => setSelected(c)}>
                <TD>{formatContractType(c.contract_type) || "Sell"}</TD>
                <TD className="font-semibold">{formatBuyerWithType(c.buyer_name, c.buyer_account_type)}</TD>
                <TD>{c.name}</TD>
                <TD>{formatContractLeadType(c.lead_type) || "—"}</TD>
                <TD>{formatContractCap(c)}</TD>
                <TD>{formatMoney(c.rate_per_lead)}</TD>
                <TD>{c.lead_count ?? 0}</TD>
                <TD>
                  <ContractStatusBadge status={c.status} />
                </TD>
                <TD>
                  <div className="flex justify-end">
                    <IconButton
                      variant="danger"
                      onClick={(e) => {
                        e.stopPropagation();
                        remove.mutate(c.id, { onError: (err) => toast.error(errorMessage(err)) });
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

      <CreateContractDrawer open={open} onClose={() => setOpen(false)} />
      <ContractDetailDrawer contract={selected} onClose={() => setSelected(null)} />
      </PageBody>
    </>
  );
}

function CreateContractDrawer({ open, onClose }: { open: boolean; onClose: () => void }) {
  const { data: buyers } = useBuyers();
  const { data: partnerPublishers } = usePartnerPublishers();
  const create = useCreateContract();
  const linkPub = useLinkPublisherPartnership();
  const [form, setForm] = useState({
    contract_type: "sell" as "buy" | "sell",
    buyer_id: 0,
    name: "Contract",
    lead_type: "",
    description: "",
  });
  const [compensations, setCompensations] = useState<CompensationDraft[]>(() => [
    emptyCompensationDraft({ source_pipeline_id: 0, source_stage_id: 0, counterparty_pipeline_id: 0, return_stage_id: 0 }),
  ]);
  const [leadCriteria, setLeadCriteria] = useState<ContractLeadCriteria>(emptyLeadCriteria);

  function set<K extends keyof typeof form>(k: K, v: (typeof form)[K]) {
    setForm((f) => ({ ...f, [k]: v }));
  }

  function setContractType(t: "buy" | "sell") {
    setForm((f) => {
      const next = { ...f, contract_type: t };
      if (t === "buy" && (buyers ?? []).some((b) => b.id === f.buyer_id)) {
        next.buyer_id = 0;
      }
      return next;
    });
  }

  const counterpartyIsPublisher = (partnerPublishers ?? []).some((p) => p.id === form.buyer_id);
  const isBuy = form.contract_type === "buy";
  const primary = compensations[0];

  const counterpartyValid =
    !!form.buyer_id &&
    (isBuy
      ? counterpartyIsPublisher
      : (buyers ?? []).some((b) => b.id === form.buyer_id) || counterpartyIsPublisher);

  const routingValid =
    primary?.delivery === "leads" ||
    (!!primary?.source_stage_id && !!primary?.counterparty_pipeline_id && !!primary?.return_stage_id);

  const canCreate =
    counterpartyValid &&
    isContractType(form.contract_type) &&
    isContractLeadType(form.lead_type) &&
    compensationsValid(compensations) &&
    routingValid;

  function submit() {
    const first = compensations[0];
    const rate = first?.kind === "flat_rate" && first.flat_amount !== "" ? Number(first.flat_amount) : 0;
    const body: Record<string, unknown> = {
      contract_type: form.contract_type,
      buyer_id: form.buyer_id,
      name: form.name,
      lead_type: form.lead_type,
      description: form.description,
      rate_per_lead: rate,
      source_pipeline_id: first?.source_pipeline_id ?? 0,
      source_stage_id: first?.source_stage_id ?? 0,
      buyer_pipeline_id: first?.counterparty_pipeline_id ?? 0,
      return_stage_id: first?.return_stage_id ?? 0,
      cap_period: first?.cap_period ?? "one_time",
      cap_total: compensationDraftToBody(first).cap_total,
      cap_max_daily: compensationDraftToBody(first).cap_max_daily,
      delivery: first?.delivery ?? "leads_pipeline",
      compensations: compensations.map(compensationDraftToBody),
      lead_criteria: leadCriteria,
    };

    const run = () =>
      create.mutate(body, {
        onSuccess: () => {
          toast.success("Contract created");
          onClose();
        },
        onError: (e) => toast.error(errorMessage(e)),
      });

    if (counterpartyIsPublisher) {
      const pub = (partnerPublishers ?? []).find((p) => p.id === form.buyer_id);
      if (pub?.handler_id) {
        linkPub.mutate(pub.handler_id, {
          onSuccess: run,
          onError: (e) => toast.error(errorMessage(e)),
        });
        return;
      }
    }
    run();
  }

  return (
    <FormDrawer
      open={open}
      onClose={onClose}
      title="New Contract"
      width={720}
      footer={
        <>
          <Button variant="secondary" onClick={onClose}>
            Cancel
          </Button>
          <Button disabled={!canCreate} onClick={submit}>
            Create
          </Button>
        </>
      }
    >
      <div className="space-y-4">
        <div>
          <SectionLabel className="mb-2">Contract Details</SectionLabel>
          <div className="space-y-3">
            <div>
              <Label>Contract type</Label>
              <Select
                value={form.contract_type}
                onChange={(e) => setContractType(e.target.value as "buy" | "sell")}
              >
                {CONTRACT_TYPES.map((t) => (
                  <option key={t.value} value={t.value}>
                    {t.label}
                  </option>
                ))}
              </Select>
            </div>
            <div>
              <Label>{counterpartyLabel(form.contract_type)}</Label>
              <Select value={form.buyer_id} onChange={(e) => set("buyer_id", Number(e.target.value))}>
                <option value={0}>Select…</option>
                {isBuy ? (
                  (partnerPublishers ?? []).map((p) => (
                    <option key={p.id} value={p.id}>
                      {formatBuyerWithType(p.name, "publisher")}
                    </option>
                  ))
                ) : (
                  <>
                    {(buyers ?? []).map((b) => (
                      <option key={b.id} value={b.id}>
                        {formatBuyerWithType(b.name, "buyer")}
                      </option>
                    ))}
                    {(partnerPublishers ?? []).map((p) => (
                      <option key={p.id} value={p.id}>
                        {formatBuyerWithType(p.name, "publisher")}
                      </option>
                    ))}
                  </>
                )}
              </Select>
              {isBuy && (partnerPublishers ?? []).length === 0 && (
                <p className="mt-1 text-xs text-gray-500">
                  Link a publisher partner under Partnerships before creating a buy contract.
                </p>
              )}
              {counterpartyIsPublisher && (
                <p className="mt-1 text-xs text-gray-500">
                  A mirrored {form.contract_type === "sell" ? "buy" : "sell"} contract will be created on the other publisher.
                </p>
              )}
            </div>
            <div>
              <Label>Name</Label>
              <Input value={form.name} onChange={(e) => set("name", e.target.value)} />
            </div>
            <div>
              <Label>Lead type</Label>
              <Select value={form.lead_type} onChange={(e) => set("lead_type", e.target.value)}>
                <option value="">Select…</option>
                {CONTRACT_LEAD_TYPES.map((t) => (
                  <option key={t.value} value={t.value}>
                    {t.label}
                  </option>
                ))}
              </Select>
            </div>
            <div>
              <Label>Description</Label>
              <Textarea value={form.description} onChange={(e) => set("description", e.target.value)} />
            </div>
          </div>
        </div>

        <div>
          <SectionLabel className="mb-2">Compensation</SectionLabel>
          <CreateContractCompensationList
            buyerId={form.buyer_id}
            items={compensations}
            onChange={setCompensations}
          />
        </div>

        <ContractLeadCriteriaSection
          buyerId={form.buyer_id}
          buyerPipelineId={primary?.counterparty_pipeline_id ?? 0}
          value={leadCriteria}
          onChange={setLeadCriteria}
        />
      </div>
    </FormDrawer>
  );
}
