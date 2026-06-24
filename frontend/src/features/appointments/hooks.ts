import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { get, patch, post, put } from "@/lib/api";
import type {
  AppointmentBooking,
  AppointmentCalendarMarker,
  AppointmentContractOption,
  AppointmentFreeSlot,
  BuyerAppointmentSlot,
  BuyerAvailability,
  ContractAppointmentSlot,
} from "@/types";

const WEEKDAYS = ["Sun", "Mon", "Tue", "Wed", "Thu", "Fri", "Sat"] as const;
export { WEEKDAYS };

export function useBuyerAvailability() {
  return useQuery({
    queryKey: ["buyer-availability"],
    queryFn: () => get<BuyerAvailability>("/buyer/availability"),
  });
}

export function useSaveBuyerAvailability() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (body: Partial<BuyerAvailability>) => put<BuyerAvailability>("/buyer/availability", body),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ["buyer-availability"] });
      qc.invalidateQueries({ queryKey: ["buyer-appointment-slots"] });
    },
  });
}

export function useBuyerAppointmentSlots() {
  return useQuery({
    queryKey: ["buyer-appointment-slots"],
    queryFn: async () => {
      const res = await get<{ items: BuyerAppointmentSlot[] }>("/buyer/appointment-slots");
      return res.items ?? [];
    },
  });
}

export function useCreateBuyerSlot() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (body: { weekday: number; start_time: string; duration_min: number; capacity: number }) =>
      post<BuyerAppointmentSlot>("/buyer/appointment-slots", body),
    onSuccess: () => qc.invalidateQueries({ queryKey: ["buyer-appointment-slots"] }),
  });
}

export function usePatchBuyerSlot() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: ({
      id,
      body,
    }: {
      id: number;
      body: { start_time?: string; duration_min?: number; capacity?: number; disabled?: boolean };
    }) => patch<BuyerAppointmentSlot>(`/buyer/appointment-slots/${id}`, body),
    onSuccess: () => qc.invalidateQueries({ queryKey: ["buyer-appointment-slots"] }),
  });
}

export function useCopyBuyerSlots() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (body: { from_weekday: number; to_weekdays: number[] }) =>
      post<{ items: BuyerAppointmentSlot[] }>("/buyer/appointment-slots/copy", body),
    onSuccess: () => qc.invalidateQueries({ queryKey: ["buyer-appointment-slots"] }),
  });
}

export function useBuyerBookings() {
  return useQuery({
    queryKey: ["buyer-appointments"],
    queryFn: async () => {
      const res = await get<{ items: AppointmentBooking[] }>("/buyer/appointments");
      return res.items ?? [];
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
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ["appointment-free-slots"] });
      qc.invalidateQueries({ queryKey: ["appointment-calendar-markers"] });
      qc.invalidateQueries({ queryKey: ["publisher-appointments"] });
      qc.invalidateQueries({ queryKey: ["buyer-appointments"] });
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

export const DEFAULT_WEEKLY_HOURS: Record<string, { start: string; end: string }> = {
  mon: { start: "09:00", end: "17:00" },
  tue: { start: "09:00", end: "17:00" },
  wed: { start: "09:00", end: "17:00" },
  thu: { start: "09:00", end: "17:00" },
  fri: { start: "09:00", end: "17:00" },
};

export const WEEKDAY_KEYS = ["sun", "mon", "tue", "wed", "thu", "fri", "sat"] as const;
