package partnerships

import "time"

const (
	StatusActive            = "active"
	StatusPendingBuyer      = "pending_buyer"
	StatusPendingPublisher  = "pending_publisher"
	StatusRejected          = "rejected"
	StatusRevoked           = "revoked"
)

type Partnership struct {
	ID                int64
	PublisherID       int64
	BuyerID           int64
	Status            string
	RequestedBy       string
	RequestedByUserID *int64
	CreatedAt         time.Time
	UpdatedAt         time.Time
}

type ListItem struct {
	ID                 int64     `json:"id"`
	Status             string    `json:"status"`
	RequestedBy        string    `json:"requested_by"`
	PartnerName        string    `json:"partner_name"`
	PartnerHandlerID   string    `json:"partner_handler_id"`
	CreatedAt          time.Time `json:"created_at"`
}
