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
