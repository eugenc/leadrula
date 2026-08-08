import {
  useMutation,
  useQuery,
  useQueryClient,
  type QueryClient,
} from "@tanstack/react-query";
import { format, parse } from "date-fns";
import { get, patch, post, put, del } from "@/lib/api";
import { QUARTER_MINUTES } from "@/features/leads/customFieldDate";
import type {
  AppointmentBooking,
  AppointmentCalendarMarker,
  AppointmentContractOption,
  AppointmentDaySlotsResult,
  AppointmentDayWorkingHours,
  BuyerAppointmentContractOption,
  AppointmentFreeSlot,
  BuyerAppointmentSlot,
  BuyerBookingCalendar,
  ContractAppointmentSlot,
  ContractPublisherAppointmentSlot,
  PublisherAppointmentSlot,
  PublisherBookingCalendar,
} from "@/types";

const WEEKDAYS = ["Sun", "Mon", "Tue", "Wed", "Thu", "Fri", "Sat"] as const;
export { WEEKDAYS };

type CalendarOwnerKind = "buyer" | "publisher";

type CreateSlotVariables = {
  weekday: number;
  start_time: string;
  duration_min: number;
  capacity: number;
  skipOptimistic?: boolean;
};

type PatchSlotVariables = {
  id: number;
  body: { start_time?: string; duration_min?: number; capacity?: number; disabled?: boolean };
  skipOptimistic?: boolean;
};

type SlotMutationContext = {
  prev: BuyerAppointmentSlot[];
  tempId?: number;
  slotsKey: readonly [string, number];
};

function optimisticTempId(): number {
  return -(Date.now() + Math.floor(Math.random() * 1000));
}

export function isOptimisticSlotId(id: number | string): boolean {
  return typeof id === "number" && id < 0;
}

export function buildOptimisticSlot(
  calendarId: number,
  accountId: number,
  body: { weekday: number; start_time: string; duration_min: number; capacity: number }
): BuyerAppointmentSlot {
  return {
    id: optimisticTempId(),
    account_id: accountId,
    calendar_id: calendarId,
    weekday: body.weekday,
    start_time: body.start_time,
    duration_min: body.duration_min,
    capacity: body.capacity,
    disabled_at: null,
  };
}

function countActiveSlots(slots: BuyerAppointmentSlot[]): number {
  return slots.filter((s) => !s.disabled_at).length;
}

function slotsQueryKey(owner: CalendarOwnerKind, calendarId: number) {
  return owner === "buyer"
    ? (["buyer-calendar-slots", calendarId] as const)
    : (["publisher-calendar-slots", calendarId] as const);
}

function calendarsListQueryKey(owner: CalendarOwnerKind) {
  return owner === "buyer" ? (["buyer-booking-calendars"] as const) : (["publisher-booking-calendars"] as const);
}

function calendarQueryKey(owner: CalendarOwnerKind, calendarId: number) {
  return owner === "buyer"
    ? (["buyer-booking-calendar", calendarId] as const)
    : (["publisher-booking-calendar", calendarId] as const);
}

function syncCalendarSlotMeta(
  qc: QueryClient,
  owner: CalendarOwnerKind,
  calendarId: number,
  slots: BuyerAppointmentSlot[]
) {
  const activeCount = countActiveSlots(slots);
  const listKey = calendarsListQueryKey(owner);
  qc.setQueryData<BuyerBookingCalendar[]>(listKey, (old) => {
    if (!old) return old;
    return old.map((c) =>
      c.id === calendarId ? { ...c, slot_count: activeCount, configured: activeCount > 0 } : c
    );
  });
  qc.setQueryData<BuyerBookingCalendar>(calendarQueryKey(owner, calendarId), (old) => {
    if (!old) return old;
    return { ...old, slot_count: activeCount, configured: activeCount > 0 };
  });
}

export function setCalendarSlotsCache(
  qc: QueryClient,
  owner: CalendarOwnerKind,
  calendarId: number,
  slots: BuyerAppointmentSlot[]
) {
  qc.setQueryData(slotsQueryKey(owner, calendarId), slots);
  syncCalendarSlotMeta(qc, owner, calendarId, slots);
}

