import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { format, parse } from "date-fns";
import { get, patch, post, put } from "@/lib/api";
import { QUARTER_MINUTES } from "@/features/leads/customFieldDate";
import type {
  AppointmentBooking,
  AppointmentCalendarMarker,
  AppointmentContractOption,
  BuyerAppointmentContractOption,
  AppointmentFreeSlot,
  BuyerAppointmentSlot,
  BuyerBookingCalendar,
  ContractAppointmentSlot,
} from "@/types";

const WEEKDAYS = ["Sun", "Mon", "Tue", "Wed", "Thu", "Fri", "Sat"] as const;
export { WEEKDAYS };

export function useBuyerCalendars() {
  return useQuery({
    queryKey: ["buyer-booking-calendars"],
    queryFn: async () => {
      const res = await get<{ items: BuyerBookingCalendar[] }>("/buyer/booking-calendars");
      return res.items ?? [];
    },
  });
}

export function useBookingCalendar(calendarId: number | null) {
  return useQuery({
    queryKey: ["buyer-booking-calendar", calendarId],
    queryFn: () => get<BuyerBookingCalendar>(`/buyer/booking-calendars/${calendarId}`),
    enabled: !!calendarId,
  });
}

export function useCreateBookingCalendar() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (body: { name: string; timezone: string }) =>
      post<BuyerBookingCalendar>("/buyer/booking-calendars", body),
    onSuccess: () => qc.invalidateQueries({ queryKey: ["buyer-booking-calendars"] }),
  });
}

export function useSaveBookingCalendar(calendarId: number) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (body: Partial<BuyerBookingCalendar>) =>
      put<BuyerBookingCalendar>(`/buyer/booking-calendars/${calendarId}`, body),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ["buyer-booking-calendar", calendarId] });
      qc.invalidateQueries({ queryKey: ["buyer-booking-calendars"] });
      qc.invalidateQueries({ queryKey: ["buyer-calendar-slots", calendarId] });
    },
  });
}

export function useCalendarSlots(calendarId: number | null) {
  return useQuery({
    queryKey: ["buyer-calendar-slots", calendarId],
    queryFn: async () => {
      const res = await get<{ items: BuyerAppointmentSlot[] }>(
        `/buyer/booking-calendars/${calendarId}/slots`
      );
      return res.items ?? [];
    },
    enabled: !!calendarId,
  });
}

export function useCreateCalendarSlot(calendarId: number) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (body: { weekday: number; start_time: string; duration_min: number; capacity: number }) =>
      post<BuyerAppointmentSlot>(`/buyer/booking-calendars/${calendarId}/slots`, body),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ["buyer-calendar-slots", calendarId] });
      qc.invalidateQueries({ queryKey: ["buyer-booking-calendars"] });
      qc.invalidateQueries({ queryKey: ["buyer-booking-calendar", calendarId] });
    },
  });
}

export function usePatchCalendarSlot(calendarId: number) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: ({
      id,
      body,
    }: {
      id: number;
      body: { start_time?: string; duration_min?: number; capacity?: number; disabled?: boolean };
    }) => patch<BuyerAppointmentSlot>(`/buyer/booking-calendars/${calendarId}/slots/${id}`, body),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ["buyer-calendar-slots", calendarId] });
      qc.invalidateQueries({ queryKey: ["buyer-booking-calendars"] });
    },
  });
}

export function useCopyCalendarSlots(calendarId: number) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (body: { from_weekday: number; to_weekdays: number[] }) =>
      post<{ items: BuyerAppointmentSlot[] }>(`/buyer/booking-calendars/${calendarId}/slots/copy`, body),
    onSuccess: () => qc.invalidateQueries({ queryKey: ["buyer-calendar-slots", calendarId] }),
  });
}

export function useBuyerCalendarMarkers(calendarId: number | null, from: string, to: string) {
  return useQuery({
    queryKey: ["buyer-calendar-markers", calendarId, from, to],
    queryFn: async () => {
      const res = await get<{ items: AppointmentCalendarMarker[] }>(
        `/buyer/booking-calendars/${calendarId}/markers?from=${from}&to=${to}`
      );
      return res.items ?? [];
    },
    enabled: !!calendarId && !!from && !!to,
  });
}

export function useBuyerCalendarBookings(calendarId: number | null) {
  return useQuery({
    queryKey: ["buyer-calendar-appointments", calendarId],
    queryFn: async () => {
      const res = await get<{ items: AppointmentBooking[] }>(
        `/buyer/booking-calendars/${calendarId}/appointments`
      );
      return res.items ?? [];
    },
    enabled: !!calendarId,
  });
}

export function useSetContractAppointmentCalendar() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: ({ contractId, appointment_calendar_id }: { contractId: number; appointment_calendar_id: number }) =>
      patch(`/buyer/contracts/${contractId}/appointment-calendar`, { appointment_calendar_id }),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ["buyer-contracts"] });
      qc.invalidateQueries({ queryKey: ["contract-appointment-slots"] });
    },
  });
}

