import { useState } from "react";
import { useBuyerParticipations } from "@/features/admin/hooks";
import { PageBody } from "@/components/layout/PageBody";
import { Table, THead, TH, TBody, TR, TD } from "@/components/ui/table";
import { Spinner, EmptyState } from "@/components/ui/misc";
import { errorMessage } from "@/lib/api";
import { formatContractLeadType } from "@/features/admin/contractLeadType";
import { formatParticipationStatus } from "@/features/admin/contractOffer";
import { BuyerParticipationAcceptDrawer } from "@/features/admin/BuyerParticipationAcceptDrawer";
import type { ContractParticipation } from "@/types";

export function ContractPage() {
  const { data: participations, isLoading, isError, error } = useBuyerParticipations();
  const [selected, setSelected] = useState<ContractParticipation | null>(null);

  return (
    <>
      <PageBody>
        {isLoading ? (
          <Spinner className="h-6 w-6" />
        ) : isError ? (
          <EmptyState title="Could not load contracts." subtitle={errorMessage(error)} />
        ) : (participations ?? []).length === 0 ? (
          <EmptyState title="No contract invitations yet." />
        ) : (
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
        )}

        <BuyerParticipationAcceptDrawer participation={selected} onClose={() => setSelected(null)} />
      </PageBody>
    </>
  );
}
