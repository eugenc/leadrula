package accounts

import "time"

type Account struct {
	ID        int64     `json:"-"`
	PublicID  string    `json:"id"`
	Type      string    `json:"type"`
	Name      string    `json:"name"`
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
	Role      string    `json:"role"`
	Token     string    `json:"-"`
	ExpiresAt time.Time `json:"expires_at"`
	CreatedAt time.Time `json:"created_at"`
}
