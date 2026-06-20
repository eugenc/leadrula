import { useCallback, useState } from "react";

const storageKey = (accountId: string) => `customFieldFolderCollapsed:${accountId}`;

export function loadCollapsedFolders(accountId: string): Record<string, boolean> {
  try {
    const raw = localStorage.getItem(storageKey(accountId));
    return raw ? (JSON.parse(raw) as Record<string, boolean>) : {};
  } catch {
    return {};
  }
}

export function saveCollapsedFolders(accountId: string, state: Record<string, boolean>): void {
  try {
    localStorage.setItem(storageKey(accountId), JSON.stringify(state));
  } catch {
    // localStorage may be unavailable (private mode); collapse state is non-critical.
  }
}

/** Per-user folder collapse state. Folders default to expanded (absent = expanded). */
export function useFolderCollapse(accountId: string | undefined) {
  const [collapsed, setCollapsed] = useState<Record<string, boolean>>(() =>
    accountId ? loadCollapsedFolders(accountId) : {}
  );

  const toggle = useCallback(
    (folderId: number) => {
      setCollapsed((prev) => {
        const next = { ...prev, [folderId]: !prev[folderId] };
        if (accountId) saveCollapsedFolders(accountId, next);
        return next;
      });
    },
    [accountId]
  );

  return { collapsed, toggle };
}
