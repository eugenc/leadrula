import { useEffect, useState } from "react";
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
import { DeleteContractConfirmDialog } from "@/features/admin/DeleteContractConfirmDialog";
import { ContractStatusBadge } from "@/features/admin/contractStatus";
import { formatContractCap } from "@/features/admin/contractCap";
import {
  CreateContractCompensationList,
  blankCompensationDraft,
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
} from "@/features/admin/contractLeadType";
import {
  CONTRACT_TYPES,
  counterpartyLabel,
  formatBuyerWithType,
  formatContractType,
} from "@/features/admin/contractType";
import { ContractDeliverySection } from "@/features/admin/ContractDeliverySection";
import {
  emptyContractDelivery,
  type ContractDeliveryDraft,
} from "@/features/admin/contractCompensation";
import { ContractFormTabs } from "@/features/admin/ContractFormTabs";
import { allRequiredSectionsComplete } from "@/features/admin/contractSectionCompleteness";
import { buildContractPayload } from "@/features/admin/contractDraftPayload";
import {
  emptyContractOffer,
  type ContractOfferDraft,
} from "@/features/admin/contractOffer";
import { ContractOfferSection } from "@/features/admin/ContractOfferSection";
import { isOpenSellOffer } from "@/features/admin/contractSectionCompleteness";
import type { Contract } from "@/types";

