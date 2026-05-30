package accounts

import "time"

type Account struct {
	ID        int64     `json:"-"`
	PublicID  string    `json:"id"`
	Type      string    `json:"type"`
	Name      string    `json:"name"`
	Website   string    `json:"website"`
	Timezone  string    `json:"timezone"`
	CreatedAt time.Time `json:"created_at"`
}

type User struct {
	ID          int64     `json:"id"`
	PublicID    string    `json:"public_id"`
	AccountID   int64     `json:"-"`
	Email       string    `json:"email"`
	FullName    string    `json:"full_name"`
	Role        string    `json:"role"`
	IsActive    bool      `json:"is_active"`
	Prefs       []byte    `json:"-"`
	LastLoginAt *time.Time `json:"last_login_at,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
}

type Invite struct {
	ID        int64     `json:"-"`
	AccountID int64     `json:"-"`
	Email     string    `json:"email"`
	FullName  string    `json:"full_name"`
	Role      string    `json:"role"`
	Token     string    `json:"-"`
	ExpiresAt time.Time `json:"expires_at"`
	CreatedAt time.Time `json:"created_at"`
}

// UserListItem is a member or pending invite for the admin users table.
type UserListItem struct {
	ID        int64   `json:"id"`
	InviteID  int64   `json:"invite_id"`
	PublicID  string  `json:"public_id,omitempty"`
	Email     string  `json:"email"`
	FullName  string  `json:"full_name"`
	Role      string  `json:"role"`
	Status    string  `json:"status"`
	AvatarURL *string `json:"avatar_url,omitempty"`
}

type UpdateUserParams struct {
	Role     *string
	FullName *string
	Email    *string
	IsActive *bool
}

type UpdateInviteParams struct {
	FullName *string
	Email    *string
	Role     *string
}

type CreateBuyerParams struct {
	Name            string
	Website         string
	Timezone        string
	AdminEmail      string
	AdminFirstName  string
	AdminLastName   string
	StartingBalance float64
}

type CreateBuyerResult struct {
	Buyer       BuyerSummary
	InviteToken string
	AdminEmail  string
}
