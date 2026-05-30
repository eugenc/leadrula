package leads

import (
	"encoding/json"
	"time"
)

type FilterCondition struct {
	Field string          `json:"field"`
	Op    string          `json:"op"`
	Value json.RawMessage `json:"value,omitempty"`
}

type SavedView struct {
	ID          int64             `json:"id"`
	PublicID    string            `json:"public_id"`
	AccountID   int64             `json:"account_id"`
	OwnerUserID *int64            `json:"owner_user_id,omitempty"`
	Name        string            `json:"name"`
	Placement   string            `json:"placement"`
	Filters     []FilterCondition `json:"filters"`
	Columns     []string          `json:"columns,omitempty"`
	Sort        string            `json:"sort,omitempty"`
	SortDir     string            `json:"sort_dir,omitempty"`
	IsBuiltin   bool              `json:"is_builtin"`
	BuiltinKey  string            `json:"builtin_key,omitempty"`
	Shared      bool              `json:"shared"`
	Position    int               `json:"position"`
	CreatedBy   int64             `json:"created_by"`
	CreatedAt   time.Time         `json:"created_at"`
	UpdatedAt   time.Time         `json:"updated_at"`
}

var defaultListColumns = []string{
	"name", "phone", "source", "buyer", "assignee", "status", "action_at", "created_at",
}

var BuiltinViews = map[string]SavedView{
	"all": {
		PublicID:   "all",
		Name:       "All leads",
		Placement:  "both",
		Filters:    []FilterCondition{},
		Columns:    defaultListColumns,
		Sort:       "created_at",
		SortDir:    "desc",
		IsBuiltin:  true,
		BuiltinKey: "all",
	},
	"action_today": {
		PublicID:   "action_today",
		Name:       "Action date today",
		Placement:  "both",
		Filters:    []FilterCondition{{Field: "action_at", Op: "on", Value: json.RawMessage(`"today"`)}},
		Columns:    []string{"name", "assignee", "action_at", "status"},
		Sort:       "action_at",
		SortDir:    "asc",
		IsBuiltin:  true,
		BuiltinKey: "action_today",
	},
	"overdue": {
		PublicID:   "overdue",
		Name:       "Overdue actions",
		Placement:  "both",
		Filters:    []FilterCondition{{Field: "action_at", Op: "overdue"}},
		Columns:    []string{"name", "assignee", "action_at", "status"},
		Sort:       "action_at",
		SortDir:    "asc",
		IsBuiltin:  true,
		BuiltinKey: "overdue",
	},
}

func BuiltinView(key string) (SavedView, bool) {
	v, ok := BuiltinViews[key]
	return v, ok
}

func filterValueString(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	var s string
	if json.Unmarshal(raw, &s) == nil {
		return s
	}
	return string(raw)
}

func filterValueInt64(raw json.RawMessage) int64 {
	if len(raw) == 0 {
		return 0
	}
	var n int64
	if json.Unmarshal(raw, &n) == nil {
		return n
	}
	var f float64
	if json.Unmarshal(raw, &f) == nil {
		return int64(f)
	}
	return 0
}
