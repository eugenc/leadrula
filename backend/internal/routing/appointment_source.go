package routing

import (
	"context"

	"github.com/echayko/leadrula/backend/pkg/httpx"
)

type appointmentColumns struct {
	contractID          any
	calendarID          any
	deliveryMode        any
	publisherPipelineID any
	publisherStageID    any
	phoneMatchMode      any
}

func (s *Service) resolveAppointmentColumns(ctx context.Context, publisherID int64, sourceType string, appt *AppointmentSourceParams) (appointmentColumns, error) {
	if sourceType != "appointment" {
		return appointmentColumns{
			deliveryMode:   "contract",
			phoneMatchMode: "update_and_book",
		}, nil
	}
	return s.validateAppointmentParams(ctx, publisherID, appt)
}

func (s *Service) resolveAppointmentUpdate(ctx context.Context, publisherID int64, old *Source, appt *AppointmentSourceParams) (appointmentColumns, error) {
	merged := &AppointmentSourceParams{
		ContractID:          old.ContractID,
		CalendarID:          old.CalendarID,
		DeliveryMode:        strPtr(old.DeliveryMode),
		PublisherPipelineID: old.PublisherPipelineID,
		PublisherStageID:    old.PublisherStageID,
		PhoneMatchMode:      strPtr(old.PhoneMatchMode),
	}
	if appt.ContractID != nil {
		if *appt.ContractID == 0 {
			merged.ContractID = nil
		} else {
			merged.ContractID = appt.ContractID
		}
	}
	if appt.CalendarID != nil {
		if *appt.CalendarID == 0 {
			merged.CalendarID = nil
		} else {
			merged.CalendarID = appt.CalendarID
		}
	}
	if appt.DeliveryMode != nil {
		merged.DeliveryMode = appt.DeliveryMode
	}
	if appt.PublisherPipelineID != nil {
		if *appt.PublisherPipelineID == 0 {
			merged.PublisherPipelineID = nil
		} else {
			merged.PublisherPipelineID = appt.PublisherPipelineID
		}
	}
	if appt.PublisherStageID != nil {
		if *appt.PublisherStageID == 0 {
			merged.PublisherStageID = nil
		} else {
			merged.PublisherStageID = appt.PublisherStageID
		}
	}
	if appt.PhoneMatchMode != nil {
		merged.PhoneMatchMode = appt.PhoneMatchMode
	}
	return s.validateAppointmentParams(ctx, publisherID, merged)
}

