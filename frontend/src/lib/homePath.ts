import type { AccountType } from "@/types";

export function homePath(accountType: AccountType | string): string {
  if (accountType === "publisher") return "/p";
  if (accountType === "platform") return "/platform";
  return "/b";
}