export function invalidateCalendarSlots(
  qc: QueryClient,
  owner: CalendarOwnerKind,
  calendarId: number
) {
  qc.invalidateQueries({ queryKey: slotsQueryKey(owner, calendarId) });
  qc.invalidateQueries({ queryKey: calendarsListQueryKey(owner) });
  qc.invalidateQueries({ queryKey: calendarQueryKey(owner, calendarId) });
}

function createCalendarSlotMutations(
  qc: QueryClient,
  owner: CalendarOwnerKind,
  calendarId: number,
  postFn: (body: {
    weekday: number;
    start_time: string;
    duration_min: number;
    capacity: number;
  }) => Promise<BuyerAppointmentSlot>
) {
  const slotsKey = slotsQueryKey(owner, calendarId);
  return {
    mutationFn: ({ skipOptimistic: _, ...body }: CreateSlotVariables) => postFn(body),
    onMutate: async (variables: CreateSlotVariables) => {
      if (variables.skipOptimistic) return undefined;
      const { weekday, start_time, duration_min, capacity } = variables;
      await qc.cancelQueries({ queryKey: slotsKey });
      const prev = qc.getQueryData<BuyerAppointmentSlot[]>(slotsKey) ?? [];
      const tempId = optimisticTempId();
      const optimistic: BuyerAppointmentSlot = {
        id: tempId,
        account_id: prev[0]?.account_id ?? 0,
        calendar_id: calendarId,
        weekday,
        start_time,
        duration_min,
        capacity,
        disabled_at: null,
      };
      const next = [...prev, optimistic];
      qc.setQueryData(slotsKey, next);
      syncCalendarSlotMeta(qc, owner, calendarId, next);
      return { prev, tempId, slotsKey } satisfies SlotMutationContext;
    },
    onError: (_err: unknown, _vars: unknown, context: SlotMutationContext | undefined) => {
      if (!context) return;
      qc.setQueryData(context.slotsKey, context.prev);
      syncCalendarSlotMeta(qc, owner, calendarId, context.prev);
    },
    onSuccess: (serverSlot: BuyerAppointmentSlot, _vars: unknown, context: SlotMutationContext | undefined) => {
      if (!context?.tempId) return;
      qc.setQueryData(context.slotsKey, (old: BuyerAppointmentSlot[] | undefined) => {
        if (!old) return [serverSlot];
        return old.map((s) => (s.id === context.tempId ? serverSlot : s));
      });
    },
  };
}

function patchCalendarSlotMutations(
  qc: QueryClient,
  owner: CalendarOwnerKind,
  calendarId: number,
  patchFn: (args: {
    id: number;
    body: { start_time?: string; duration_min?: number; capacity?: number; disabled?: boolean };
  }) => Promise<BuyerAppointmentSlot>
) {
  const slotsKey = slotsQueryKey(owner, calendarId);
  return {
    mutationFn: ({ id, body }: PatchSlotVariables) => patchFn({ id, body }),
    onMutate: async ({ id, body, skipOptimistic }: PatchSlotVariables) => {
      if (skipOptimistic) return undefined;
      await qc.cancelQueries({ queryKey: slotsKey });
      const prev = qc.getQueryData<BuyerAppointmentSlot[]>(slotsKey) ?? [];
      const now = new Date().toISOString();
      const next = prev.map((s) => {
        if (s.id !== id) return s;
        if (body.disabled) return { ...s, disabled_at: now };
        return {
          ...s,
          ...(body.start_time != null ? { start_time: body.start_time } : {}),
          ...(body.duration_min != null ? { duration_min: body.duration_min } : {}),
          ...(body.capacity != null ? { capacity: body.capacity } : {}),
        };
      });
      qc.setQueryData(slotsKey, next);
      syncCalendarSlotMeta(qc, owner, calendarId, next);
      return { prev, slotsKey } satisfies SlotMutationContext;
    },
    onError: (_err: unknown, _vars: unknown, context: SlotMutationContext | undefined) => {
      if (!context) return;
      qc.setQueryData(context.slotsKey, context.prev);
      syncCalendarSlotMeta(qc, owner, calendarId, context.prev);
    },
    onSuccess: (
      serverSlot: BuyerAppointmentSlot,
      { id }: { id: number },
      context: SlotMutationContext | undefined
    ) => {
      if (!context) return;
      const next = qc.getQueryData<BuyerAppointmentSlot[]>(context.slotsKey)?.map((s) =>
        s.id === id ? serverSlot : s
      );
      if (next) {
        qc.setQueryData(context.slotsKey, next);
        syncCalendarSlotMeta(qc, owner, calendarId, next);
      }
    },
  };
}

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