func (s *Service) validateAppointmentParams(ctx context.Context, publisherID int64, appt *AppointmentSourceParams) (appointmentColumns, error) {
	deliveryMode := "contract"
	if appt.DeliveryMode != nil && *appt.DeliveryMode != "" {
		deliveryMode = *appt.DeliveryMode
	}
	switch deliveryMode {
	case "contract", "publisher_pipeline", "publisher":
	default:
		return appointmentColumns{}, httpx.Validation("delivery_mode must be contract, publisher_pipeline, or publisher")
	}

	phoneMatchMode := "update_and_book"
	if appt.PhoneMatchMode != nil && *appt.PhoneMatchMode != "" {
		phoneMatchMode = *appt.PhoneMatchMode
	}
	switch phoneMatchMode {
	case "update_and_book", "book_existing", "reject_duplicate":
	default:
		return appointmentColumns{}, httpx.Validation("phone_match_mode must be update_and_book, book_existing, or reject_duplicate")
	}

	var contractID, calendarID, pipelineID, stageID any

	switch deliveryMode {
	case "contract":
		if appt.ContractID == nil || *appt.ContractID == 0 {
			return appointmentColumns{}, httpx.Validation("contract_id is required for contract delivery")
		}
		if appt.CalendarID != nil && *appt.CalendarID != 0 {
			return appointmentColumns{}, httpx.Validation("calendar_id is not allowed for contract delivery")
		}
		if appt.PublisherPipelineID != nil && *appt.PublisherPipelineID != 0 {
			return appointmentColumns{}, httpx.Validation("publisher pipeline is not allowed for contract delivery")
		}
		if appt.PublisherStageID != nil && *appt.PublisherStageID != 0 {
			return appointmentColumns{}, httpx.Validation("publisher stage is not allowed for contract delivery")
		}
		var owned bool
		err := s.pool.QueryRow(ctx,
			`SELECT EXISTS(
				SELECT 1 FROM contracts
				WHERE id=$1 AND publisher_id=$2 AND lead_type='Appointment'
				  AND status='active' AND deleted_at IS NULL AND buyer_id IS NOT NULL)`,
			*appt.ContractID, publisherID).Scan(&owned)
		if err != nil {
			return appointmentColumns{}, err
		}
		if !owned {
			return appointmentColumns{}, httpx.Validation("appointment contract not found")
		}
		ok, err := s.appointmentContractCalendarConfigured(ctx, *appt.ContractID)
		if err != nil {
			return appointmentColumns{}, err
		}
		if !ok {
			return appointmentColumns{}, httpx.Validation("appointment calendar is not configured for this contract")
		}
		contractID = *appt.ContractID

	case "publisher", "publisher_pipeline":
		if appt.CalendarID == nil || *appt.CalendarID == 0 {
			return appointmentColumns{}, httpx.Validation("calendar_id is required for publisher delivery")
		}
		if appt.ContractID != nil && *appt.ContractID != 0 {
			return appointmentColumns{}, httpx.Validation("contract_id is not allowed for publisher delivery")
		}
		ok, err := s.publisherCalendarConfigured(ctx, publisherID, *appt.CalendarID)
		if err != nil {
			return appointmentColumns{}, err
		}
		if !ok {
			return appointmentColumns{}, httpx.Validation("appointment calendar is not configured")
		}
		calendarID = *appt.CalendarID

		if deliveryMode == "publisher_pipeline" {
			if appt.PublisherPipelineID == nil || *appt.PublisherPipelineID == 0 ||
				appt.PublisherStageID == nil || *appt.PublisherStageID == 0 {
				return appointmentColumns{}, httpx.Validation("publisher pipeline and stage required")
			}
			var pipelineOK bool
			err = s.pool.QueryRow(ctx,
				`SELECT EXISTS(
					SELECT 1 FROM pipelines p
					JOIN pipeline_stages ps ON ps.pipeline_id = p.id AND ps.id = $3
					WHERE p.id = $2 AND p.account_id = $1)`,
				publisherID, *appt.PublisherPipelineID, *appt.PublisherStageID).Scan(&pipelineOK)
			if err != nil {
				return appointmentColumns{}, err
			}
			if !pipelineOK {
				return appointmentColumns{}, httpx.Validation("publisher pipeline or stage not found")
			}
			pipelineID = *appt.PublisherPipelineID
			stageID = *appt.PublisherStageID
		} else {
			if appt.PublisherPipelineID != nil && *appt.PublisherPipelineID != 0 {
				return appointmentColumns{}, httpx.Validation("publisher pipeline is not allowed for inbox delivery")
			}
			if appt.PublisherStageID != nil && *appt.PublisherStageID != 0 {
				return appointmentColumns{}, httpx.Validation("publisher stage is not allowed for inbox delivery")
			}
		}
	}

	return appointmentColumns{
		contractID:          contractID,
		calendarID:          calendarID,
		deliveryMode:        deliveryMode,
		publisherPipelineID: pipelineID,
		publisherStageID:    stageID,
		phoneMatchMode:      phoneMatchMode,
	}, nil
}

func strPtr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func (s *Service) appointmentContractCalendarConfigured(ctx context.Context, contractID int64) (bool, error) {
	var ok bool
	err := s.pool.QueryRow(ctx,
		`SELECT CASE
		   WHEN c.appointment_calendar_source = 'buyer' THEN
		     bc.id IS NOT NULL AND bc.schedule::text NOT IN ('{}', 'null')
		     AND EXISTS(SELECT 1 FROM buyer_appointment_slots sl
		                WHERE sl.calendar_id = bc.id AND sl.disabled_at IS NULL)
		   WHEN c.appointment_calendar_source = 'publisher' THEN
		     pc.id IS NOT NULL AND pc.schedule::text NOT IN ('{}', 'null')
		     AND EXISTS(SELECT 1 FROM publisher_appointment_slots sl
		                WHERE sl.calendar_id = pc.id AND sl.disabled_at IS NULL)
		   ELSE false
		 END
		 FROM contracts c
		 LEFT JOIN buyer_booking_calendars bc ON bc.id = c.appointment_calendar_id
		 LEFT JOIN publisher_booking_calendars pc ON pc.id = c.publisher_appointment_calendar_id
		 WHERE c.id = $1`, contractID).Scan(&ok)
	return ok, err
}

func (s *Service) publisherCalendarConfigured(ctx context.Context, publisherID, calendarID int64) (bool, error) {
	var ok bool
	err := s.pool.QueryRow(ctx,
		`SELECT EXISTS(
			SELECT 1 FROM publisher_booking_calendars c
			WHERE c.id = $2 AND c.account_id = $1
			  AND c.schedule::text NOT IN ('{}', 'null')
			  AND EXISTS(
			    SELECT 1 FROM publisher_appointment_slots sl
			    WHERE sl.calendar_id = c.id AND sl.disabled_at IS NULL))`,
		publisherID, calendarID).Scan(&ok)
	return ok, err
}
