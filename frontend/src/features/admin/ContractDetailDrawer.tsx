import { useCallback, useEffect, useLayoutEffect, useRef, useState } from "react";
import { Sheet, DrawerHeader, DrawerBody, DrawerFooter } from "@/components/ui/dialog";
import { ContractMessageButton } from "@/features/messaging/MessageButton";
import { Button } from "@/components/ui/button";
import { Input, Label, Select, Textarea } from "@/components/ui/input";
import { toast } from "@/store/toastStore";
import { errorMessage } from "@/lib/api";
import { formatMoney } from "@/lib/utils";
import {
  useUpdateContract,
  useBuyers,
  usePartnerPublishers,
  useContractCompensations,
  useContractLeadCriteria,
  useSaveContractLeadCriteria,
  useSaveContractDelivery,
  useSaveContractDraft,
  useActivateContract,
  useLinkPublisherPartnership,
  useUpdateContractOffer,
  useContractDetail,
  useReturnRules,
} from "@/features/admin/hooks";
import { ContractOfferSection } from "@/features/admin/ContractOfferSection";
import { ContractParticipationsSection } from "@/features/admin/ContractParticipationsSection";
import { CallSettingsSection } from "@/features/calls/CallSettingsSection";
import { AppointmentSettingsSection } from "@/features/appointments/AppointmentSettingsSection";
import { PublisherContractCalendarSection } from "@/features/appointments/PublisherContractCalendarSection";
import { offerFromContractModes, type ContractOfferDraft } from "@/features/admin/contractOffer";
import {
  ContractLeadCriteriaSection,
  emptyLeadCriteria,
} from "@/features/admin/ContractLeadCriteriaSection";
import { CONTRACT_STATUSES, ContractStatusBadge } from "@/features/admin/contractStatus";
import { CONTRACT_LEAD_TYPES, isContractLeadType } from "@/features/admin/contractLeadType";
import {
  CONTRACT_TYPES,
  counterpartyLabel,
  formatBuyerWithType,
  formatContractType,
} from "@/features/admin/contractType";
import { ContractCompensationEditor } from "@/features/admin/ContractCompensationEditor";
import { ContractDeliverySection } from "@/features/admin/ContractDeliverySection";
import { ContractPublisherPipelineSection } from "@/features/admin/ContractPublisherPipelineSection";
import {
  CreateContractCompensationList,
  compensationDraftFromComp,
  emptyCompensationDraft,
  type CompensationDraft,
} from "@/features/admin/CreateContractCompensationList";
import {
  deliveryDraftFromContract,
  deliveryDraftToBody,
  deliveryDraftValid,
  openOfferPipelineRequired,
  type ContractDeliveryDraft,
} from "@/features/admin/contractCompensation";
import { openOfferDeliveryComplete } from "@/features/admin/contractSectionCompleteness";
import { ContractFormTabs } from "@/features/admin/ContractFormTabs";
import {
  allRequiredSectionsComplete,
  isOpenSellOffer,
} from "@/features/admin/contractSectionCompleteness";
import { buildContractPayload } from "@/features/admin/contractDraftPayload";
import { PublisherContractReturnRoutesSection } from "@/features/admin/PublisherContractReturnRoutesSection";
import type { Contract, ContractLeadCriteria } from "@/types";

function leadCriteriaForCompare(c: ContractLeadCriteria) {
  return {
    required_fields: c.required_fields ?? [],
    filter_rules: c.filter_rules ?? [],
    quality_rules: c.quality_rules ?? [],
  };
}

function leadCriteriaEqual(a: ContractLeadCriteria, b: ContractLeadCriteria) {
  return JSON.stringify(leadCriteriaForCompare(a)) === JSON.stringify(leadCriteriaForCompare(b));
}

export function ContractDetailDrawer({
  contract,
  onClose,
}: {
  contract: Contract | null;
  onClose: () => void;
}) {
  const { data: detail } = useContractDetail(contract?.id ?? null);
  const resolved = detail ?? contract;
  const closeRef = useRef(onClose);
  closeRef.current = onClose;

  return (
    <Sheet open={!!contract} onClose={() => closeRef.current()} width={720}>
      {resolved && (
        <DrawerContent
          contract={resolved}
          onClose={onClose}
          registerClose={(fn) => {
            closeRef.current = fn;
          }}
        />
      )}
    </Sheet>
  );
}