export function ContractsPage() {
  const { data: contracts, isLoading, isError, error } = useContracts();
  const remove = useDeleteContract();
  const [open, setOpen] = useState(false);
  const [createSession, setCreateSession] = useState(0);
  const [selected, setSelected] = useState<Contract | null>(null);
  const [contractToDelete, setContractToDelete] = useState<Contract | null>(null);

  return (
    <>
      <PageHeader
        action={
          <Button
            onClick={() => {
              setCreateSession((n) => n + 1);
              setOpen(true);
            }}
          >
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
              <TH className="min-w-0 w-12" />
            </tr>
          </THead>
          <TBody>
            {(contracts ?? []).map((c) => (
              <TR key={c.id} onClick={() => setSelected(c)}>
                <TD>{formatContractType(c.contract_type) || "Sell"}</TD>
                <TD className="font-semibold">
                  {formatBuyerWithType(c.buyer_name, c.buyer_account_type) ||
                    (c.participations?.length ? `${c.participations.length} buyer(s)` : "Open offer")}
                </TD>
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
                        setContractToDelete(c);
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

      <CreateContractDrawer
        open={open}
        createSession={createSession}
        onClose={() => setOpen(false)}
        onDraftCreated={(c) => {
          setOpen(false);
          setSelected(c);
        }}
      />
      <ContractDetailDrawer contract={selected} onClose={() => setSelected(null)} />
      <DeleteContractConfirmDialog
        open={contractToDelete != null}
        onClose={() => setContractToDelete(null)}
        contractName={contractToDelete?.name ?? ""}
        buyerLabel={
          contractToDelete
            ? formatBuyerWithType(contractToDelete.buyer_name, contractToDelete.buyer_account_type) || undefined
            : undefined
        }
        loading={remove.isPending}
        onConfirm={() => {
          if (!contractToDelete) return;
          remove.mutate(contractToDelete.id, {
            onSuccess: () => {
              toast.success("Contract deleted");
              setContractToDelete(null);
              if (selected?.id === contractToDelete.id) setSelected(null);
            },
            onError: (err) => toast.error(errorMessage(err)),
          });
        }}
      />
      </PageBody>
    </>
  );
}

function initialCreateContractState() {
  return {
    form: {
      contract_type: "sell" as "buy" | "sell",
      buyer_id: 0,
      name: "",
      lead_type: "",
      description: "",
    },
    compensations: [blankCompensationDraft()] as CompensationDraft[],
    deliveryDraft: emptyContractDelivery(),
    leadCriteria: emptyLeadCriteria(),
    offerDraft: emptyContractOffer(),
  };
}

function CreateContractDrawer({
  open,
  createSession,
  onClose,
  onDraftCreated,
}: {
  open: boolean;
  createSession: number;
  onClose: () => void;
  onDraftCreated: (contract: Contract) => void;
}) {
  const { data: buyers } = useBuyers();
  const { data: partnerPublishers } = usePartnerPublishers();
  const create = useCreateContract();
  const linkPub = useLinkPublisherPartnership();
  const [form, setForm] = useState(initialCreateContractState().form);
  const [compensations, setCompensations] = useState<CompensationDraft[]>(
    initialCreateContractState().compensations
  );
  const [deliveryDraft, setDeliveryDraft] = useState<ContractDeliveryDraft>(
    initialCreateContractState().deliveryDraft
  );
  const [leadCriteria, setLeadCriteria] = useState<ContractLeadCriteria>(
    initialCreateContractState().leadCriteria
  );
  const [offerDraft, setOfferDraft] = useState<ContractOfferDraft>(initialCreateContractState().offerDraft);

  useEffect(() => {
    if (!open) return;
    const s = initialCreateContractState();
    setForm(s.form);
    setCompensations(s.compensations);
    setDeliveryDraft(s.deliveryDraft);
    setLeadCriteria(s.leadCriteria);
    setOfferDraft(s.offerDraft);
  }, [open, createSession]);

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

  const canSaveDraft = !!form.name.trim();
  const openOffer = isOpenSellOffer(form);
  const canCreate = allRequiredSectionsComplete({
    form,
    compensations,
    delivery: deliveryDraft,
    leadCriteria,
    offer: offerDraft,
  });

  function runCreate(status: "draft" | "active", onSuccess: () => void) {
    const body = buildContractPayload({
      status,
      form,
      compensations,
      delivery: deliveryDraft,
      leadCriteria,
      offer: offerDraft,
    });

    const run = () =>
      create.mutate(body, {
        onSuccess: (created) => {
          toast.success(status === "draft" ? "Draft saved" : "Contract created");
          if (status === "draft") {
            onDraftCreated(created);
          } else {
            onSuccess();
          }
        },
        onError: (e) => toast.error(errorMessage(e)),
      });

    if (status === "active" && counterpartyIsPublisher) {
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
          <Button
            variant="secondary"
            disabled={!canSaveDraft || create.isPending}
            onClick={() => runCreate("draft", onClose)}
          >
            Save draft
          </Button>
          <Button disabled={!canCreate || create.isPending} onClick={() => runCreate("active", onClose)}>
            Create
          </Button>
        </>
      }
    >
      <ContractFormTabs
        key={`new-contract-${createSession}`}
        resetKey={createSession}
        showCheckmarks={false}
        form={form}
        compensations={compensations}
        delivery={deliveryDraft}
        leadCriteria={leadCriteria}
        panels={{
          details: (
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
                <Label>
                  {counterpartyLabel(form.contract_type)}
                  {!isBuy ? " (optional — leave empty for open offer)" : ""}
                </Label>
                <Select value={form.buyer_id} onChange={(e) => set("buyer_id", Number(e.target.value))}>
                  <option value={0}>{isBuy ? "Select…" : "Open offer (no buyer yet)"}</option>
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
          ),
          compensation: (
            <CreateContractCompensationList
              buyerId={form.buyer_id}
              contractType={form.contract_type}
              buyerPipelineId={deliveryDraft.counterparty_pipeline_id}
              leadType={form.lead_type}
              items={compensations}
              onChange={setCompensations}
              blankNewRows
            />
          ),
          delivery: openOffer ? (
            <ContractOfferSection value={offerDraft} onChange={setOfferDraft} />
          ) : (
            <ContractDeliverySection
              buyerId={form.buyer_id}
              contractType={form.contract_type}
              value={deliveryDraft}
              onChange={setDeliveryDraft}
            />
          ),
          criteria: (
            <ContractLeadCriteriaSection
              buyerId={form.buyer_id}
              buyerPipelineId={deliveryDraft.counterparty_pipeline_id}
              contractType={form.contract_type}
              value={leadCriteria}
              onChange={setLeadCriteria}
            />
          ),
          returns: (
            <p className="text-sm text-gray-500">
              Return rules can be configured after the contract is saved.
            </p>
          ),
        }}
      />
    </FormDrawer>
  );
}
