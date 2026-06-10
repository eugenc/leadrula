import type { InvoiceStatus } from "@/types";
import { formatTxnType } from "@/lib/utils";

export const MANUAL_PAYMENT_METHODS = [
  { value: "bank_transfer", label: "Bank Transfer (ACH / Wire / EFT)" },
  { value: "check", label: "Check" },
  { value: "cash", label: "Cash" },
  { value: "other_digital", label: "Other Digital (Zelle, Venmo, PayPal…)" },
  { value: "other", label: "Other" },
] as const;

export const INVOICE_KIND_LABELS: Record<string, string> = {
  starting_balance: "Starting balance",
  prepay_request: "Prepay request",
};

export const INVOICE_PAYMENT_LABELS: Record<string, string> = {
  stripe: "Stripe",
  bank_transfer: "Bank Transfer",
  check: "Check",
  cash: "Cash",
  other_digital: "Other Digital",
  other: "Other",
};

const INVOICE_STATUS_LABELS: Record<InvoiceStatus, string> = {
  open: "Open",
  paid: "Paid",
  void: "Canceled",
};

export function formatInvoiceStatus(status: InvoiceStatus | string): string {
  return INVOICE_STATUS_LABELS[status as InvoiceStatus] ?? status;
}

export function formatInvoicePaymentMethod(method: string): string {
  return INVOICE_PAYMENT_LABELS[method] ?? formatTxnType(method);
}
