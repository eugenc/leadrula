import { useCallback, useEffect, useLayoutEffect, useRef, useState } from "react";
import { Sheet, DrawerHeader, DrawerBody, DrawerFooter } from "@/components/ui/dialog";
import { ContractMessageButton } from "@/features/messaging/MessageButton";
import { Button } from "@/components/ui/button";
import { Input, Label, Select, Textarea } from "@/components/ui/input";
import { SectionLabel } from "@/components/layout/SectionLabel";
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
import {
  PublisherContractCalendarSection,
  type BookingSectionSave,
} from "@/features/appointments/PublisherContractCalendarSection";
import { offerFromContractModes, type ContractOfferDraft } from "@/features/admin/contractOffer";
import {
  ContractLeadFieldsSection,
  ContractLeadFilterRulesSection,
  ContractLeadCriteriaSaveButton,
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

async function flushBookingSections(
  calendarSave: BookingSectionSave | null,
  slotsSave: BookingSectionSave | null
): Promise<boolean> {
  if (calendarSave?.isDirty()) {
    const ok = await calendarSave.flush();
    if (!ok) return false;
  }
  if (slotsSave?.isDirty()) {
    const ok = await slotsSave.flush();
    if (!ok) return false;
  }
  return true;
}

export function ContractDetailDrawer({
  contract,
  onClose,
  registerFlushHandler,
}: {
  contract: Contract | null;
  onClose: () => void;
  registerFlushHandler?: (fn: (() => Promise<boolean>) | null) => void;
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
            closeRef.current = fn ?? onClose;
          }}
          registerFlushHandler={registerFlushHandler}
        />
      )}
    </Sheet>
  );
}

function DrawerContent({
  contract,
  onClose,
  registerClose,
  registerFlushHandler,
}: {
  contract: Contract;
  onClose: () => void;
  registerClose: (fn: (() => void) | null) => void;
  registerFlushHandler?: (fn: (() => Promise<boolean>) | null) => void;
}) {
  if (contract.status === "draft") {
    return (
      <DraftDrawerContent
        contract={contract}
        onClose={onClose}
        registerClose={registerClose}
        registerFlushHandler={registerFlushHandler}
      />
    );
  }
  return (
    <ActiveDrawerContent
      contract={contract}
      onClose={onClose}
      registerClose={registerClose}
      registerFlushHandler={registerFlushHandler}
    />
  );
}

