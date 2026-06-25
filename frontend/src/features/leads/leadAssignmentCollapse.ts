import { useCallback, useState } from "react";

const storageKey = (accountId: string) => `leadAssignmentCollapsed:${accountId}`;

export function loadLeadAssignmentCollapsed(accountId: string): boolean {
  try {
    return localStorage.getItem(storageKey(accountId)) === "true";
  } catch {
    return false;
  }
}

export function saveLeadAssignmentCollapsed(accountId: string, collapsed: boolean): void {
  try {
    localStorage.setItem(storageKey(accountId), collapsed ? "true" : "false");
  } catch {
    // localStorage may be unavailable (private mode); collapse state is non-critical.
  }
}

/** Per-user assignment section collapse. Defaults to expanded. */
export function useLeadAssignmentCollapse(accountId: string | undefined) {
  const [collapsed, setCollapsed] = useState(() =>
    accountId ? loadLeadAssignmentCollapsed(accountId) : false
  );

  const toggle = useCallback(() => {
    setCollapsed((prev) => {
      const next = !prev;
      if (accountId) saveLeadAssignmentCollapsed(accountId, next);
      return next;
    });
  }, [accountId]);

  return { collapsed, toggle };
}