export interface BuyerBookingsParams {
  page?: number;
  limit?: number;
  sort?: string;
  sort_dir?: "asc" | "desc";
  q?: string;
  contract_id?: number;
  publisher_id?: number;
  appointment_preset?: string;
}

export interface BuyerBookingsResult {
  items: AppointmentBooking[];
  total: number;
}

export function useBuyerBookings(params: BuyerBookingsParams) {
  return useQuery({
    queryKey: ["buyer-appointments", params],
    queryFn: async () => {
      const sp = new URLSearchParams();
      if (params.page) sp.set("page", String(params.page));
      if (params.limit) sp.set("limit", String(params.limit));
      if (params.sort) sp.set("sort", params.sort);
      if (params.sort_dir) sp.set("sort_dir", params.sort_dir);
      if (params.q) sp.set("q", params.q);
      if (params.contract_id) sp.set("contract_id", String(params.contract_id));
      if (params.publisher_id) sp.set("publisher_id", String(params.publisher_id));
      if (params.appointment_preset) sp.set("appointment_preset", params.appointment_preset);
      const qs = sp.toString();
      return get<BuyerBookingsResult>(`/buyer/appointments${qs ? `?${qs}` : ""}`);
    },
  });
}

export function usePublisherAppointmentContracts() {
  return useQuery({
    queryKey: ["publisher-appointment-contracts"],
    queryFn: async () => {
      const res = await get<{ items: AppointmentContractOption[] }>("/publisher/appointments/contracts");
      return res.items ?? [];
    },
  });
}

export function useContractAppointmentSlots(contractId: number | null) {
  return useQuery({
    queryKey: ["contract-appointment-slots", contractId],
    queryFn: async () => {
      const res = await get<{ items: ContractAppointmentSlot[] }>(
        `/publisher/contracts/${contractId}/appointment-slots`
      );
      return res.items ?? [];
    },
    enabled: !!contractId,
  });
}

export function useSaveContractAppointmentSlots() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: ({
      contractId,
      slots,
    }: {
      contractId: number;
      slots: {
        buyer_slot_id: number;
        enabled: boolean;
        duration_min_override?: number | null;
        capacity_override?: number | null;
      }[];
    }) =>
      put<{ items: ContractAppointmentSlot[] }>(`/publisher/contracts/${contractId}/appointment-slots`, { slots }),
    onSuccess: (_d, { contractId }) => {
      qc.invalidateQueries({ queryKey: ["contract-appointment-slots", contractId] });
    },
  });
}

export function useFreeSlots(contractId: number | null, date: string) {
  return useQuery({
    queryKey: ["appointment-free-slots", contractId, date],
    queryFn: async () => {
      const res = await get<{ items: AppointmentFreeSlot[] }>(
        `/publisher/appointments/slots?contract_id=${contractId}&date=${date}`
      );
      return res.items ?? [];
    },
    enabled: !!contractId && !!date,
  });
}

export function useCalendarMarkers(contractId: number | null, from: string, to: string) {
  return useQuery({
    queryKey: ["appointment-calendar-markers", contractId, from, to],
    queryFn: async () => {
      const res = await get<{ items: AppointmentCalendarMarker[] }>(
        `/publisher/appointments/calendar-markers?contract_id=${contractId}&from=${from}&to=${to}`
      );
      return res.items ?? [];
    },
    enabled: !!contractId && !!from && !!to,
  });
}

export function usePublisherBookings() {
  return useQuery({
    queryKey: ["publisher-appointments"],
    queryFn: async () => {
      const res = await get<{ items: AppointmentBooking[] }>("/publisher/appointments/booked");
      return res.items ?? [];
    },
  });
}

export function useBookAppointment() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (body: Record<string, unknown>) => post<AppointmentBooking>("/publisher/appointments/book", body),
    onSuccess: (_data, body) => {
      qc.invalidateQueries({ queryKey: ["appointment-free-slots"] });
      qc.invalidateQueries({ queryKey: ["appointment-calendar-markers"] });
      qc.invalidateQueries({ queryKey: ["publisher-appointments"] });
      qc.invalidateQueries({ queryKey: ["buyer-appointments"] });
      qc.invalidateQueries({ queryKey: ["leads"] });
      qc.invalidateQueries({ queryKey: ["lead"] });
      const leadId = body.lead_id;
      if (typeof leadId === "number") {
        qc.invalidateQueries({ queryKey: ["lead", leadId] });
        qc.invalidateQueries({ queryKey: ["lead-history", leadId] });
      }
    },
  });
}

