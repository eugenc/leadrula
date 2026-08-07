package appointments

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/echayko/leadrula/backend/internal/accounts"
)

func TestPutPublisherBookingCalendarSave(t *testing.T) {
	pool := connectAppointmentsTestDB(t)
	ctx := context.Background()
	var publisherID int64
	err := pool.QueryRow(ctx, `SELECT account_id FROM publisher_booking_calendars WHERE id=1`).Scan(&publisherID)
	if err != nil {
		t.Skip("no publisher calendar id=1")
	}
	svc := NewService(pool, nil, accounts.NewRepository(pool), nil)
	sched := json.RawMessage(`{"mon":{"start":"09:00","end":"17:00"},"tue":{"start":"09:00","end":"17:00"}}`)
	_, err = svc.PutPublisherBookingCalendar(ctx, publisherID, 1, PutCalendarParams{
		Schedule: sched, Timezone: "America/New_York", BufferMin: 30,
	})
	if err != nil {
		t.Fatal(err)
	}
}
