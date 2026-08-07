import { SectionLabel } from "@/components/layout/SectionLabel";
import { BuyerContractCalendarSection } from "@/features/appointments/BuyerContractCalendarSection";
import { BuyerPublisherSlotSection } from "@/features/appointments/BuyerPublisherSlotSection";
import type { Contract } from "@/types";

export function BuyerContractDeliveryCalendarSection({ contract }: { contract: Contract }) {
  const contractAccepted = contract.status === "active";
  const hasPublisherCalendar = (contract.publisher_appointment_calendar_id ?? 0) > 0;

  return (
    <div className="mt-4 space-y-4 border-t border-gray-100 pt-4">
      <BuyerContractCalendarSection
        contractId={contract.id}
        appointmentCalendarId={contract.appointment_calendar_id}
      />

      <div className="space-y-2">
        <SectionLabel>Publisher calendar</SectionLabel>
        {contractAccepted ? (
          hasPublisherCalendar ? (
            <>
              <p className="text-xs text-gray-400">
                The publisher attached a calendar. You can book on it from Appointments after accepting this contract.
              </p>
              <BuyerPublisherSlotSection contractId={contract.id} />
            </>
          ) : (
            <p className="text-sm text-gray-500">Publisher has not attached a calendar yet.</p>
          )
        ) : (
          <p className="text-sm text-gray-500">Available after you accept this contract.</p>
        )}
      </div>
    </div>
  );
}
