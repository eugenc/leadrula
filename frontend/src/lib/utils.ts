import { clsx, type ClassValue } from "clsx";
import { twMerge } from "tailwind-merge";

export function cn(...inputs: ClassValue[]) {
  return twMerge(clsx(inputs));
}

export function formatMoney(n: number | undefined | null): string {
  const v = typeof n === "number" ? n : 0;
  return v.toLocaleString("en-US", { style: "currency", currency: "USD" });
}

export function initials(name: string): string {
  return name
    .split(" ")
    .filter(Boolean)
    .slice(0, 2)
    .map((p) => p[0]?.toUpperCase())
    .join("");
}

export function formatRole(role: string): string {
  if (!role) return "";
  return role.charAt(0).toUpperCase() + role.slice(1);
}

export function formatTxnType(type: string): string {
  return type
    .split("_")
    .map((word) => word.charAt(0).toUpperCase() + word.slice(1))
    .join(" ");
}

export function resolveBuyerTxnCategory(t: { type: string }): string {
  switch (t.type) {
    case "debit":
      return "Purchase";
    case "credit":
      return "Credit";
    case "topup":
      return "Topup";
    case "dispute_credit":
      return "Refund";
    case "manual_invoice":
      return "Invoice";
    default:
      return formatTxnType(t.type);
  }
}

export function resolveTxnCategory(t: {
  category?: string;
  side?: string;
  type: string;
}): string {
  if (t.category) return t.category;
  if (t.side === "purchase") return "Purchase";
  if (t.side === "prepay") {
    if (t.type === "topup") return "Topup";
    if (t.type === "credit") return "Credit";
    if (t.type === "dispute_credit") return "Refund";
  }
  if (t.type === "manual_invoice") return "Invoice";
  if (t.side === "sale" || t.type === "debit") return "Sale";
  return formatTxnType(t.type);
}