function DrawerContent({
  contract,
  onClose,
  registerClose,
}: {
  contract: Contract;
  onClose: () => void;
  registerClose: (fn: () => void) => void;
}) {
  useEffect(() => {
    if (contract.status !== "draft") {
      registerClose(onClose);
    }
  }, [contract.status, contract.id, onClose, registerClose]);

  if (contract.status === "draft") {
    return (
      <DraftDrawerContent contract={contract} onClose={onClose} registerClose={registerClose} />
    );
  }
  return <ActiveDrawerContent contract={contract} onClose={onClose} />;
}

function DraftDrawerContent({
  contract,
  onClose,
  registerClose,
}: {
  contract: Contract;
  onClose: () => void;
  registerClose: (fn: () => void) => void;
}) {
  const { data: buyers } = useBuyers();
  const { data: partnerPublishers } = usePartnerPublishers();
  const saveDraft = useSaveContractDraft();
  const activate = useActivateContract();
  const linkPub = useLinkPublisherPartnership();
  const {
    data: detail,
    isLoading: detailLoading,
    isFetching: detailFetching,
  } = useContractDetail(contract.id);
  const resolved = detail ?? contract;
  const {
    data: compensations,
    isLoading: compsLoading,
    isFetching: compsFetching,
  } = useContractCompensations(contract.id);
  const {
    data: leadCriteriaData,
    isLoading: leadCriteriaLoading,
    isFetching: leadCriteriaFetching,
  } = useContractLeadCriteria(contract.id);

  const [form, setForm] = useState({
    contract_type: (contract.contract_type ?? "sell") as "buy" | "sell",
    buyer_id: contract.buyer_id ?? 0,
    name: contract.name,
    lead_type: contract.lead_type ?? "",
    description: contract.description ?? "",
  });
  const [compDrafts, setCompDrafts] = useState<CompensationDraft[]>([emptyCompensationDraft()]);
  const [deliveryDraft, setDeliveryDraft] = useState<ContractDeliveryDraft>(() =>
    deliveryDraftFromContract(contract)
  );
  const [leadCriteria, setLeadCriteria] = useState(emptyLeadCriteria());
  const [offerDraft, setOfferDraft] = useState<ContractOfferDraft>(() =>
    offerFromContractModes(contract.allowed_delivery_modes, contract.distribution_strategy)
  );

  const lastSavedSnapshot = useRef<string | null>(null);
  const initialSyncDone = useRef(false);

  useEffect(() => {
    initialSyncDone.current = false;
    lastSavedSnapshot.current = null;
  }, [contract.id]);

  useEffect(() => {
    if (initialSyncDone.current) return;
    if (
      compsLoading ||
      compsFetching ||
      leadCriteriaLoading ||
      leadCriteriaFetching ||
      detailLoading ||
      detailFetching
    ) {
      return;
    }

    const formState = {
      contract_type: (resolved.contract_type ?? "sell") as "buy" | "sell",
      buyer_id: resolved.buyer_id ?? 0,
      name: resolved.name,
      lead_type: resolved.lead_type ?? "",
      description: resolved.description ?? "",
    };
    const comps =
      (compensations ?? []).length > 0
        ? compensations!.map((c) => compensationDraftFromComp(c, resolved.rate_per_lead))
        : [emptyCompensationDraft()];
    const compDelivery = compensations?.[0]?.delivery;
    const delivery = deliveryDraftFromContract(resolved, compDelivery);
    const criteria = leadCriteriaData ?? emptyLeadCriteria();
    const offer = offerFromContractModes(
      resolved.allowed_delivery_modes,
      resolved.distribution_strategy
    );

    setForm(formState);
    setCompDrafts(comps);
    setDeliveryDraft(delivery);
    setLeadCriteria(criteria);
    setOfferDraft(offer);
    lastSavedSnapshot.current = JSON.stringify(
      buildContractPayload({
        status: "draft",
        form: formState,
        compensations: comps,
        delivery,
        leadCriteria: criteria,
        offer,
      })
    );
    initialSyncDone.current = true;
  }, [
    contract.id,
    resolved,
    compensations,
    compsLoading,
    compsFetching,
    leadCriteriaData,
    leadCriteriaLoading,
    leadCriteriaFetching,
    detailLoading,
    detailFetching,
  ]);

  const counterpartyIsPublisher = (partnerPublishers ?? []).some((p) => p.id === form.buyer_id);
  const isBuy = form.contract_type === "buy";
  const openOffer = isOpenSellOffer(form);
  const { data: returnRules } = useReturnRules(openOffer ? null : contract.id, false);
  const returnRulesCount = openOffer ? undefined : (returnRules?.length ?? 0);
  const canSaveDraft = !!form.name.trim();
  const canActivate = allRequiredSectionsComplete({
    form,
    compensations: compDrafts,
    delivery: deliveryDraft,
    leadCriteria,
    offer: offerDraft,
    returnRulesCount,
  });

  function payload(status: "draft" | "active") {
    return buildContractPayload({
      status,
      form,
      compensations: compDrafts,
      delivery: deliveryDraft,
      leadCriteria,
      offer: offerDraft,
    });
  }

  function draftSnapshot() {
    return JSON.stringify(payload("draft"));
  }

  function isDirty() {
    return lastSavedSnapshot.current !== null && draftSnapshot() !== lastSavedSnapshot.current;
  }

  const handleClose = useCallback(async () => {
    if (canSaveDraft && isDirty()) {
      try {
        await saveDraft.mutateAsync({ contractId: contract.id, body: payload("draft") });
        lastSavedSnapshot.current = draftSnapshot();
      } catch (e) {
        toast.error(errorMessage(e));
        return;
      }
    }
    onClose();
  }, [canSaveDraft, contract.id, form, compDrafts, deliveryDraft, leadCriteria, offerDraft, onClose, saveDraft]);

  useLayoutEffect(() => {
    registerClose(handleClose);
  }, [handleClose, registerClose]);

  function runSave(status: "draft" | "active", onSuccess: () => void) {
    const body = payload(status);
    const mutate = status === "draft" ? saveDraft : activate;
    const run = () =>
      mutate.mutate(
        { contractId: contract.id, body },
        {
          onSuccess: () => {
            if (status === "draft") {
              lastSavedSnapshot.current = draftSnapshot();
              toast.success("Draft saved");
            } else {
              toast.success("Contract activated");
            }
            onSuccess();
          },
          onError: (e) => toast.error(errorMessage(e)),
        }
      );

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
    <div className="flex h-full flex-col">
      <DrawerHeader
        title={contract.name}
        subtitle={`Draft · ${formatContractType(contract.contract_type)}`}
        onClose={handleClose}
      />

      <DrawerBody>
        <div className="mb-4 flex items-center gap-2">
          <ContractStatusBadge status="draft" />
          <span className="text-xs text-gray-500">Complete all required tabs to activate.</span>
        </div>

        <ContractFormTabs
          form={form}
          compensations={compDrafts}
          delivery={deliveryDraft}
          leadCriteria={leadCriteria}
          offer={offerDraft}
          returnRulesCount={returnRulesCount}
          panels={{
            details: (
              <div className="space-y-3">
                <div>
                  <Label>Contract type</Label>
                  <Select
                    value={form.contract_type}
                    onChange={(e) =>
                      setForm((f) => ({ ...f, contract_type: e.target.value as "buy" | "sell" }))
                    }
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
                  <Select
                    value={form.buyer_id}
                    onChange={(e) => setForm((f) => ({ ...f, buyer_id: Number(e.target.value) }))}
                  >
                    <option value={0}>{isBuy ? "Select…" : "Open offer (no buyer yet)"}</option>
                    {isBuy
                      ? (partnerPublishers ?? []).map((p) => (
                          <option key={p.id} value={p.id}>
                            {formatBuyerWithType(p.name, "publisher")}
                          </option>
                        ))
                      : (
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
                </div>
                <div>
                  <Label>Name</Label>
                  <Input
                    value={form.name}
                    onChange={(e) => setForm((f) => ({ ...f, name: e.target.value }))}
                  />
                </div>
                <div>
                  <Label>Lead type</Label>
                  <Select
                    value={form.lead_type}
                    onChange={(e) => setForm((f) => ({ ...f, lead_type: e.target.value }))}
                  >
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
                  <Textarea
                    value={form.description}
                    onChange={(e) => setForm((f) => ({ ...f, description: e.target.value }))}
                  />
                </div>
              </div>
            ),
            compensation: compsLoading ? (
              <p className="text-sm text-gray-400">Loading…</p>
            ) : (
              <CreateContractCompensationList
                buyerId={form.buyer_id}
                contractType={form.contract_type}
                buyerPipelineId={contract.buyer_pipeline_id ?? 0}
                leadType={form.lead_type}
                items={compDrafts}
                onChange={setCompDrafts}
              />
            ),
            delivery: (
              <div className="space-y-4">
                {openOffer ? (
                  <div className="space-y-4">
                    <ContractOfferSection value={offerDraft} onChange={setOfferDraft} />
                    {openOfferPipelineRequired(offerDraft.allowed_delivery_modes) && (
                      <ContractPublisherPipelineSection value={deliveryDraft} onChange={setDeliveryDraft} />
                    )}
                  </div>
                ) : (
                  <ContractDeliverySection
                    value={deliveryDraft}
                    onChange={setDeliveryDraft}
                  />
                )}
                {form.lead_type === "Call" && <CallSettingsSection contractId={contract.id} />}
                {form.lead_type === "Appointment" && (
                  <>
                    <PublisherContractCalendarSection
                      contractId={contract.id}
                      publisherAppointmentCalendarId={contract.publisher_appointment_calendar_id}
                    />
                    {contract.status === "active" && (contract.appointment_calendar_id ?? 0) > 0 && (
                      <AppointmentSettingsSection contractId={contract.id} />
                    )}
                  </>
                )}
              </div>
            ),
            criteria: (
              <ContractLeadCriteriaSection
                buyerId={form.buyer_id}
                buyerPipelineId={contract.buyer_pipeline_id ?? 0}
                contractType={form.contract_type}
                value={leadCriteria}
                onChange={setLeadCriteria}
              />
            ),
            returns: (
              <PublisherContractReturnRoutesSection
                contractId={contract.id}
                delivery={deliveryDraft}
                openOffer={openOffer}
              />
            ),
          }}
        />
      </DrawerBody>

      <DrawerFooter className="flex justify-end gap-2">
        <Button
          variant="secondary"
          className="min-w-[10.5rem]"
          disabled={!canSaveDraft || saveDraft.isPending}
          onClick={() => runSave("draft", () => {})}
        >
          Save draft
        </Button>
        <Button
          className="min-w-[10.5rem]"
          disabled={!canActivate || activate.isPending}
          onClick={() => runSave("active", onClose)}
        >
          Activate contract
        </Button>
      </DrawerFooter>
    </div>
  );
}

function ActiveDrawerContent({ contract, onClose }: { contract: Contract; onClose: () => void }) {
  const isOpenOffer = !contract.buyer_id && (contract.contract_type ?? "sell") === "sell";
  const update = useUpdateContract();
  const saveDelivery = useSaveContractDelivery();
  const saveOffer = useUpdateContractOffer();
  const { data: compensations, isLoading: compsLoading } = useContractCompensations(contract.id);
  const { data: returnRules } = useReturnRules(isOpenOffer ? null : contract.id, false);
  const returnRulesCount = isOpenOffer ? undefined : (returnRules?.length ?? 0);

  const [name, setName] = useState(contract.name);
  const [leadType, setLeadType] = useState(contract.lead_type ?? "");
  const [description, setDescription] = useState(contract.description ?? "");
  const [status, setStatus] = useState(contract.status);
  const [deliveryDraft, setDeliveryDraft] = useState<ContractDeliveryDraft>(() =>
    deliveryDraftFromContract(contract)
  );
  const { data: leadCriteriaData } = useContractLeadCriteria(contract.id);
  const saveCriteria = useSaveContractLeadCriteria();
  const [leadCriteria, setLeadCriteria] = useState(emptyLeadCriteria());
  const [offerDraft, setOfferDraft] = useState<ContractOfferDraft>(() =>
    offerFromContractModes(contract.allowed_delivery_modes, contract.distribution_strategy)
  );

  useEffect(() => {
    if (leadCriteriaData) setLeadCriteria(leadCriteriaData);
  }, [leadCriteriaData]);

  useEffect(() => {
    setName(contract.name);
    setLeadType(contract.lead_type ?? "");
    setDescription(contract.description ?? "");
    setStatus(contract.status);
  }, [contract.id, contract.name, contract.lead_type, contract.description, contract.status]);

  const serverDeliveryKey = [
    contract.id,
    contract.source_pipeline_id ?? 0,
    contract.source_stage_id ?? 0,
    JSON.stringify(contract.allowed_delivery_modes ?? []),
    contract.distribution_strategy ?? "",
  ].join("|");

  useEffect(() => {
    setDeliveryDraft(deliveryDraftFromContract(contract));
    setOfferDraft(offerFromContractModes(contract.allowed_delivery_modes, contract.distribution_strategy));
  }, [serverDeliveryKey]);

  const savedOffer = offerFromContractModes(contract.allowed_delivery_modes, contract.distribution_strategy);
  const savedDelivery = deliveryDraftFromContract(contract, compensations?.[0]?.delivery);
  const offerUnchanged =
    JSON.stringify(offerDraft.allowed_delivery_modes) === JSON.stringify(savedOffer.allowed_delivery_modes) &&
    offerDraft.distribution_strategy === savedOffer.distribution_strategy &&
    (!openOfferPipelineRequired(offerDraft.allowed_delivery_modes) ||
      (deliveryDraft.source_pipeline_id === savedDelivery.source_pipeline_id &&
        deliveryDraft.source_stage_id === savedDelivery.source_stage_id));

  const deliveryUnchanged =
    deliveryDraft.delivery === savedDelivery.delivery &&
    deliveryDraft.source_pipeline_id === savedDelivery.source_pipeline_id &&
    deliveryDraft.source_stage_id === savedDelivery.source_stage_id;

  const unchanged =
    name.trim() === contract.name &&
    leadType === (contract.lead_type ?? "") &&
    description === (contract.description ?? "") &&
    status === contract.status;
  const invalid = !name.trim() || !isContractLeadType(leadType);
  const offerDirty = !offerUnchanged;
  const offerSaveable =
    isOpenOffer && offerDirty && openOfferDeliveryComplete(offerDraft, deliveryDraft);
  const offerBlocked = isOpenOffer && offerDirty && !openOfferDeliveryComplete(offerDraft, deliveryDraft);
  const savedCriteria = leadCriteriaData ?? emptyLeadCriteria();
  const criteriaDirty = !leadCriteriaEqual(leadCriteria, savedCriteria);
  const footerDirty = !unchanged || offerSaveable || criteriaDirty;
  const activeStatuses = CONTRACT_STATUSES.filter((s) => s.value !== "draft");

  function offerSaveBody() {
    return {
      allowed_delivery_modes: offerDraft.allowed_delivery_modes,
      distribution_strategy: offerDraft.distribution_strategy,
      source_pipeline_id: deliveryDraft.source_pipeline_id || null,
      source_stage_id: deliveryDraft.source_stage_id || null,
    };
  }

  function saveOfferSettings(onSuccess?: () => void) {
    saveOffer.mutate(
      { contractId: contract.id, body: offerSaveBody() },
      {
        onSuccess: () => {
          toast.success("Offer settings saved");
          onSuccess?.();
        },
        onError: (e) => toast.error(errorMessage(e)),
      }
    );
  }

  function saveLeadCriteriaSettings(onSuccess?: () => void) {
    saveCriteria.mutate(
      { contractId: contract.id, body: leadCriteria },
      {
        onSuccess: () => {
          toast.success("Lead criteria saved");
          onSuccess?.();
        },
        onError: (e) => toast.error(errorMessage(e)),
      }
    );
  }

  function saveDetails() {
    const body: Record<string, unknown> = {};
    const trimmed = name.trim();
    if (trimmed !== contract.name) body.name = trimmed;
    if (leadType !== (contract.lead_type ?? "")) body.lead_type = leadType;
    if (description !== (contract.description ?? "")) body.description = description;
    if (status !== contract.status) body.status = status;
    const hasDetails = Object.keys(body).length > 0;

    if (!hasDetails && !criteriaDirty && !offerSaveable) return;

    void (async () => {
      try {
        if (hasDetails) await update.mutateAsync({ id: contract.id, body });
        if (criteriaDirty) await saveCriteria.mutateAsync({ contractId: contract.id, body: leadCriteria });
        if (offerSaveable) await saveOffer.mutateAsync({ contractId: contract.id, body: offerSaveBody() });
        toast.success("Contract saved");
      } catch (e) {
        toast.error(errorMessage(e));
      }
    })();
  }

  function setContractStatus(next: string, successMessage: string) {
    update.mutate(
      { id: contract.id, body: { status: next } },
      {
        onSuccess: () => {
          setStatus(next);
          toast.success(successMessage);
        },
        onError: (e) => toast.error(errorMessage(e)),
      }
    );
  }

  const primaryRate =
    compensations?.find((c) => c.kind === "flat_rate")?.flat_amount ?? contract.rate_per_lead;

  return (
    <div className="flex h-full flex-col">
      <DrawerHeader
        title={contract.name}
        subtitle={`${formatContractType(contract.contract_type)} · ${formatBuyerWithType(contract.buyer_name, contract.buyer_account_type) || (isOpenOffer ? "Open offer" : contract.buyer_id ? `Buyer #${contract.buyer_id}` : "—")} · ${formatMoney(primaryRate)}/lead · ${contract.lead_count ?? 0} distributed`}
        onClose={onClose}
      />

      <DrawerBody>
        {contract.buyer_id != null && (
          <div className="mb-4">
            <ContractMessageButton contractId={contract.public_id} />
          </div>
        )}
        <div className="mb-4 flex items-center justify-between rounded-lg border border-gray-100 bg-gray-50 px-3 py-2.5">
          <span className="text-sm text-gray-400">Contract ID</span>
          <div className="flex items-center gap-2">
            <code className="text-sm font-semibold text-gray-800">{contract.handler_id}</code>
            <Button
              variant="secondary"
              className="h-7 px-2 text-xs"
              onClick={() => {
                void navigator.clipboard
                  .writeText(contract.handler_id)
                  .then(() => toast.success("Copied Contract ID"));
              }}
            >
              Copy
            </Button>
          </div>
        </div>
        {contract.mirror_contract_id != null && (
          <p className="mb-3 text-xs text-gray-500">
            Mirrored with publisher contract #{contract.mirror_contract_id}. Edits on shared fields sync on save from the sell side.
          </p>
        )}

        <ContractFormTabs
          showCheckmarks={false}
          form={{
            contract_type: contract.contract_type ?? "sell",
            buyer_id: contract.buyer_id ?? 0,
            name,
            lead_type: leadType,
          }}
          compensations={[emptyCompensationDraft()]}
          delivery={deliveryDraft}
          leadCriteria={leadCriteria}
          returnRulesCount={returnRulesCount}
          panels={{
            details: (
              <div className="flex flex-col gap-2.5">
                <div>
                  <Label>Contract type</Label>
                  <div className="mt-1 text-sm text-gray-700">{formatContractType(contract.contract_type) || "Sell"}</div>
                </div>
                <div>
                  <Label>{counterpartyLabel(contract.contract_type)}</Label>
                  <div className="mt-1 text-sm text-gray-700">
                    {formatBuyerWithType(contract.buyer_name, contract.buyer_account_type) ||
                      (contract.buyer_id ? `#${contract.buyer_id}` : "—")}
                  </div>
                </div>
                <div>
                  <Label>Name</Label>
                  <Input value={name} onChange={(e) => setName(e.target.value)} />
                </div>
                <div>
                  <Label>Lead type</Label>
                  <Select value={leadType} onChange={(e) => setLeadType(e.target.value)}>
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
                  <Textarea value={description} onChange={(e) => setDescription(e.target.value)} />
                </div>
                <div>
                  <Label>Status</Label>
                  <Select value={status} onChange={(e) => setStatus(e.target.value)}>
                    {activeStatuses.map((s) => (
                      <option key={s.value} value={s.value}>
                        {s.label}
                      </option>
                    ))}
                  </Select>
                </div>
              </div>
            ),
            compensation: (
              <>
                <p className="mb-2 text-xs text-gray-400">Each row has its own cap limits and payout settings.</p>
                {compsLoading ? (
                  <p className="text-sm text-gray-400">Loading…</p>
                ) : (
                  <ContractCompensationEditor
                    contract={contract}
                    items={compensations ?? []}
                    deliveryDraft={deliveryDraft}
                  />
                )}
              </>
            ),
            delivery: (
              <>
                {isOpenOffer ? (
                  <>
                    <ContractOfferSection value={offerDraft} onChange={setOfferDraft} />
                    {openOfferPipelineRequired(offerDraft.allowed_delivery_modes) && (
                      <div className="mt-4">
                        <ContractPublisherPipelineSection value={deliveryDraft} onChange={setDeliveryDraft} />
                        {offerBlocked && (
                          <p className="mt-2 text-xs text-gray-500">
                            Select source pipeline, distribute stage, and return stage to save.
                          </p>
                        )}
                      </div>
                    )}
                    <Button
                      className="mt-3"
                      variant="secondary"
                      disabled={offerUnchanged || offerBlocked || saveOffer.isPending}
                      onClick={() => saveOfferSettings()}
                    >
                      Save offer settings
                    </Button>
                  </>
                ) : (
                  <>
                    <ContractDeliverySection
                      value={deliveryDraft}
                      onChange={setDeliveryDraft}
                    />
                    <Button
                      className="mt-3"
                      variant="secondary"
                      disabled={
                        deliveryUnchanged ||
                        !deliveryDraftValid(deliveryDraft) ||
                        saveDelivery.isPending
                      }
                      onClick={() =>
                        saveDelivery.mutate(
                          { contractId: contract.id, body: deliveryDraftToBody(deliveryDraft) },
                          {
                            onSuccess: () => toast.success("Delivery saved"),
                            onError: (e) => toast.error(errorMessage(e)),
                          }
                        )
                      }
                    >
                      Save delivery
                    </Button>
                  </>
                )}
                {leadType === "Call" && (
                  <div className="mt-4">
                    <CallSettingsSection contractId={contract.id} />
                  </div>
                )}
                {leadType === "Appointment" && (
                  <div className="mt-4">
                    <PublisherContractCalendarSection
                      contractId={contract.id}
                      publisherAppointmentCalendarId={contract.publisher_appointment_calendar_id}
                    />
                    {contract.status === "active" && (contract.appointment_calendar_id ?? 0) > 0 && (
                      <AppointmentSettingsSection contractId={contract.id} />
                    )}
                  </div>
                )}
              </>
            ),
            ...(isOpenOffer
              ? {
                  buyers: <ContractParticipationsSection contract={contract} />,
                }
              : {}),
            criteria: (
              <>
                <ContractLeadCriteriaSection
                  buyerId={contract.buyer_id ?? 0}
                  buyerPipelineId={contract.buyer_pipeline_id ?? 0}
                  contractType={contract.contract_type ?? "sell"}
                  value={leadCriteria}
                  onChange={setLeadCriteria}
                />
                <Button
                  className="mt-3"
                  variant="secondary"
                  disabled={!criteriaDirty || saveCriteria.isPending}
                  onClick={() => saveLeadCriteriaSettings()}
                >
                  Save lead criteria
                </Button>
              </>
            ),
            returns: (
              <PublisherContractReturnRoutesSection
                contractId={contract.id}
                delivery={deliveryDraft}
                openOffer={isOpenOffer}
              />
            ),
          }}
          extraTabs={isOpenOffer ? [{ id: "buyers", label: "Buyers" }] : undefined}
        />
      </DrawerBody>

      <DrawerFooter className="flex justify-end gap-2">
        {status === "active" && (
          <Button
            variant="secondary"
            className="min-w-[10.5rem]"
            disabled={update.isPending}
            onClick={() => setContractStatus("paused", "Contract paused")}
          >
            Pause contract
          </Button>
        )}
        {status === "paused" && (
          <Button
            variant="secondary"
            className="min-w-[10.5rem]"
            disabled={update.isPending}
            onClick={() => setContractStatus("active", "Contract resumed")}
          >
            Resume contract
          </Button>
        )}
        <Button
          className="min-w-[10.5rem]"
          disabled={
            !footerDirty ||
            offerBlocked ||
            invalid ||
            update.isPending ||
            saveOffer.isPending ||
            saveCriteria.isPending
          }
          onClick={saveDetails}
        >
          Save details
        </Button>
      </DrawerFooter>
    </div>
  );
}
