package accounts

// accountMeFields returns the account slice included in GET /auth/me and PATCH /auth/me/account responses.
func accountMeFields(a *Account) map[string]any {
	return map[string]any{
		"id":             a.PublicID,
		"handler_id":     a.HandlerID,
		"type":           a.Type,
		"name":           a.Name,
		"website":        a.Website,
		"timezone":       a.Timezone,
		"contact_email":  a.ContactEmail,
		"phone":          a.Phone,
		"address_line1":  a.AddressLine1,
		"address_line2":  a.AddressLine2,
		"city":           a.City,
		"state":          a.State,
		"postal_code":    a.PostalCode,
		"country":        a.Country,
	}
}
