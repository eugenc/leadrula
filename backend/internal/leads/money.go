package leads

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/echayko/leadrula/backend/internal/database"
)

var moneyBuiltinFields = map[string]bool{
	"cost": true, "revenue": true,
}

func IsMoneyBuiltin(field string) bool {
	return moneyBuiltinFields[field]
}

func IsBuiltinField(field string) bool {
	return builtinFields[field] || moneyBuiltinFields[field]
}

// ParseMoney converts payload/import values to a dollar amount.
func ParseMoney(v any) *float64 {
	switch x := v.(type) {
	case float64:
		f := x
		return &f
	case float32:
		f := float64(x)
		return &f
	case int:
		f := float64(x)
		return &f
	case int64:
		f := float64(x)
		return &f
	case json.Number:
		f, err := x.Float64()
		if err != nil {
			return nil
		}
		return &f
	case string:
		return ParseMoneyString(x)
	default:
		if s := fmt.Sprint(v); s != "" && s != "<nil>" {
			return ParseMoneyString(s)
		}
		return nil
	}
}

func ParseMoneyString(s string) *float64 {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	s = strings.TrimPrefix(s, "$")
	s = strings.ReplaceAll(s, ",", "")
	f, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return nil
	}
	return &f
}

func (r *Repository) SetMoneyField(ctx context.Context, q database.Querier, leadID int64, field string, amount *float64) error {
	if !moneyBuiltinFields[field] {
		return fmt.Errorf("unknown money builtin field %q", field)
	}
	if amount == nil {
		_, err := q.Exec(ctx, fmt.Sprintf(`UPDATE leads SET %s = NULL WHERE id = $1`, field), leadID)
		return err
	}
	_, err := q.Exec(ctx, fmt.Sprintf(`UPDATE leads SET %s = $2 WHERE id = $1`, field), leadID, *amount)
	return err
}

func ApplyMappedBuiltin(ctx context.Context, q database.Querier, repo *Repository, leadID int64, field string, v any) error {
	if IsMoneyBuiltin(field) {
		amount := ParseMoney(v)
		if amount == nil {
			return nil
		}
		return repo.SetMoneyField(ctx, q, leadID, field, amount)
	}
	str := strings.TrimSpace(fmt.Sprint(v))
	if str == "" || str == "<nil>" {
		return nil
	}
	return repo.SetBuiltinField(ctx, q, leadID, field, str)
}
