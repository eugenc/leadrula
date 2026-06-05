import { useState } from "react";
import { useBuyerContracts } from "@/features/admin/hooks";
import { PageBody } from "@/components/layout/PageBody";
import { Table, THead, TH, TBody, TR, TD } from "@/components/ui/table";
import { Spinner, EmptyState } from "@/components/ui/misc";
import { errorMessage } from "@/lib/api";
import { formatMoney } from "@/lib/utils";
import { ContractStatusBadge } from "@/features/admin/contractStatus";
import { formatContractCap } from "@/features/admin/contractCap";
import { formatContractLeadType } from "@/features/admin/contractLeadType";
import { BuyerContractDetailDrawer } from "@/features/admin/BuyerContractDetailDrawer";
import type { Contract } from "@/types";

export function ContractPage() {
  const { data: contracts, isLoading, isError, error } = useBuyerContracts();
  const [selected, setSelected] = useState<Contract | null>(null);

  return (
    <>
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
                <TH>Publisher</TH>
                <TH>Name</TH>
                <TH>Lead Type</TH>
                <TH>Cap</TH>
                <TH>Rate / Lead</TH>
                <TH>Received</TH>
                <TH>Status</TH>
              </tr>
            </THead>
            <TBody>
              {(contracts ?? []).map((c) => (
                <TR key={c.id} onClick={() => setSelected(c)}>
                  <TD>Buy</TD>
                  <TD className="font-semibold">{c.publisher_name}</TD>
                  <TD>{c.name}</TD>
                  <TD>{formatContractLeadType(c.lead_type) || "—"}</TD>
                  <TD>{formatContractCap(c)}</TD>
                  <TD>{formatMoney(c.rate_per_lead)}</TD>
                  <TD>{c.lead_count ?? 0}</TD>
                  <TD>
                    <ContractStatusBadge status={c.status} />
                  </TD>
                </TR>
              ))}
            </TBody>
          </Table>
        )}

        <BuyerContractDetailDrawer contract={selected} onClose={() => setSelected(null)} />
      </PageBody>
    </>
  );
}