export function useDeleteBookingCalendar() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (id: number) => del(`/buyer/booking-calendars/${id}`),
    onSuccess: () => qc.invalidateQueries({ queryKey: ["buyer-booking-calendars"] }),
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
  return useMutation(
    createCalendarSlotMutations(qc, "buyer", calendarId, (body) =>
      post<BuyerAppointmentSlot>(`/buyer/booking-calendars/${calendarId}/slots`, body)
    )
  );
}

export function usePatchCalendarSlot(calendarId: number) {
  const qc = useQueryClient();
  return useMutation(
    patchCalendarSlotMutations(qc, "buyer", calendarId, ({ id, body }) =>
      patch<BuyerAppointmentSlot>(`/buyer/booking-calendars/${calendarId}/slots/${id}`, body)
    )
  );
}

export function useCopyCalendarSlots(calendarId: number) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (body: { from_weekday: number; to_weekdays: number[] }) =>
      post<{ items: BuyerAppointmentSlot[] }>(`/buyer/booking-calendars/${calendarId}/slots/copy`, body),
    onSuccess: (res) => {
      setCalendarSlotsCache(qc, "buyer", calendarId, res.items ?? []);
    },
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

export function useSetContractAppointmentCalendarSource() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: ({
      contractId,
      source,
      appointment_calendar_id,
    }: {
      contractId: number;
      source: "buyer" | "publisher";
      appointment_calendar_id?: number;
    }) =>
      patch(`/buyer/contracts/${contractId}/appointment-calendar-source`, {
        source,
        appointment_calendar_id,
      }),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ["buyer-contracts"] });
      qc.invalidateQueries({ queryKey: ["contract"] });
      qc.invalidateQueries({ queryKey: ["contract-appointment-slots"] });
      qc.invalidateQueries({ queryKey: ["contract-publisher-appointment-slots"] });
      qc.invalidateQueries({ queryKey: ["publisher-appointment-contracts"] });
    },
  });
}

export function useContractPublisherAppointmentSlots(contractId: number | null) {
  return useQuery({
    queryKey: ["contract-publisher-appointment-slots", contractId],
    queryFn: async () => {
      const res = await get<{ items: ContractPublisherAppointmentSlot[] }>(
        `/buyer/contracts/${contractId}/publisher-appointment-slots`
      );
      return res.items ?? [];
    },
    enabled: !!contractId,
  });
}

export function useSaveContractPublisherAppointmentSlots() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: ({
      contractId,
      slots,
    }: {
      contractId: number;
      slots: {
        publisher_slot_id: number;
        enabled: boolean;
        duration_min_override?: number | null;
        capacity_override?: number | null;
      }[];
    }) =>
      put<{ items: ContractPublisherAppointmentSlot[] }>(
        `/buyer/contracts/${contractId}/publisher-appointment-slots`,
        { slots }
      ),
    onSuccess: (_d, { contractId }) => {
      qc.invalidateQueries({ queryKey: ["contract-publisher-appointment-slots", contractId] });
    },
  });
}

export type CalendarOwner = "buyer" | "publisher";

export function usePublisherCalendars() {
  return useQuery({
    queryKey: ["publisher-booking-calendars"],
    queryFn: async () => {
      const res = await get<{ items: PublisherBookingCalendar[] }>("/publisher/booking-calendars");
      return res.items ?? [];
    },
  });
}

