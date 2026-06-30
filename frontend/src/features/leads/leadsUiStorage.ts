import {
  DEFAULT_VISIBLE_COLUMNS,
  DEFAULT_BOARD_CARD_FIELDS,
  normalizeBoardCardFields,
} from "./leadsListColumns";

export type SortDir = "asc" | "desc";

type ViewPlacement = "list" | "board";

export interface ListUiState {
  sort: string;
  sort_dir: SortDir;
  columns: string[];
  active_view_id?: string;
}

export interface BoardUiState {
  sort: string;
  sort_dir: SortDir;
  card_fields: string[];
  active_view_id?: string;
}

function listKey(userId: string) {
  return `leads-ui:list:${userId}`;
}

function boardKey(userId: string) {
  return `leads-ui:board:${userId}`;
}

function readJson<T>(key: string): T | null {
  try {
    const raw = localStorage.getItem(key);
    if (!raw) return null;
    return JSON.parse(raw) as T;
  } catch {
    return null;
  }
}

function writeJson(key: string, value: unknown) {
  localStorage.setItem(key, JSON.stringify(value));
}

function parseSortDir(v: unknown): SortDir {
  return v === "asc" ? "asc" : "desc";
}

export function normalizeListColumns(cols: string[], validIds: string[]): string[] {
  const mapped = cols.map((c) => (c === "campaign" ? "source" : c));
  const filtered = mapped.filter((id) => validIds.includes(id));
  return filtered.length ? filtered : [...DEFAULT_VISIBLE_COLUMNS];
}

export function loadListUi(userId: string, validColumnIds: string[]): ListUiState | null {
  const stored = readJson<Partial<ListUiState>>(listKey(userId));
  if (!stored || typeof stored.sort !== "string") return null;
  return {
    sort: stored.sort,
    sort_dir: parseSortDir(stored.sort_dir),
    columns: normalizeListColumns(Array.isArray(stored.columns) ? stored.columns : [], validColumnIds),
  };
}

export function saveListUi(userId: string, patch: Partial<ListUiState>) {
  const key = listKey(userId);
  const prev = readJson<Partial<ListUiState>>(key) ?? {};
  writeJson(key, { ...prev, ...patch });
}

export function loadBoardUi(
  userId: string,
  validColumnIds: string[],
  legacyBackendCardFields?: string[] | null
): BoardUiState | null {
  const stored = readJson<Partial<BoardUiState>>(boardKey(userId));
  if (stored && typeof stored.sort === "string") {
    return {
      sort: stored.sort,
      sort_dir: parseSortDir(stored.sort_dir),
      card_fields: normalizeBoardCardFields(
        Array.isArray(stored.card_fields) ? stored.card_fields : [],
        validColumnIds
      ),
    };
  }
  if (legacyBackendCardFields?.length) {
    const migrated: BoardUiState = {
      sort: "created_at",
      sort_dir: "desc",
      card_fields: normalizeBoardCardFields(legacyBackendCardFields, validColumnIds),
    };
    saveBoardUi(userId, migrated);
    return migrated;
  }
  return null;
}

export function saveBoardUi(userId: string, patch: Partial<BoardUiState>) {
  const key = boardKey(userId);
  const prev = readJson<Partial<BoardUiState>>(key) ?? {};
  writeJson(key, { ...prev, ...patch });
}

export function defaultBoardUi(validColumnIds: string[]): BoardUiState {
  return {
    sort: "created_at",
    sort_dir: "desc",
    card_fields: normalizeBoardCardFields(DEFAULT_BOARD_CARD_FIELDS, validColumnIds),
  };
}

function uiKey(userId: string, placement: ViewPlacement) {
  return placement === "list" ? listKey(userId) : boardKey(userId);
}

export function loadActiveViewId(userId: string, placement: ViewPlacement): string | null {
  const stored = readJson<Partial<ListUiState & BoardUiState>>(uiKey(userId, placement));
  const id = stored?.active_view_id;
  return typeof id === "string" && id ? id : null;
}

export function saveActiveViewId(userId: string, placement: ViewPlacement, viewId: string) {
  const key = uiKey(userId, placement);
  const prev = readJson<Partial<ListUiState & BoardUiState>>(key) ?? {};
  writeJson(key, { ...prev, active_view_id: viewId });
}
