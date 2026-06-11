import { useState } from "react";
import { useBuyerParticipations, useBuyerContracts } from "@/features/admin/hooks";
import { PageBody } from "@/components/layout/PageBody";
import { Table, THead, TH, TBody, TR, TD } from "@/components/ui/table";
import { Spinner, EmptyState } from "@/components/ui/misc";
import { errorMessage } from "@/lib/api";
import { formatContractLeadType } from "@/features/admin/contractLeadType";
import { formatParticipationStatus } from "@/features/admin/contractOffer";
import { BuyerParticipationAcceptDrawer } from "@/features/admin/BuyerParticipationAcceptDrawer";
import { BuyerContractDetailDrawer } from "@/features/admin/BuyerContractDetailDrawer";
import type { Contract, ContractParticipation } from "@/types";

export function ContractPage() {
  const { data: participations, isLoading: partsLoading, isError, error } = useBuyerParticipations();
  const { data: contracts, isLoading: contractsLoading } = useBuyerContracts();
  const [selected, setSelected] = useState<ContractParticipation | null>(null);
  const [selectedContract, setSelectedContract] = useState<Contract | null>(null);

  const loading = partsLoading || contractsLoading;

  return (
    <>
      <PageBody>
        {loading ? (
          <Spinner className="h-6 w-6" />
        ) : isError ? (
          <EmptyState title="Could not load contracts." subtitle={errorMessage(error)} />
        ) : (participations ?? []).length === 0 && (contracts ?? []).length === 0 ? (
          <EmptyState title="No contract invitations yet." />
        ) : (
          <div className="space-y-8">
            {(participations ?? []).length > 0 && (
              <div>
                <h2 className="mb-3 text-sm font-semibold text-gray-700">Invitations</h2>
                <Table>
                  <THead>
                    <tr>
                      <TH>Publisher</TH>
                      <TH>Contract</TH>
                      <TH>Lead Type</TH>
                      <TH>Status</TH>
                      <TH>Delivery</TH>
                    </tr>
                  </THead>
                  <TBody>
                    {(participations ?? []).map((p) => (
                      <TR key={p.id} onClick={() => setSelected(p)}>
                        <TD className="font-semibold">{p.publisher_name}</TD>
                        <TD>{p.contract_name}</TD>
                        <TD>{formatContractLeadType(p.lead_type) || "—"}</TD>
                        <TD>{formatParticipationStatus(p.status)}</TD>
                        <TD>{p.delivery || "—"}</TD>
                      </TR>
                    ))}
                  </TBody>
                </Table>
              </div>
            )}

            {(contracts ?? []).length > 0 && (
              <div>
                <h2 className="mb-3 text-sm font-semibold text-gray-700">Active contracts</h2>
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
                    {(contracts ?? []).map((c) => (
                      <TR key={c.id} onClick={() => setSelectedContract(c)}>
                        <TD className="font-semibold">{c.publisher_name}</TD>
                        <TD>{c.name}</TD>
                        <TD>{formatContractLeadType(c.lead_type) || "—"}</TD>
                        <TD>{c.status}</TD>
                      </TR>
                    ))}
                  </TBody>
                </Table>
              </div>
            )}
          </div>
        )}

        <BuyerParticipationAcceptDrawer participation={selected} onClose={() => setSelected(null)} />
        <BuyerContractDetailDrawer contract={selectedContract} onClose={() => setSelectedContract(null)} />
      </PageBody>
    </>
  );
}
