export type AppointmentPreset = "all" | "today" | "this_week" | "this_month";
export type AppointmentSort = "booked_at" | "appointment_at";
export type SortDir = "asc" | "desc";

export interface AppointmentsUiState {
  preset: AppointmentPreset;
  sort: AppointmentSort;
  sort_dir: SortDir;
  contract_id: number;
  publisher_id: number;
  limit: number;
}

const DEFAULT: AppointmentsUiState = {
  preset: "this_week",
  sort: "booked_at",
  sort_dir: "desc",
  contract_id: 0,
  publisher_id: 0,
  limit: 25,
};

function storageKey(userId: string) {
  return `appointments-ui:${userId}`;
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

export function loadAppointmentsUi(userId: string): AppointmentsUiState {
  const stored = readJson<Partial<AppointmentsUiState>>(storageKey(userId));
  if (!stored) return { ...DEFAULT };
  return {
    preset: stored.preset ?? DEFAULT.preset,
    sort: stored.sort === "appointment_at" ? "appointment_at" : "booked_at",
    sort_dir: stored.sort_dir === "asc" ? "asc" : "desc",
    contract_id: stored.contract_id ?? 0,
    publisher_id: stored.publisher_id ?? 0,
    limit: stored.limit && [25, 50, 100].includes(stored.limit) ? stored.limit : DEFAULT.limit,
  };
}

export function saveAppointmentsUi(userId: string, patch: Partial<AppointmentsUiState>) {
  const key = storageKey(userId);
  const prev = readJson<Partial<AppointmentsUiState>>(key) ?? {};
  localStorage.setItem(key, JSON.stringify({ ...prev, ...patch }));
}

export function defaultAppointmentsUi(): AppointmentsUiState {
  return { ...DEFAULT };
}

export const APPOINTMENT_PRESET_LABELS: Record<AppointmentPreset, string> = {
  all: "All dates",
  today: "Today",
  this_week: "This week",
  this_month: "This month",
};

export const PAGE_SIZES = [25, 50, 100] as const;
