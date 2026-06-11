package accounts

import (
	"strings"

	"github.com/echayko/leadrula/backend/pkg/httpx"
	"github.com/echayko/leadrula/backend/pkg/validate"
)

func trimStringPtr(p **string) {
	if p != nil && *p != nil {
		s := strings.TrimSpace(**p)
		*p = &s
	}
}

func validateContactEmailPtr(p **string) error {
	if p == nil || *p == nil {
		return nil
	}
	s := **p
	if s != "" && !validate.Email(s) {
		return httpx.Validation("invalid contact email")
	}
	return nil
}

func normalizeBusinessProfile(
	contactEmail, phone, addressLine1, addressLine2, city, state, postalCode, country **string,
) error {
	trimStringPtr(contactEmail)
	if err := validateContactEmailPtr(contactEmail); err != nil {
		return err
	}
	trimStringPtr(phone)
	trimStringPtr(addressLine1)
	trimStringPtr(addressLine2)
	trimStringPtr(city)
	trimStringPtr(state)
	trimStringPtr(postalCode)
	trimStringPtr(country)
	return nil
}
