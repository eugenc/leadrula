package leads

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/echayko/leadrula/backend/pkg/httpx"
)

type FilterContext struct {
	UserID int64
	TZ     string
}

func todayInTimezone(tz string) string {
	loc, err := time.LoadLocation(tz)
	if err != nil {
		loc = time.UTC
	}
	return time.Now().In(loc).Format("2006-01-02")
}

func resolveUserID(raw json.RawMessage, ctx FilterContext) (int64, error) {
	s := filterValueString(raw)
	if s == "me" {
		return ctx.UserID, nil
	}
	if s == "" {
		return filterValueInt64(raw), nil
	}
	n, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return 0, httpx.Validation("invalid user id in filter")
	}
	return n, nil
}

func resolveDate(raw json.RawMessage, ctx FilterContext) (string, error) {
	s := filterValueString(raw)
	if s == "today" {
		return todayInTimezone(ctx.TZ), nil
	}
	if s == "" {
		return "", httpx.Validation("date value required")
	}
	if _, err := time.Parse("2006-01-02", s); err != nil {
		return "", httpx.Validation("invalid date format, use YYYY-MM-DD")
	}
	return s, nil
}

func resolveText(raw json.RawMessage) string {
	return strings.TrimSpace(filterValueString(raw))
}

// appendCompiledFilters adds AND clauses for each condition onto where/args.
func appendCompiledFilters(where string, args []any, conditions []FilterCondition, ctx FilterContext) (string, []any, error) {
	add := func(cond string, val any) {
		args = append(args, val)
		where += fmt.Sprintf(" AND %s $%d", cond, len(args))
	}
	addRaw := func(fragment string) {
		where += " AND " + fragment
	}

	for _, c := range conditions {
		field := c.Field
		op := c.Op
		switch field {
		case "assigned_user_id":
			switch op {
			case "equals":
				uid, err := resolveUserID(c.Value, ctx)
				if err != nil {
					return "", nil, err
				}
				add("l.assigned_user_id =", uid)
			case "not_equals":
				uid, err := resolveUserID(c.Value, ctx)
				if err != nil {
					return "", nil, err
				}
				args = append(args, uid)
				where += fmt.Sprintf(" AND (l.assigned_user_id IS NULL OR l.assigned_user_id != $%d)", len(args))
			case "is_empty":
				addRaw("l.assigned_user_id IS NULL")
			case "is_not_empty":
				addRaw("l.assigned_user_id IS NOT NULL")
			default:
				return "", nil, httpx.Validation("unsupported operator for assigned_user_id: " + op)
			}
		case "action_at":
			switch op {
			case "on":
				date, err := resolveDate(c.Value, ctx)
				if err != nil {
					return "", nil, err
				}
				tz := ctx.TZ
				if tz == "" {
					tz = "UTC"
				}
				args = append(args, date, tz)
				dateArg := len(args) - 1
				tzArg := len(args)
				where += fmt.Sprintf(
					" AND l.action_at >= ($%d::date AT TIME ZONE $%d) AND l.action_at < (($%d::date + interval '1 day') AT TIME ZONE $%d)",
					dateArg, tzArg, dateArg, tzArg,
				)
			case "before":
				date, err := resolveDate(c.Value, ctx)
				if err != nil {
					return "", nil, err
				}
				tz := ctx.TZ
				if tz == "" {
					tz = "UTC"
				}
				args = append(args, date, tz)
				where += fmt.Sprintf(" AND l.action_at < ($%d::date AT TIME ZONE $%d)", len(args)-1, len(args))
			case "after":
				date, err := resolveDate(c.Value, ctx)
				if err != nil {
					return "", nil, err
				}
				tz := ctx.TZ
				if tz == "" {
					tz = "UTC"
				}
				args = append(args, date, tz)
				where += fmt.Sprintf(" AND l.action_at >= (($%d::date + interval '1 day') AT TIME ZONE $%d)", len(args)-1, len(args))
			case "is_empty":
				addRaw("l.action_at IS NULL")
			case "is_not_empty":
				addRaw("l.action_at IS NOT NULL")
			case "overdue":
				addRaw("l.action_at IS NOT NULL AND l.action_at < now()")
			default:
				return "", nil, httpx.Validation("unsupported operator for action_at: " + op)
			}
		case "status":
			switch op {
			case "equals":
				v := resolveText(c.Value)
				if v == "" {
					return "", nil, httpx.Validation("status value required")
				}
				add("l.status =", v)
			case "not_equals":
				v := resolveText(c.Value)
				if v == "" {
					return "", nil, httpx.Validation("status value required")
				}
				add("l.status !=", v)
			default:
				return "", nil, httpx.Validation("unsupported operator for status: " + op)
			}
		case "pipeline_id", "stage_id":
			if op != "equals" {
				return "", nil, httpx.Validation("unsupported operator for " + field + ": " + op)
			}
			n := filterValueInt64(c.Value)
			if n == 0 {
				s := filterValueString(c.Value)
				if s != "" {
					var err error
					n, err = strconv.ParseInt(s, 10, 64)
					if err != nil || n == 0 {
						return "", nil, httpx.Validation(field + " value required")
					}
				} else {
					return "", nil, httpx.Validation(field + " value required")
				}
			}
			add("l."+field+" =", n)
		case "campaign_name":
			v := resolveText(c.Value)
			if v == "" {
				return "", nil, httpx.Validation("campaign_name value required")
			}
			switch op {
			case "equals":
				add("l.campaign_name =", v)
			case "contains":
				args = append(args, "%"+v+"%")
				where += fmt.Sprintf(" AND l.campaign_name ILIKE $%d", len(args))
			default:
				return "", nil, httpx.Validation("unsupported operator for campaign_name: " + op)
			}
		case "tags":
			if op != "contains" {
				return "", nil, httpx.Validation("unsupported operator for tags: " + op)
			}
			v := resolveText(c.Value)
			if v == "" {
				return "", nil, httpx.Validation("tag value required")
			}
			args = append(args, v)
			where += fmt.Sprintf(" AND $%d = ANY(l.tags)", len(args))
		case "buyer_name":
			v := resolveText(c.Value)
			if v == "" {
				return "", nil, httpx.Validation("buyer_name value required")
			}
			switch op {
			case "equals":
				add("ba.name =", v)
			case "contains":
				args = append(args, "%"+v+"%")
				where += fmt.Sprintf(" AND ba.name ILIKE $%d", len(args))
			default:
				return "", nil, httpx.Validation("unsupported operator for buyer_name: " + op)
			}
		default:
			return "", nil, httpx.Validation("unsupported filter field: " + field)
		}
	}
	return where, args, nil
}

