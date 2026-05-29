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
	CampaignName     *string         `json:"campaign_name"`
	PipelineID       *int64          `json:"pipeline_id"`
	StageID          *int64          `json:"stage_id"`
	Position         int             `json:"position"`
	AssignedUserID   *int64          `json:"assigned_user_id"`
	ActionAt         *time.Time      `json:"action_at"`
	Status           string          `json:"status"`
	DisqReasonID     *int64          `json:"disqualification_reason_id"`
	RawPayload       json.RawMessage `json:"raw_payload,omitempty"`
	CreatedAt        time.Time       `json:"created_at"`
	UpdatedAt        time.Time       `json:"updated_at"`
	CustomValues     map[string]json.RawMessage `json:"custom_values"`
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
