package providers

import (
	"errors"
	"strings"
)

// DeliverySkippedError marks an outbound delivery that intentionally did not run.
type DeliverySkippedError struct {
	Reason string
}

func (e *DeliverySkippedError) Error() string {
	if e == nil || e.Reason == "" {
		return "Skipped: delivery not sent"
	}
	return "Skipped: " + e.Reason
}

// IsDeliverySkipped reports whether err is a deliberate skip (not a transport failure).
func IsDeliverySkipped(err error) bool {
	var skipped *DeliverySkippedError
	if errors.As(err, &skipped) {
		return true
	}
	return strings.HasPrefix(err.Error(), "Skipped:")
}