func flatFiltersToConditions(f ListFilters) []FilterCondition {
	var out []FilterCondition
	if f.Status != "" {
		out = append(out, FilterCondition{Field: "status", Op: "equals", Value: mustJSON(f.Status)})
	}
	if f.Campaign != "" {
		out = append(out, FilterCondition{Field: "campaign_name", Op: "equals", Value: mustJSON(f.Campaign)})
	}
	if f.PipelineID != 0 {
		out = append(out, FilterCondition{Field: "pipeline_id", Op: "equals", Value: mustJSON(f.PipelineID)})
	}
	if f.StageID != 0 {
		out = append(out, FilterCondition{Field: "stage_id", Op: "equals", Value: mustJSON(f.StageID)})
	}
	if f.Assigned != 0 {
		out = append(out, FilterCondition{Field: "assigned_user_id", Op: "equals", Value: mustJSON(f.Assigned)})
	}
	if f.Tag != "" {
		out = append(out, FilterCondition{Field: "tags", Op: "contains", Value: mustJSON(f.Tag)})
	}
	if f.ActionOverdue {
		out = append(out, FilterCondition{Field: "action_at", Op: "overdue"})
	}
	if f.ActionOn != "" {
		val := f.ActionOn
		if val != "" {
			out = append(out, FilterCondition{Field: "action_at", Op: "on", Value: mustJSON(val)})
		}
	}
	return out
}

func mustJSON(v any) json.RawMessage {
	b, _ := json.Marshal(v)
	return b
}