export function usePublisherBookingCalendar(calendarId: number | null) {
  return useQuery({
    queryKey: ["publisher-booking-calendar", calendarId],
    queryFn: () => get<PublisherBookingCalendar>(`/publisher/booking-calendars/${calendarId}`),
    enabled: !!calendarId,
  });
}

export function useCreatePublisherBookingCalendar() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (body: { name: string; timezone: string }) =>
      post<PublisherBookingCalendar>("/publisher/booking-calendars", body),
    onSuccess: () => qc.invalidateQueries({ queryKey: ["publisher-booking-calendars"] }),
  });
}

export function useSavePublisherBookingCalendar(calendarId: number) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (body: Partial<PublisherBookingCalendar>) =>
      put<PublisherBookingCalendar>(`/publisher/booking-calendars/${calendarId}`, body),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ["publisher-booking-calendar", calendarId] });
      qc.invalidateQueries({ queryKey: ["publisher-booking-calendars"] });
      qc.invalidateQueries({ queryKey: ["publisher-calendar-slots", calendarId] });
    },
  });
}

export function useDeletePublisherBookingCalendar() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (id: number) => del(`/publisher/booking-calendars/${id}`),
    onSuccess: () => qc.invalidateQueries({ queryKey: ["publisher-booking-calendars"] }),
  });
}

export function usePublisherCalendarSlots(calendarId: number | null) {
  return useQuery({
    queryKey: ["publisher-calendar-slots", calendarId],
    queryFn: async () => {
      const res = await get<{ items: PublisherAppointmentSlot[] }>(
        `/publisher/booking-calendars/${calendarId}/slots`
      );
      return res.items ?? [];
    },
    enabled: !!calendarId,
  });
}

export function useCreatePublisherCalendarSlot(calendarId: number) {
  const qc = useQueryClient();
  return useMutation(
    createCalendarSlotMutations(qc, "publisher", calendarId, (body) =>
      post<PublisherAppointmentSlot>(`/publisher/booking-calendars/${calendarId}/slots`, body)
    )
  );
}

export function usePatchPublisherCalendarSlot(calendarId: number) {
  const qc = useQueryClient();
  return useMutation(
    patchCalendarSlotMutations(qc, "publisher", calendarId, ({ id, body }) =>
      patch<PublisherAppointmentSlot>(`/publisher/booking-calendars/${calendarId}/slots/${id}`, body)
    )
  );
}

export function useCopyPublisherCalendarSlots(calendarId: number) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (body: { from_weekday: number; to_weekdays: number[] }) =>
      post<{ items: PublisherAppointmentSlot[] }>(
        `/publisher/booking-calendars/${calendarId}/slots/copy`,
        body
      ),
    onSuccess: (res) => {
      setCalendarSlotsCache(qc, "publisher", calendarId, res.items ?? []);
    },
  });
}

export function useSetContractPublisherAppointmentCalendar() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: ({
      contractId,
      publisher_appointment_calendar_id,
    }: {
      contractId: number;
      publisher_appointment_calendar_id: number;
    }) =>
      patch(`/publisher/contracts/${contractId}/appointment-calendar`, {
        publisher_appointment_calendar_id,
      }),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ["contracts"] });
      qc.invalidateQueries({ queryKey: ["contract"] });
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

export function useCalendarAppointmentFreeSlots(
  calendarId: number | null,
  date: string,
  owner: "publisher" | "buyer"
) {
  const base = owner === "publisher" ? "/publisher" : "/buyer";
  return useQuery({
    queryKey: ["calendar-appointment-free-slots", owner, calendarId, date],
    queryFn: async () => {
      const res = await get<AppointmentDaySlotsResult>(
        `${base}/appointments/slots?calendar_id=${calendarId}&date=${date}`
      );
      return {
        items: res.items ?? [],
        booked: res.booked ?? [],
        hours: res.hours ?? [],
        working_hours: res.working_hours ?? null,
      };
    },
    enabled: !!calendarId && !!date,
  });
}

