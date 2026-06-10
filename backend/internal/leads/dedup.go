package leads

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"

	"github.com/echayko/leadrula/backend/internal/database"
	"github.com/echayko/leadrula/backend/pkg/httpx"
	"github.com/jackc/pgx/v5"
)

var nonDigits = regexp.MustCompile(`\D`)

func NormalizePhone(s string) string {
	return nonDigits.ReplaceAllString(strings.TrimSpace(s), "")
}

func NormalizeEmail(s string) string {
	return strings.ToLower(strings.TrimSpace(s))
}

// FindDuplicate returns an existing lead owned by ownerAccountID matching phone OR email.
func FindDuplicate(ctx context.Context, q database.Querier, ownerAccountID int64, phone, email *string, excludeLeadID int64) (int64, bool, error) {
	var phoneNorm, emailNorm string
	if phone != nil {
		phoneNorm = NormalizePhone(*phone)
	}
	if email != nil {
		emailNorm = NormalizeEmail(*email)
	}
	if phoneNorm == "" && emailNorm == "" {
		return 0, false, nil
	}

	args := []any{ownerAccountID, excludeLeadID}
	where := "owner_account_id = $1 AND deleted_at IS NULL AND id <> $2 AND ("
	var parts []string
	if phoneNorm != "" {
		args = append(args, phoneNorm)
		parts = append(parts, fmt.Sprintf(
			`(phone IS NOT NULL AND regexp_replace(phone, '[^0-9]', '', 'g') = $%d)`, len(args)))
	}
	if emailNorm != "" {
		args = append(args, emailNorm)
		parts = append(parts, fmt.Sprintf(`(email IS NOT NULL AND lower(trim(email)) = $%d)`, len(args)))
	}
	where += strings.Join(parts, " OR ") + ")"

	var id int64
	err := q.QueryRow(ctx, `SELECT id FROM leads WHERE `+where+` LIMIT 1`, args...).Scan(&id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return 0, false, nil
		}
		return 0, false, err
	}
	return id, true, nil
}

func CheckDuplicate(ctx context.Context, q database.Querier, ownerAccountID int64, phone, email *string, excludeLeadID int64) error {
	if _, found, err := FindDuplicate(ctx, q, ownerAccountID, phone, email, excludeLeadID); err != nil {
		return err
	} else if found {
		return httpx.Conflict("lead already exists")
	}
	return nil
}
