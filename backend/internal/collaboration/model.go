package collaboration

import "time"

const (
	StatusActive            = "active"
	StatusRevoked           = "revoked"
	StatusPendingBuyer      = "pending_buyer"
	StatusPendingPublisher  = "pending_publisher"
)

type Collaboration struct {
	ID                      int64      `json:"id"`
	PublisherID             int64      `json:"-"`
	BuyerID                 int64      `json:"-"`
	Status                  string     `json:"status"`
	Version                 int64      `json:"version"`
	AutoGranted             bool       `json:"auto_granted"`
	TargetPublisherUserID   *int64     `json:"-"`
	RequestedByUserID       *int64     `json:"-"`
	RevokedAt               *time.Time `json:"revoked_at,omitempty"`
	CreatedAt               time.Time  `json:"created_at"`
	UpdatedAt               time.Time  `json:"updated_at"`
}

type AuditEntry struct {
	ID          int64          `json:"id"`
	EventType   string         `json:"event_type"`
	ActorUserID *int64         `json:"actor_user_id,omitempty"`
	ActorName   string         `json:"actor_name,omitempty"`
	Metadata    map[string]any `json:"metadata"`
	CreatedAt   time.Time      `json:"created_at"`
}

type StatusResponse struct {
	Status                  string       `json:"status"`
	Version                 int64        `json:"version,omitempty"`
	AutoGranted             bool         `json:"auto_granted,omitempty"`
	PublisherName           string       `json:"publisher_name,omitempty"`
	BuyerName               string       `json:"buyer_name,omitempty"`
	BuyerID                 string       `json:"buyer_id,omitempty"`
	TargetPublisherUserName string       `json:"target_publisher_user_name,omitempty"`
	RequestedByName         string       `json:"requested_by_name,omitempty"`
	CreatedAt               *time.Time   `json:"created_at,omitempty"`
	RevokedAt               *time.Time   `json:"revoked_at,omitempty"`
	AuditLog                []AuditEntry `json:"audit_log,omitempty"`
}

type BuyerCollabSummary struct {
	BuyerID   int64  `json:"buyer_id"`
	Status    string `json:"status"`
	Version   int64  `json:"version"`
}
