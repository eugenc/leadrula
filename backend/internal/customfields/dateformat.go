package customfields

import (
	"encoding/json"
	"strings"
	"time"

	"github.com/echayko/leadrula/backend/pkg/httpx"
)

var dateFormats = map[string]string{
	"yyyy-MM-DD":  "2006-01-02",
	"MM/dd/yyyy":  "01/02/2006",
	"dd/MM/yyyy":  "02/01/2006",
	"MMM d, yyyy": "Jan 2, 2006",
}

var datetimeFormats = map[string]string{
	"yyyy-MM-DDTHH:mm":  "2006-01-02T15:04",
	"yyyy-MM-DD HH:mm":  "2006-01-02 15:04",
	"RFC3339":           time.RFC3339,
	"MM/dd/yyyy h:mm a": "01/02/2006 3:04 PM",
}

var parseFallbackLayouts = []string{
	time.RFC3339,
	time.RFC3339Nano,
	"2006-01-02T15:04:05",
	"2006-01-02T15:04",
	"2006-01-02 15:04",
	"2006-01-02",
	"01/02/2006",
	"02/01/2006",
	"01/02/2006 3:04 PM",
	"Jan 2, 2006",
}

// ValidFormat reports whether format is an allowed preset for the field type.
func ValidFormat(ftype, format string) bool {
	switch ftype {
	case "date":
		_, ok := dateFormats[format]
		return ok
	case "datetime":
		_, ok := datetimeFormats[format]
		return ok
	default:
		return false
	}
}

// DefaultFormat returns the preset token for date/datetime types.
func DefaultFormat(ftype string) string {
	switch ftype {
	case "date":
		return "yyyy-MM-DD"
	case "datetime":
		return "yyyy-MM-DDTHH:mm"
	default:
		return ""
	}
}

// EffectiveFormat returns the stored format or the type default.
func EffectiveFormat(f CustomField) string {
	if f.Format != nil && *f.Format != "" {
		return *f.Format
	}
	return DefaultFormat(f.Type)
}

func goLayout(ftype, token string) string {
	if token == "RFC3339" {
		return time.RFC3339
	}
	switch ftype {
	case "date":
		return dateFormats[token]
	case "datetime":
		return datetimeFormats[token]
	default:
		return ""
	}
}

// ParseFlexible tries the configured format first, then known fallbacks.
func ParseFlexible(ftype, formatToken, input string) (time.Time, bool) {
	input = strings.TrimSpace(input)
	if input == "" {
		return time.Time{}, false
	}

	seen := map[string]bool{}
	try := func(layout string) (time.Time, bool) {
		if layout == "" || seen[layout] {
			return time.Time{}, false
		}
		seen[layout] = true
		if t, err := time.Parse(layout, input); err == nil {
			return t, true
		}
		return time.Time{}, false
	}

	if t, ok := try(goLayout(ftype, formatToken)); ok {
		return t, true
	}
	for _, layout := range parseFallbackLayouts {
		if t, ok := try(layout); ok {
			return t, true
		}
	}
	return time.Time{}, false
}

// FormatTime formats t using the configured preset token.
func FormatTime(ftype, formatToken string, t time.Time) string {
	layout := goLayout(ftype, formatToken)
	if layout == "" {
		layout = goLayout(ftype, DefaultFormat(ftype))
	}
	return t.Format(layout)
}

func resolveFormat(ftype string, format *string) (*string, error) {
	if ftype != "date" && ftype != "datetime" {
		return nil, nil
	}
	if format != nil && *format != "" {
		if !ValidFormat(ftype, *format) {
			return nil, httpx.Validation("invalid date format")
		}
		return format, nil
	}
	d := DefaultFormat(ftype)
	return &d, nil
}

// NormalizeValue parses a date/datetime custom value and stores it in the field format.
func NormalizeValue(f CustomField, raw json.RawMessage) (json.RawMessage, error) {
	if f.Type != "date" && f.Type != "datetime" {
		return raw, nil
	}
	if len(raw) == 0 || string(raw) == "null" {
		return raw, nil
	}
	var s string
	if err := json.Unmarshal(raw, &s); err != nil {
		return raw, nil
	}
	s = strings.TrimSpace(s)
	if s == "" {
		return json.Marshal("")
	}
	formatToken := EffectiveFormat(f)
	t, ok := ParseFlexible(f.Type, formatToken, s)
	if !ok {
		return raw, nil
	}
	formatted := FormatTime(f.Type, formatToken, t)
	return json.Marshal(formatted)
}
