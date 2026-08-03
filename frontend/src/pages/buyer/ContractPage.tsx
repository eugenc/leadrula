import { useMemo, useState } from "react";
import { useBuyerParticipations, useBuyerContracts } from "@/features/admin/hooks";
import { PageBody } from "@/components/layout/PageBody";
import { Table, THead, TH, TBody, TR, TD } from "@/components/ui/table";
import { Spinner, EmptyState } from "@/components/ui/misc";
import { errorMessage } from "@/lib/api";
import { formatMoney } from "@/lib/utils";
import { formatContractLeadType } from "@/features/admin/contractLeadType";
import { formatParticipationStatus, PUBLISHER_DELIVERY_MODES } from "@/features/admin/contractOffer";
import { formatContractStatus } from "@/features/admin/contractStatus";
import { BuyerParticipationAcceptDrawer } from "@/features/admin/BuyerParticipationAcceptDrawer";
import { BuyerParticipationDetailDrawer } from "@/features/admin/BuyerParticipationDetailDrawer";
import { BuyerContractDetailDrawer } from "@/features/admin/BuyerContractDetailDrawer";
import type { Contract, ContractParticipation } from "@/types";

function formatDeliveryMode(mode: string | undefined): string {
  if (!mode) return "—";
  return PUBLISHER_DELIVERY_MODES.find((m) => m.value === mode)?.label ?? mode;
}

export function ContractPage() {
  const { data: participations, isLoading: partsLoading, isError, error } = useBuyerParticipations();
  const { data: contracts, isLoading: contractsLoading } = useBuyerContracts();
  const [selectedInvite, setSelectedInvite] = useState<ContractParticipation | null>(null);
  const [selectedActiveId, setSelectedActiveId] = useState<number | null>(null);
  const [selectedContractId, setSelectedContractId] = useState<number | null>(null);

  const selectedActive = useMemo(
    () => participations?.find((p) => p.id === selectedActiveId) ?? null,
    [participations, selectedActiveId]
  );

  const selectedContract = useMemo(
    () => contracts?.find((c) => c.id === selectedContractId) ?? null,
    [contracts, selectedContractId]
  );

  const loading = partsLoading || contractsLoading;

  const invitations = useMemo(
    () => (participations ?? []).filter((p) => p.status === "pending" || p.status === "counter_pending"),
    [participations]
  );

  const activeParticipations = useMemo(
    () => (participations ?? []).filter((p) => p.status === "active" || p.status === "paused"),
    [participations]
  );

  const activeParticipationContractIds = useMemo(
    () => new Set(activeParticipations.map((p) => p.contract_id)),
    [activeParticipations]
  );

  const legacyContracts = useMemo(
    () => (contracts ?? []).filter((c) => !activeParticipationContractIds.has(c.id)),
    [contracts, activeParticipationContractIds]
  );

  function openLegacyContract(contract: Contract) {
    const part = (participations ?? []).find(
      (p) => p.contract_id === contract.id && (p.status === "active" || p.status === "paused")
    );
    if (part) {
      setSelectedActiveId(part.id);
      return;
    }
    setSelectedContractId(contract.id);
  }

  const hasContent =
    invitations.length > 0 || activeParticipations.length > 0 || legacyContracts.length > 0;

  return (
    <>
      <PageBody>
        {loading ? (
          <Spinner className="h-6 w-6" />
        ) : isError ? (
          <EmptyState title="Could not load contracts." subtitle={errorMessage(error)} />
        ) : !hasContent ? (
          <EmptyState title="No contract invitations yet." />
        ) : (
          <div className="space-y-8">
            {invitations.length > 0 && (
              <div>
                <h2 className="mb-3 text-sm font-semibold text-gray-700">Invitations</h2>
                <Table>
                  <THead>
                    <tr>
                      <TH>Publisher</TH>
                      <TH>Contract</TH>
                      <TH>Lead Type</TH>
                      <TH>Status</TH>
                    </tr>
                  </THead>
                  <TBody>
                    {invitations.map((p) => (
                      <TR key={p.id} onClick={() => setSelectedInvite(p)}>
                        <TD className="font-semibold">{p.publisher_name}</TD>
                        <TD>{p.contract_name}</TD>
                        <TD>{formatContractLeadType(p.lead_type) || "—"}</TD>
                        <TD>{formatParticipationStatus(p.status)}</TD>
                      </TR>
                    ))}
                  </TBody>
                </Table>
              </div>
            )}

            {activeParticipations.length > 0 && (
              <div>
                <h2 className="mb-3 text-sm font-semibold text-gray-700">Active contracts</h2>
                <Table>
                  <THead>
                    <tr>
                      <TH>Publisher</TH>
                      <TH>Contract</TH>
                      <TH>Lead Type</TH>
                      <TH>Rate / Lead</TH>
                      <TH>Leads</TH>
                      <TH>Status</TH>
                      <TH>Delivery</TH>
                    </tr>
                  </THead>
                  <TBody>
                    {activeParticipations.map((p) => (
                      <TR key={p.id} onClick={() => setSelectedActiveId(p.id)}>
                        <TD className="font-semibold">{p.publisher_name}</TD>
                        <TD>{p.contract_name}</TD>
                        <TD>{formatContractLeadType(p.lead_type) || "—"}</TD>
                        <TD>{formatMoney(p.rate_per_lead ?? 0)}</TD>
                        <TD>{p.lead_count ?? 0}</TD>
                        <TD>{formatParticipationStatus(p.status)}</TD>
                        <TD>{formatDeliveryMode(p.delivery)}</TD>
                      </TR>
                    ))}
                  </TBody>
                </Table>
              </div>
            )}

            {legacyContracts.length > 0 && (
              <div>
                <h2 className="mb-3 text-sm font-semibold text-gray-700">Legacy contracts</h2>
                <Table>
                  <THead>
                    <tr>
                      <TH>Publisher</TH>
                      <TH>Contract</TH>
                      <TH>Lead Type</TH>
                      <TH>Status</TH>
                    </tr>
                  </THead>
                  <TBody>
                    {legacyContracts.map((c) => (
                      <TR key={c.id} onClick={() => openLegacyContract(c)}>
                        <TD className="font-semibold">{c.publisher_name}</TD>
                        <TD>{c.name}</TD>
                        <TD>{formatContractLeadType(c.lead_type) || "—"}</TD>
                        <TD>{formatContractStatus(c.status)}</TD>
                      </TR>
                    ))}
                  </TBody>
                </Table>
              </div>
            )}
          </div>
        )}

        <BuyerParticipationAcceptDrawer participation={selectedInvite} onClose={() => setSelectedInvite(null)} />
        <BuyerParticipationDetailDrawer participation={selectedActive} onClose={() => setSelectedActiveId(null)} />
        <BuyerContractDetailDrawer contract={selectedContract} onClose={() => setSelectedContractId(null)} />
      </PageBody>
    </>
  );
}
