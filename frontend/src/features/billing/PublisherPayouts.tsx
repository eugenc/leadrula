import { format } from "date-fns";
import { usePayoutSummary, usePayoutByCompensation } from "@/features/admin/hooks";
import { Card, Badge, Spinner, StatCard, EmptyState } from "@/components/ui/misc";
import { Table, THead, TH, TBody, TR, TD } from "@/components/ui/table";
import { formatMoney } from "@/lib/utils";
import { PAYOUT_FREQUENCIES, PAYOUT_WEEKDAYS } from "@/features/admin/contractCompensation";
import type { CompensationPayoutRow } from "@/types";

function payoutScheduleLabel(row: {
  payout_frequency?: string | null;
  payout_weekday?: number | null;
  payout_month_day?: number | null;
}): string {
  if (!row.payout_frequency) return "—";
  const freq = PAYOUT_FREQUENCIES.find((f) => f.value === row.payout_frequency)?.label ?? row.payout_frequency;
  if (row.payout_frequency === "weekly" && row.payout_weekday != null) {
    const day = PAYOUT_WEEKDAYS.find((d) => d.value === row.payout_weekday)?.label;
    return `${freq} · ${day ?? row.payout_weekday}`;
  }
  if (row.payout_frequency === "monthly" && row.payout_month_day != null) {
    return `${freq} · day ${row.payout_month_day}`;
  }
  return freq;
}

function isSharePayout(kind: string): boolean {
  return kind === "rev_share" || kind === "profit_share";
}

function payoutStatusLabel(row: CompensationPayoutRow): string {
  if (isSharePayout(row.kind)) {
    if (row.invoice_status === "paid") return "Paid via invoice";
    if (row.invoice_status === "open") return "Invoice open";
    if (row.cleared > 0) return "Invoice pending";
    return "—";
  }
  if (row.buyer_kind === "direct") {
    return row.latest_transfer_status === "skipped" || !row.latest_transfer_status ? "N/A (direct)" : row.latest_transfer_status;
  }
  switch (row.latest_transfer_status) {
    case "sent":
      return "Paid out";
    case "pending":
      return "Pending connect";
    case "failed":
      return "Transfer failed";
    case "skipped":
      return "Skipped";
    default:
      return "—";
  }
}

function payoutStatusVariant(
  row: CompensationPayoutRow
): "default" | "distributed" | "pending" | "overdue" {
  if (isSharePayout(row.kind)) {
    if (row.invoice_status === "paid") return "distributed";
    if (row.invoice_status === "open") return "pending";
    return "default";
  }
  if (row.buyer_kind === "direct") return "default";
  switch (row.latest_transfer_status) {
    case "sent":
      return "distributed";
    case "failed":
      return "overdue";
    case "pending":
      return "pending";
    default:
      return "default";
  }
}

export function PublisherPayouts() {
  const { data: summary, isLoading: summaryLoading } = usePayoutSummary();
  const { data: rows, isLoading: rowsLoading } = usePayoutByCompensation();

  return (
    <div className="space-y-6">
      <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-4">
        {summaryLoading ? (
          <Spinner className="h-6 w-6" />
        ) : (
          <>
            <StatCard label="Payout Hold" value={formatMoney(summary?.hold ?? 0)} />
            <StatCard label="Payout Cleared" value={formatMoney(summary?.cleared ?? 0)} />
            <StatCard label="Distributed from prepay" value={formatMoney(summary?.distributed_value ?? 0)} />
            <StatCard label="Returned" value={formatMoney(summary?.returned_value ?? 0)} />
          </>
        )}
      </div>

      {!summaryLoading && summary && (
        <Card className="p-4">
          <div className="mb-2 text-xs font-semibold uppercase tracking-wide text-gray-400">Prepay context</div>
          <p className="text-sm text-gray-600">
            Buyer prepay balance across contracts:{" "}
            <span className="font-semibold text-gray-800">{formatMoney(summary.prepay_balance)}</span>
            {" · "}
            Cleared from lump sum (distributed minus returns, paid out on schedule):{" "}
            <span className="font-semibold text-gray-800">{formatMoney(summary.cleared_from_prepay)}</span>
          </p>
        </Card>
      )}

      <div>
        <h3 className="mb-3 text-base font-semibold text-gray-800">By compensation</h3>
        {rowsLoading ? (
          <Spinner className="h-6 w-6" />
        ) : (rows ?? []).length === 0 ? (
          <EmptyState title="No compensation rows yet." />
        ) : (
          <Table>
            <THead>
              <tr>
                <TH>Contract</TH>
                <TH>Kind</TH>
                <TH>Buyer</TH>
                <TH>Schedule</TH>
                <TH>Hold</TH>
                <TH>Cleared</TH>
                <TH>Payout</TH>
                <TH>Next payout</TH>
              </tr>
            </THead>
            <TBody>
              {(rows ?? []).map((r) => (
                <TR key={r.compensation_id}>
                  <TD className="font-medium text-gray-800">{r.contract_name}</TD>
                  <TD className="text-gray-600">{r.kind.replace("_", " ")}</TD>
                  <TD className="capitalize text-gray-600">{r.buyer_kind}</TD>
                  <TD className="text-gray-600">{payoutScheduleLabel(r)}</TD>
                  <TD>{formatMoney(r.hold)}</TD>
                  <TD>{formatMoney(r.cleared)}</TD>
                  <TD>
                    <Badge variant={payoutStatusVariant(r)}>
                      {payoutStatusLabel(r)}
                    </Badge>
                    {isSharePayout(r.kind) && r.invoice_public_id && (
                      <div className="mt-1 text-xs text-gray-400">{r.invoice_public_id.slice(0, 8)}…</div>
                    )}
                  </TD>
                  <TD className="text-gray-600">
                    {r.next_period_end
                      ? format(new Date(r.next_period_end), "MMM d, yyyy")
                      : "—"}
                  </TD>
                </TR>
              ))}
            </TBody>
          </Table>
        )}
      </div>
    </div>
  );
}
