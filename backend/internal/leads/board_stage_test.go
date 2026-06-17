package leads

import (
	"strings"
	"testing"
)

func TestBoardStageSQL(t *testing.T) {
	buyer := boardStageSQL("buyer")
	if buyer != "l.stage_id" {
		t.Fatalf("buyer boardStageSQL = %q want l.stage_id", buyer)
	}

	pub := boardStageSQL("publisher")
	if !strings.Contains(pub, "publisher_stage_id") {
		t.Fatalf("publisher boardStageSQL should use publisher_stage_id: %q", pub)
	}
	if !strings.Contains(pub, "owner_account_id <> l.publisher_id") {
		t.Fatalf("publisher boardStageSQL should check owner vs publisher: %q", pub)
	}

	platform := boardStageSQL("platform")
	if platform != "l.stage_id" {
		t.Fatalf("platform boardStageSQL = %q want l.stage_id", platform)
	}
}

func TestListOrderBy_boardStageId(t *testing.T) {
	buyer := listOrderBy("buyer", "board_stage_id", "asc")
	if !strings.HasPrefix(buyer, "l.stage_id ASC") {
		t.Fatalf("buyer board_stage_id order = %q", buyer)
	}

	pub := listOrderBy("publisher", "board_stage_id", "desc")
	if !strings.Contains(pub, "publisher_stage_id") {
		t.Fatalf("publisher board_stage_id order = %q", pub)
	}
	if !strings.Contains(pub, "DESC") {
		t.Fatalf("publisher board_stage_id order should be DESC: %q", pub)
	}
}

func int64Ptr(v int64) *int64 { return &v }

// Mirrors frontend computeBoardStageId for unit tests.
func computeBoardStageID(accountType string, ownerID, publisherID int64, stageID, pubStageID *int64) *int64 {
	if accountType == "publisher" && pubStageID != nil && ownerID != publisherID {
		return pubStageID
	}
	return stageID
}

func TestComputeBoardStageID(t *testing.T) {
	stageID := int64(100)
	pubStageID := int64(200)
	ownerID := int64(1)
	pubID := int64(2)

	tests := []struct {
		name        string
		accountType string
		ownerID     int64
		publisherID int64
		stageID     *int64
		pubStageID  *int64
		want        *int64
	}{
		{
			name:        "buyer distributed lead uses buyer stage",
			accountType: "buyer",
			ownerID:     ownerID,
			publisherID: pubID,
			stageID:     &stageID,
			pubStageID:  &pubStageID,
			want:        &stageID,
		},
		{
			name:        "publisher distributed lead uses publisher mirror stage",
			accountType: "publisher",
			ownerID:     ownerID,
			publisherID: pubID,
			stageID:     &stageID,
			pubStageID:  &pubStageID,
			want:        &pubStageID,
		},
		{
			name:        "publisher owned lead uses stage_id",
			accountType: "publisher",
			ownerID:     pubID,
			publisherID: pubID,
			stageID:     &stageID,
			pubStageID:  &pubStageID,
			want:        &stageID,
		},
		{
			name:        "buyer with no stage",
			accountType: "buyer",
			ownerID:     ownerID,
			publisherID: pubID,
			stageID:     nil,
			pubStageID:  &pubStageID,
			want:        nil,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := computeBoardStageID(tc.accountType, tc.ownerID, tc.publisherID, tc.stageID, tc.pubStageID)
			if (got == nil) != (tc.want == nil) {
				t.Fatalf("got %v want %v", got, tc.want)
			}
			if got != nil && tc.want != nil && *got != *tc.want {
				t.Fatalf("got %d want %d", *got, *tc.want)
			}
		})
	}
}
