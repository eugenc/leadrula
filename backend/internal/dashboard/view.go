package dashboard

import "time"

type Goals struct {
	LeadTarget    *float64 `json:"lead_target,omitempty"`
	RevenueTarget *float64 `json:"revenue_target,omitempty"`
}

type View struct {
	ID          int64     `json:"id"`
	PublicID    string    `json:"public_id"`
	AccountID   int64     `json:"account_id"`
	OwnerUserID *int64    `json:"owner_user_id,omitempty"`
	Name        string    `json:"name"`
	Widgets     []string  `json:"widgets"`
	Period      string    `json:"period"`
	Goals       Goals     `json:"goals"`
	Shared      bool      `json:"shared"`
	Position    int       `json:"position"`
	CreatedBy   int64     `json:"created_by"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}