export function useBuyerAppointmentContracts() {
  return useQuery({
    queryKey: ["buyer-appointment-contracts"],
    queryFn: async () => {
      const res = await get<{ items: BuyerAppointmentContractOption[] }>("/buyer/appointments/contracts");
      return res.items ?? [];
    },
  });
}

export function useBuyerFreeSlots(contractId: number | null, date: string) {
  return useQuery({
    queryKey: ["buyer-appointment-free-slots", contractId, date],
    queryFn: async () => {
      const res = await get<{ items: AppointmentFreeSlot[] }>(
        `/buyer/appointments/slots?contract_id=${contractId}&date=${date}`
      );
      return res.items ?? [];
    },
    enabled: !!contractId && !!date,
  });
}

export function useBuyerAppointmentCalendarMarkers(contractId: number | null, from: string, to: string) {
  return useQuery({
    queryKey: ["buyer-appointment-calendar-markers", contractId, from, to],
    queryFn: async () => {
      const res = await get<{ items: AppointmentCalendarMarker[] }>(
        `/buyer/appointments/calendar-markers?contract_id=${contractId}&from=${from}&to=${to}`
      );
      return res.items ?? [];
    },
    enabled: !!contractId && !!from && !!to,
  });
}

export function useBuyerBookAppointment() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (body: Record<string, unknown>) => post<AppointmentBooking>("/buyer/appointments/book", body),
    onSuccess: (_data, body) => {
      qc.invalidateQueries({ queryKey: ["buyer-appointment-free-slots"] });
      qc.invalidateQueries({ queryKey: ["buyer-appointment-calendar-markers"] });
      qc.invalidateQueries({ queryKey: ["buyer-appointments"] });
      qc.invalidateQueries({ queryKey: ["leads"] });
      qc.invalidateQueries({ queryKey: ["lead"] });
      const leadId = body.lead_id;
      if (typeof leadId === "number") {
        qc.invalidateQueries({ queryKey: ["lead", leadId] });
        qc.invalidateQueries({ queryKey: ["lead-history", leadId] });
      }
    },
  });
}

export function timeOptions15(): string[] {
  const out: string[] = [];
  for (let h = 0; h < 24; h++) {
    for (const m of [0, 15, 30, 45]) {
      out.push(`${String(h).padStart(2, "0")}:${String(m).padStart(2, "0")}`);
    }
  }
  return out;
}

export type TimeHhmmParts = { hour12: number; minute: number; period: "AM" | "PM" };

const pad2 = (n: number) => String(n).padStart(2, "0");

export function timeHhmmToMinutes(t: string): number {
  const [h, m] = t.split(":").map(Number);
  return h * 60 + m;
}

export function minutesToTimeHhmm(m: number): string {
  const h = Math.floor(m / 60) % 24;
  const min = m % 60;
  return `${pad2(h)}:${pad2(min)}`;
}

export function parseTimeHhmm(value: string): TimeHhmmParts {
  const [h24, minute] = value.split(":").map(Number);
  const period: "AM" | "PM" = h24 >= 12 ? "PM" : "AM";
  const hour12 = h24 % 12 || 12;
  const snapped = QUARTER_MINUTES.reduce((best, m) =>
    Math.abs(m - minute) < Math.abs(best - minute) ? m : best
  );
  return { hour12, minute: snapped, period };
}

export function buildTimeHhmm(parts: TimeHhmmParts): string {
  let h24 = parts.hour12 % 12;
  if (parts.period === "PM") h24 += 12;
  return `${pad2(h24)}:${pad2(parts.minute)}`;
}

export function formatTimeHhmm12(value: string): string {
  const d = parse(value.slice(0, 5), "HH:mm", new Date());
  return format(d, "h:mm a");
}

export function isStartValidForWindow(
  start: string,
  durationMin: number,
  dayStart: string,
  dayEnd: string
): boolean {
  const startMin = timeHhmmToMinutes(start);
  const endMin = startMin + durationMin;
  return startMin >= timeHhmmToMinutes(dayStart) && endMin <= timeHhmmToMinutes(dayEnd);
}

export function firstValidStartInWindow(
  dayStart: string,
  dayEnd: string,
  durationMin: number
): string | null {
  const startMin = timeHhmmToMinutes(dayStart);
  const latestStart = timeHhmmToMinutes(dayEnd) - durationMin;
  if (latestStart < startMin) return null;
  return minutesToTimeHhmm(startMin);
}

export const DEFAULT_WEEKLY_HOURS: Record<string, { start: string; end: string }> = {
  mon: { start: "09:00", end: "17:00" },
  tue: { start: "09:00", end: "17:00" },
  wed: { start: "09:00", end: "17:00" },
  thu: { start: "09:00", end: "17:00" },
  fri: { start: "09:00", end: "17:00" },
};

export const WEEKDAY_KEYS = ["sun", "mon", "tue", "wed", "thu", "fri", "sat"] as const;
