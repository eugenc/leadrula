package leads

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/echayko/leadrula/backend/internal/customfields"
	"github.com/echayko/leadrula/backend/internal/database"
	"github.com/echayko/leadrula/backend/pkg/httpx"
)

func actionAtText(v any) string {
	s := strings.TrimSpace(fmt.Sprint(v))
	if s == "" || s == "<nil>" {
		return ""
	}
	return s
}

// ParseActionAt parses inbound payload values into a lead action datetime.
func ParseActionAt(v any) (*time.Time, error) {
	s := actionAtText(v)
	if s == "" {
		return nil, nil
	}
	if t, ok := customfields.ParseFlexible("datetime", "RFC3339", s); ok {
		return &t, nil
	}
	return nil, httpx.Validation("invalid action_at value")
}

// ApplyMappedActionAt writes a parsed action datetime onto a lead.
func ApplyMappedActionAt(ctx context.Context, q database.Querier, repo *Repository, leadID int64, v any) error {
	t, err := ParseActionAt(v)
	if err != nil || t == nil {
		return err
	}
	return repo.SetActionAt(ctx, q, leadID, t)
}