export function useCalendarAppointmentMarkers(
  calendarId: number | null,
  from: string,
  to: string,
  owner: "publisher" | "buyer"
) {
  const base = owner === "publisher" ? "/publisher" : "/buyer";
  return useQuery({
    queryKey: ["calendar-appointment-markers", owner, calendarId, from, to],
    queryFn: async () => {
      const res = await get<{ items: AppointmentCalendarMarker[] }>(
        `${base}/appointments/calendar-markers?calendar_id=${calendarId}&from=${from}&to=${to}`
      );
      return res.items ?? [];
    },
    enabled: !!calendarId && !!from && !!to,
  });
}

export function useFreeSlots(contractId: number | null, date: string, bookingTarget = "own") {
  return useQuery({
    queryKey: ["appointment-free-slots", contractId, date, bookingTarget],
    queryFn: async () => {
      const res = await get<AppointmentDaySlotsResult>(
        `/publisher/appointments/slots?contract_id=${contractId}&date=${date}&booking_target=${bookingTarget}`
      );
      return {
        items: res.items ?? [],
        booked: res.booked ?? [],
        hours: res.hours ?? [],
        working_hours: res.working_hours ?? null,
      };
    },
    enabled: !!contractId && !!date,
  });
}

export function useCalendarMarkers(contractId: number | null, from: string, to: string, bookingTarget = "own") {
  return useQuery({
    queryKey: ["appointment-calendar-markers", contractId, from, to, bookingTarget],
    queryFn: async () => {
      const res = await get<{ items: AppointmentCalendarMarker[] }>(
        `/publisher/appointments/calendar-markers?contract_id=${contractId}&from=${from}&to=${to}&booking_target=${bookingTarget}`
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
      qc.invalidateQueries({ queryKey: ["calendar-appointment-free-slots"] });
      qc.invalidateQueries({ queryKey: ["appointment-calendar-markers"] });
      qc.invalidateQueries({ queryKey: ["calendar-appointment-markers"] });
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

export function useBuyerFreeSlots(contractId: number | null, date: string, bookingTarget = "own") {
  return useQuery({
    queryKey: ["buyer-appointment-free-slots", contractId, date, bookingTarget],
    queryFn: async () => {
      const res = await get<AppointmentDaySlotsResult>(
        `/buyer/appointments/slots?contract_id=${contractId}&date=${date}&booking_target=${bookingTarget}`
      );
      return {
        items: res.items ?? [],
        booked: res.booked ?? [],
        hours: res.hours ?? [],
        working_hours: res.working_hours ?? null,
      };
    },
    enabled: !!contractId && !!date,
  });
}

export function useBuyerAppointmentCalendarMarkers(
  contractId: number | null,
  from: string,
  to: string,
  bookingTarget = "own"
) {
  return useQuery({
    queryKey: ["buyer-appointment-calendar-markers", contractId, from, to, bookingTarget],
    queryFn: async () => {
      const res = await get<{ items: AppointmentCalendarMarker[] }>(
        `/buyer/appointments/calendar-markers?contract_id=${contractId}&from=${from}&to=${to}&booking_target=${bookingTarget}`
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
      qc.invalidateQueries({ queryKey: ["calendar-appointment-free-slots"] });
      qc.invalidateQueries({ queryKey: ["buyer-appointment-calendar-markers"] });
      qc.invalidateQueries({ queryKey: ["calendar-appointment-markers"] });
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

export function workingHoursForDate(
  apiHours: AppointmentDayWorkingHours | null | undefined,
  schedule: Record<string, { start: string; end: string }> | undefined,
  date: string
): AppointmentDayWorkingHours | null {
  if (apiHours?.start && apiHours?.end) return apiHours;
  if (!schedule) return null;
  const weekday = new Date(date + "T12:00:00").getDay();
  const day = schedule[WEEKDAY_KEYS[weekday]];
  if (!day?.start || !day?.end || day.start >= day.end) return null;
  return { start: day.start, end: day.end };
}