function DraftDrawerContent({
  contract,
  onClose,
  registerClose,
  registerFlushHandler,
}: {
  contract: Contract;
  onClose: () => void;
  registerClose: (fn: (() => void) | null) => void;
  registerFlushHandler?: (fn: (() => Promise<boolean>) | null) => void;
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
  const calendarSaveRef = useRef<BookingSectionSave | null>(null);

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

  const flushDraft = useCallback(async (): Promise<boolean> => {
    const draftDirty = canSaveDraft && isDirty();
    const bookingDirty = calendarSaveRef.current?.isDirty() ?? false;
    if (!draftDirty && !bookingDirty) return true;

    const toastId = toast.progress("Saving…");
    try {
      if (draftDirty) {
        await saveDraft.mutateAsync({ contractId: contract.id, body: payload("draft") });
        lastSavedSnapshot.current = draftSnapshot();
      }
      const bookingOk = await flushBookingSections(calendarSaveRef.current, null);
      if (!bookingOk) {
        toast.dismiss(toastId);
        return false;
      }
      toast.update(toastId, "Saved");
      setTimeout(() => toast.dismiss(toastId), 1500);
      return true;
    } catch (e) {
      toast.dismiss(toastId);
      toast.error(errorMessage(e));
      return false;
    }
  }, [canSaveDraft, contract.id, form, compDrafts, deliveryDraft, leadCriteria, offerDraft, saveDraft]);

  const handleClose = useCallback(async () => {
    const ok = await flushDraft();
    if (ok) onClose();
  }, [flushDraft, onClose]);

  useLayoutEffect(() => {
    registerClose(() => {
      void handleClose();
    });
    registerFlushHandler?.(flushDraft);
    return () => {
      registerClose(null);
      registerFlushHandler?.(null);
    };
  }, [handleClose, flushDraft, registerClose, registerFlushHandler]);

  function runSave(status: "draft" | "active", onSuccess: () => void) {
    const body = payload(status);
    const mutate = status === "draft" ? saveDraft : activate;
    const run = () =>
      mutate.mutate(
        { contractId: contract.id, body },
        {
          onSuccess: () => {
            void (async () => {
              if (status === "draft") {
                lastSavedSnapshot.current = draftSnapshot();
                const bookingOk = await flushBookingSections(calendarSaveRef.current, null);
                if (!bookingOk) return;
                toast.success("Draft saved");
              } else {
                toast.success("Contract activated");
              }
              onSuccess();
            })();
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
                <div className="border-t border-gray-100 pt-4">
                  <SectionLabel>Return Routes</SectionLabel>
                  <PublisherContractReturnRoutesSection
                    contractId={contract.id}
                    delivery={deliveryDraft}
                    openOffer={openOffer}
                  />
                </div>
                {form.lead_type === "Call" && <CallSettingsSection contractId={contract.id} />}
              </div>
            ),
            ...(form.lead_type === "Appointment"
              ? {
                  booking: (
                    <>
                      <PublisherContractCalendarSection
                        standalone
                        contractId={contract.id}
                        publisherAppointmentCalendarId={contract.publisher_appointment_calendar_id}
                        registerSave={(api) => {
                          calendarSaveRef.current = api;
                        }}
                      />
                      {contract.status === "active" && (contract.appointment_calendar_id ?? 0) > 0 && (
                        <AppointmentSettingsSection contractId={contract.id} />
                      )}
                    </>
                  ),
                }
              : {}),
            fields: (
              <ContractLeadFieldsSection
                buyerId={form.buyer_id}
                buyerPipelineId={contract.buyer_pipeline_id ?? 0}
                contractType={form.contract_type}
                leadType={form.lead_type}
                value={leadCriteria}
                onChange={setLeadCriteria}
              />
            ),
            filters: (
              <ContractLeadFilterRulesSection
                value={leadCriteria}
                onChange={setLeadCriteria}
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

function ActiveDrawerContent({
  contract,
  onClose,
  registerClose,
  registerFlushHandler,
}: {
  contract: Contract;
  onClose: () => void;
  registerClose: (fn: (() => void) | null) => void;
  registerFlushHandler?: (fn: (() => Promise<boolean>) | null) => void;
}) {
  const isOpenOffer = !contract.buyer_id && (contract.contract_type ?? "sell") === "sell";
  const update = useUpdateContract();
  const saveDelivery = useSaveContractDelivery();
  const saveOffer = useUpdateContractOffer();
  const { data: compensations, isLoading: compsLoading } = useContractCompensations(contract.id);
  const { data: returnRules } = useReturnRules(isOpenOffer ? null : contract.id, false);
  const returnRulesCount = isOpenOffer ? undefined : (returnRules?.length ?? 0);
  const touchedRef = useRef(false);
  const closingRef = useRef(false);
  const calendarSaveRef = useRef<BookingSectionSave | null>(null);
  const slotsSaveRef = useRef<BookingSectionSave | null>(null);
  const [calendarDirty, setCalendarDirty] = useState(false);
  const [slotsDirty, setSlotsDirty] = useState(false);

  const [name, setNameState] = useState(contract.name);
  const [leadType, setLeadTypeState] = useState(contract.lead_type ?? "");
  const [description, setDescriptionState] = useState(contract.description ?? "");
  const [status, setStatusState] = useState(contract.status);
  const [deliveryDraft, setDeliveryDraftState] = useState<ContractDeliveryDraft>(() =>
    deliveryDraftFromContract(contract)
  );
  const { data: leadCriteriaData } = useContractLeadCriteria(contract.id);
  const saveCriteria = useSaveContractLeadCriteria();
  const [leadCriteria, setLeadCriteriaState] = useState(emptyLeadCriteria());
  const [offerDraft, setOfferDraftState] = useState<ContractOfferDraft>(() =>
    offerFromContractModes(contract.allowed_delivery_modes, contract.distribution_strategy)
  );

  function touch() {
    touchedRef.current = true;
  }
  const setName = (v: string) => {
    touch();
    setNameState(v);
  };
  const setLeadType = (v: string) => {
    touch();
    setLeadTypeState(v);
  };
  const setDescription = (v: string) => {
    touch();
    setDescriptionState(v);
  };
  const setStatus = (v: string) => {
    touch();
    setStatusState(v);
  };
  const setDeliveryDraft = (v: ContractDeliveryDraft | ((prev: ContractDeliveryDraft) => ContractDeliveryDraft)) => {
    touch();
    setDeliveryDraftState(v);
  };
  const setLeadCriteria = (v: ContractLeadCriteria | ((prev: ContractLeadCriteria) => ContractLeadCriteria)) => {
    touch();
    setLeadCriteriaState(v);
  };
  const setOfferDraft = (v: ContractOfferDraft | ((prev: ContractOfferDraft) => ContractOfferDraft)) => {
    touch();
    setOfferDraftState(v);
  };

  useEffect(() => {
    touchedRef.current = false;
    setNameState(contract.name);
    setLeadTypeState(contract.lead_type ?? "");
    setDescriptionState(contract.description ?? "");
    setStatusState(contract.status);
    setDeliveryDraftState(deliveryDraftFromContract(contract));
    setOfferDraftState(
      offerFromContractModes(contract.allowed_delivery_modes, contract.distribution_strategy)
    );
    if (leadCriteriaData) setLeadCriteriaState(leadCriteriaData);
  }, [contract.id]);

  useEffect(() => {
    if (touchedRef.current) return;
    if (leadCriteriaData) setLeadCriteriaState(leadCriteriaData);
  }, [leadCriteriaData]);

  useEffect(() => {
    if (touchedRef.current) return;
    setNameState(contract.name);
    setLeadTypeState(contract.lead_type ?? "");
    setDescriptionState(contract.description ?? "");
    setStatusState(contract.status);
  }, [contract.name, contract.lead_type, contract.description, contract.status]);

  const serverDeliveryKey = [
    contract.id,
    contract.source_pipeline_id ?? 0,
    contract.source_stage_id ?? 0,
    JSON.stringify(contract.allowed_delivery_modes ?? []),
    contract.distribution_strategy ?? "",
  ].join("|");

  useEffect(() => {
    if (touchedRef.current) return;
    setDeliveryDraftState(deliveryDraftFromContract(contract));
    setOfferDraftState(
      offerFromContractModes(contract.allowed_delivery_modes, contract.distribution_strategy)
    );
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
  const bookingDirty = calendarDirty || slotsDirty;
  const footerDirty = !unchanged || offerSaveable || criteriaDirty || bookingDirty;
  const activeStatuses = CONTRACT_STATUSES.filter((s) => s.value !== "draft");

  function offerSaveBodyFrom(draft: ContractOfferDraft, delivery: ContractDeliveryDraft) {
    return {
      allowed_delivery_modes: draft.allowed_delivery_modes,
      distribution_strategy: draft.distribution_strategy,
      source_pipeline_id: delivery.source_pipeline_id || null,
      source_stage_id: delivery.source_stage_id || null,
    };
  }

  const flushRef = useRef({
    contract,
    name,
    leadType,
    description,
    status,
    deliveryDraft,
    leadCriteria,
    offerDraft,
    leadCriteriaData,
    compensations,
    isOpenOffer,
  });
  flushRef.current = {
    contract,
    name,
    leadType,
    description,
    status,
    deliveryDraft,
    leadCriteria,
    offerDraft,
    leadCriteriaData,
    compensations,
    isOpenOffer,
  };

  const flushActiveContract = useCallback(async (): Promise<boolean> => {
    const s = flushRef.current;
    const c = s.contract;
    const trimmed = s.name.trim();
    const detailsUnchanged =
      trimmed === c.name &&
      s.leadType === (c.lead_type ?? "") &&
      s.description === (c.description ?? "") &&
      s.status === c.status;
    const savedCriteria = s.leadCriteriaData ?? emptyLeadCriteria();
    const criteriaDirty = !leadCriteriaEqual(s.leadCriteria, savedCriteria);
    const savedOffer = offerFromContractModes(c.allowed_delivery_modes, c.distribution_strategy);
    const savedDelivery = deliveryDraftFromContract(c, s.compensations?.[0]?.delivery);
    const offerUnchanged =
      JSON.stringify(s.offerDraft.allowed_delivery_modes) ===
        JSON.stringify(savedOffer.allowed_delivery_modes) &&
      s.offerDraft.distribution_strategy === savedOffer.distribution_strategy &&
      (!openOfferPipelineRequired(s.offerDraft.allowed_delivery_modes) ||
        (s.deliveryDraft.source_pipeline_id === savedDelivery.source_pipeline_id &&
          s.deliveryDraft.source_stage_id === savedDelivery.source_stage_id));
    const deliveryUnchanged =
      s.deliveryDraft.delivery === savedDelivery.delivery &&
      s.deliveryDraft.source_pipeline_id === savedDelivery.source_pipeline_id &&
      s.deliveryDraft.source_stage_id === savedDelivery.source_stage_id;
    const invalid = !trimmed || !isContractLeadType(s.leadType);
    const offerSaveable =
      s.isOpenOffer && !offerUnchanged && openOfferDeliveryComplete(s.offerDraft, s.deliveryDraft);
    const offerBlocked =
      s.isOpenOffer && !offerUnchanged && !openOfferDeliveryComplete(s.offerDraft, s.deliveryDraft);
    const deliveryDirty = !s.isOpenOffer && !deliveryUnchanged;
    const bookingDirty =
      (calendarSaveRef.current?.isDirty() ?? false) || (slotsSaveRef.current?.isDirty() ?? false);

    const hasAnyDirty =
      !detailsUnchanged ||
      criteriaDirty ||
      offerSaveable ||
      deliveryDirty ||
      offerBlocked ||
      bookingDirty;
    if (!hasAnyDirty) return true;

    if (invalid && !detailsUnchanged) {
      toast.error("Name and lead type are required.");
      return false;
    }
    if (offerBlocked) {
      toast.error("Complete offer pipeline settings before saving.");
      return false;
    }
    if (deliveryDirty && !deliveryDraftValid(s.deliveryDraft)) {
      toast.error("Complete distribution settings before saving.");
      return false;
    }

    const toastId = toast.progress("Saving…");
    try {
      const body: Record<string, unknown> = {};
      if (trimmed !== c.name) body.name = trimmed;
      if (s.leadType !== (c.lead_type ?? "")) body.lead_type = s.leadType;
      if (s.description !== (c.description ?? "")) body.description = s.description;
      if (s.status !== c.status) body.status = s.status;
      if (Object.keys(body).length > 0) {
        await update.mutateAsync({ id: c.id, body });
      }
      if (criteriaDirty) {
        await saveCriteria.mutateAsync({ contractId: c.id, body: s.leadCriteria });
      }
      if (offerSaveable) {
        await saveOffer.mutateAsync({
          contractId: c.id,
          body: offerSaveBodyFrom(s.offerDraft, s.deliveryDraft),
        });
      }
      if (deliveryDirty) {
        await saveDelivery.mutateAsync({
          contractId: c.id,
          body: deliveryDraftToBody(s.deliveryDraft),
        });
      }
      const bookingOk = await flushBookingSections(calendarSaveRef.current, slotsSaveRef.current);
      if (!bookingOk) {
        toast.dismiss(toastId);
        return false;
      }
      toast.update(toastId, "Saved");
      setTimeout(() => toast.dismiss(toastId), 1500);
      touchedRef.current = false;
      return true;
    } catch (e) {
      toast.dismiss(toastId);
      toast.error(errorMessage(e));
      return false;
    }
  }, [update, saveCriteria, saveOffer, saveDelivery]);

  const handleClose = useCallback(async () => {
    if (closingRef.current) return;
    closingRef.current = true;
    try {
      const ok = await flushActiveContract();
      if (ok) onClose();
    } finally {
      closingRef.current = false;
    }
  }, [flushActiveContract, onClose]);

  useLayoutEffect(() => {
    registerClose(() => {
      void handleClose();
    });
    registerFlushHandler?.(flushActiveContract);
    return () => {
      registerClose(null);
      registerFlushHandler?.(null);
    };
  }, [handleClose, flushActiveContract, registerClose, registerFlushHandler]);

  function offerSaveBody() {
    return offerSaveBodyFrom(offerDraft, deliveryDraft);
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

    if (!hasDetails && !criteriaDirty && !offerSaveable && !bookingDirty) return;

    void (async () => {
      try {
        if (hasDetails) await update.mutateAsync({ id: contract.id, body });
        if (criteriaDirty) await saveCriteria.mutateAsync({ contractId: contract.id, body: leadCriteria });
        if (offerSaveable) await saveOffer.mutateAsync({ contractId: contract.id, body: offerSaveBody() });
        const bookingOk = await flushBookingSections(calendarSaveRef.current, slotsSaveRef.current);
        if (!bookingOk) return;
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
          setStatusState(next);
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
        onClose={() => void handleClose()}
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
                            Select Distribute from Pipeline and Distribute from Stage to save.
                          </p>
                        )}
                      </div>
                    )}
                    <div className="mt-4 border-t border-gray-100 pt-4">
                      <SectionLabel>Return Routes</SectionLabel>
                      <PublisherContractReturnRoutesSection
                        contractId={contract.id}
                        delivery={deliveryDraft}
                        openOffer={isOpenOffer}
                      />
                    </div>
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
                    <div className="mt-4 border-t border-gray-100 pt-4">
                      <SectionLabel>Return Routes</SectionLabel>
                      <PublisherContractReturnRoutesSection
                        contractId={contract.id}
                        delivery={deliveryDraft}
                        openOffer={isOpenOffer}
                      />
                    </div>
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
                            onSuccess: () => toast.success("Distribution saved"),
                            onError: (e) => toast.error(errorMessage(e)),
                          }
                        )
                      }
                    >
                      Save distribution
                    </Button>
                  </>
                )}
                {leadType === "Call" && (
                  <div className="mt-4">
                    <CallSettingsSection contractId={contract.id} />
                  </div>
                )}
              </>
            ),
            ...(isOpenOffer
              ? {
                  buyers: <ContractParticipationsSection contract={contract} />,
                }
              : {}),
            ...(leadType === "Appointment"
              ? {
                  booking: (
                    <>
                      <PublisherContractCalendarSection
                        standalone
                        contractId={contract.id}
                        publisherAppointmentCalendarId={contract.publisher_appointment_calendar_id}
                        registerSave={(api) => {
                          calendarSaveRef.current = api;
                        }}
                        onDirtyChange={setCalendarDirty}
                      />
                      {contract.status === "active" && (contract.appointment_calendar_id ?? 0) > 0 && (
                        <AppointmentSettingsSection
                          contractId={contract.id}
                          registerSave={(api) => {
                            slotsSaveRef.current = api;
                          }}
                          onDirtyChange={setSlotsDirty}
                        />
                      )}
                    </>
                  ),
                }
              : {}),
            fields: (
              <>
                <ContractLeadFieldsSection
                  buyerId={contract.buyer_id ?? 0}
                  buyerPipelineId={contract.buyer_pipeline_id ?? 0}
                  contractType={contract.contract_type ?? "sell"}
                  leadType={leadType}
                  value={leadCriteria}
                  onChange={setLeadCriteria}
                />
                <ContractLeadCriteriaSaveButton
                  disabled={!criteriaDirty}
                  pending={saveCriteria.isPending}
                  onClick={() => saveLeadCriteriaSettings()}
                />
              </>
            ),
            filters: (
              <>
                <ContractLeadFilterRulesSection
                  value={leadCriteria}
                  onChange={setLeadCriteria}
                />
                <ContractLeadCriteriaSaveButton
                  disabled={!criteriaDirty}
                  pending={saveCriteria.isPending}
                  onClick={() => saveLeadCriteriaSettings()}
                />
              </>
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
