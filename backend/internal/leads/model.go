package leads

import (
	"encoding/json"
	"time"
)

type Lead struct {
	ID               int64           `json:"id"`
	PublicID         string          `json:"public_id"`
	OwnerAccountID   int64           `json:"owner_account_id"`
	PublisherID      int64           `json:"publisher_id"`
	ContractID       *int64          `json:"contract_id"`
	FirstName        string          `json:"first_name"`
	LastName         string          `json:"last_name"`
	Phone            *string         `json:"phone"`
	Email            *string         `json:"email"`
	Address          *string         `json:"address"`
	City             *string         `json:"city"`
	State            *string         `json:"state"`
	Zip              *string         `json:"zip"`
	Country          *string         `json:"country"`
	AddressPlaceID   *string         `json:"address_place_id"`
	Source           *string         `json:"source"`
	ExternalID       *string         `json:"external_id"`
	PipelineID           *int64 `json:"pipeline_id"`
	StageID              *int64 `json:"stage_id"`
	PublisherPipelineID  *int64 `json:"publisher_pipeline_id,omitempty"`
	PublisherStageID     *int64 `json:"publisher_stage_id,omitempty"`
	BoardStageID         *int64 `json:"board_stage_id,omitempty"`
	Position             int    `json:"position"`
	AssignedUserID      *int64 `json:"assigned_user_id"`
	PreassignedBuyerID  *int64 `json:"preassigned_buyer_id,omitempty"`
	ActionAt            *time.Time `json:"action_at"`
	Status           string          `json:"status"`
	DisqReasonID     *int64          `json:"disqualification_reason_id"`
	RawPayload       json.RawMessage `json:"raw_payload,omitempty"`
	CreatedAt        time.Time       `json:"created_at"`
	UpdatedAt        time.Time       `json:"updated_at"`
	DeletedAt        *time.Time      `json:"deleted_at,omitempty"`
	CustomValues     map[string]json.RawMessage `json:"custom_values"`
	BuyerName              *string `json:"buyer_name,omitempty"`
	PreassignedBuyerName   *string `json:"preassigned_buyer_name,omitempty"`
	SourceName        *string                   `json:"source_name,omitempty"`
	AssigneeName      *string                   `json:"assignee_name,omitempty"`
	AssigneeAvatarURL *string                   `json:"assignee_avatar_url,omitempty"`
	PipelineName      *string                   `json:"pipeline_name,omitempty"`
	StageName         *string                   `json:"stage_name,omitempty"`
	StageEnteredAt    time.Time                 `json:"stage_entered_at"`
	Tags              []string                  `json:"tags"`
	Cost              *float64                  `json:"cost,omitempty"`
	Revenue           *float64                  `json:"revenue,omitempty"`
	GrossProfit       *float64                  `json:"gross_profit,omitempty"`
	NetProfit         *float64                  `json:"net_profit,omitempty"`
	PurchasePrice     *float64                  `json:"purchase_price,omitempty"`
}

type ListResult struct {
	Items []Lead `json:"items"`
	Total int    `json:"total"`
	Page  int    `json:"page"`
	Limit int    `json:"limit"`
}

type Note struct {
	ID         int64     `json:"id"`
	LeadID     int64     `json:"lead_id"`
	UserID     *int64    `json:"user_id"`
	AuthorName string    `json:"author_name"`
	Body       string    `json:"body"`
	CreatedAt  time.Time `json:"created_at"`
}

type StageHistoryEntry struct {
	ID            int64      `json:"id"`
	FromStageID   *int64     `json:"from_stage_id"`
	FromStageName *string    `json:"from_stage_name"`
	ToStageID     int64      `json:"to_stage_id"`
	ToStageName   *string    `json:"to_stage_name"`
	MovedByName   *string    `json:"moved_by_name"`
	ActionAt      *time.Time `json:"action_at_captured"`
	DisqReason    *string    `json:"disqualification_reason"`
	CreatedAt     time.Time  `json:"created_at"`
}

type LeadHistoryEntry struct {
	ID        int64     `json:"id"`
	Kind      string    `json:"kind"`
	CreatedAt time.Time `json:"created_at"`

	ActorType   string  `json:"actor_type,omitempty"`
	ActorName   string  `json:"actor_name,omitempty"`
	ActorDetail string  `json:"actor_detail,omitempty"`
	Status      string  `json:"status,omitempty"`
	Summary     string  `json:"summary,omitempty"`
	Amount      *float64 `json:"amount,omitempty"`

	FromStageName *string    `json:"from_stage_name,omitempty"`
	ToStageName   *string    `json:"to_stage_name,omitempty"`
	MovedByName   *string    `json:"moved_by_name,omitempty"`
	ActionAt      *time.Time `json:"action_at_captured,omitempty"`
	DisqReason    *string    `json:"disqualification_reason,omitempty"`
	AccountName   *string    `json:"account_name,omitempty"`
	AccountType   *string    `json:"account_type,omitempty"`

	TransferKind    *string `json:"transfer_kind,omitempty"`
	FromAccountName *string `json:"from_account_name,omitempty"`
	ToAccountName   *string `json:"to_account_name,omitempty"`
	TriggerLabel    *string `json:"trigger_label,omitempty"`

	FieldName  *string `json:"field_name,omitempty"`
	FromValue  *string `json:"from_value,omitempty"`
	ToValue    *string `json:"to_value,omitempty"`
	PipelineName *string `json:"pipeline_name,omitempty"`
	StageName    *string `json:"stage_name,omitempty"`
	RouteName    *string `json:"route_name,omitempty"`

	ownerAccountID int64 `json:"-"`
	fromAccountID  int64 `json:"-"`
	toAccountID    int64 `json:"-"`
	buyerAccountID int64 `json:"-"`
}
