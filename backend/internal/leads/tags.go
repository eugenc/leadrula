package leads

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/echayko/leadrula/backend/internal/database"
)

// ParseTagsFromValue extracts tag strings from inbound payload values (string, comma-separated, or JSON array).
func ParseTagsFromValue(v any) []string {
	switch x := v.(type) {
	case string:
		var out []string
		for _, part := range strings.Split(x, ",") {
			if t := strings.TrimSpace(part); t != "" {
				out = append(out, t)
			}
		}
		return out
	case []string:
		return parseTagStrings(x)
	case []any:
		var out []string
		for _, item := range x {
			out = append(out, ParseTagsFromValue(item)...)
		}
		return normalizeTags(out)
	default:
		if b, err := json.Marshal(v); err == nil {
			var arr []string
			if err := json.Unmarshal(b, &arr); err == nil {
				return parseTagStrings(arr)
			}
			var s string
			if err := json.Unmarshal(b, &s); err == nil {
				return ParseTagsFromValue(s)
			}
		}
		s := strings.TrimSpace(fmt.Sprint(v))
		if s == "" || s == "<nil>" {
			return nil
		}
		return ParseTagsFromValue(s)
	}
}

func parseTagStrings(in []string) []string {
	var out []string
	for _, t := range in {
		if t = strings.TrimSpace(t); t != "" {
			out = append(out, t)
		}
	}
	return normalizeTags(out)
}

// ApplyMappedField applies a mapped builtin value, including money fields and tags.
func ApplyMappedField(ctx context.Context, q database.Querier, repo *Repository, accountID, leadID int64, field string, v any) error {
	if field == "tags" {
		return ApplyMappedTags(ctx, q, repo, accountID, leadID, v)
	}
	return ApplyMappedBuiltin(ctx, q, repo, leadID, field, v)
}

// ApplyMappedTags writes parsed tags onto a lead (replaces the full tag list).
func ApplyMappedTags(ctx context.Context, q database.Querier, repo *Repository, accountID, leadID int64, v any) error {
	tags := ParseTagsFromValue(v)
	if len(tags) == 0 {
		return nil
	}
	return repo.setTags(ctx, q, accountID, leadID, tags)
}
