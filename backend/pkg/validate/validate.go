// Package validate holds small input-validation helpers.
package validate

import (
	"net/mail"
	"strings"
)

// Email reports whether s is a syntactically valid email address.
func Email(s string) bool {
	_, err := mail.ParseAddress(s)
	return err == nil
}

// NonEmpty reports whether s has non-whitespace content.
func NonEmpty(s string) bool {
	return strings.TrimSpace(s) != ""
}
