package leads

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/echayko/leadrula/backend/internal/auth"
)

var builtinFieldLabels = map[string]string{
	"first_name": "First name",
	"last_name":  "Last name",
	"phone":      "Phone",
	"email":      "Email",
	"address":    "Address",
	"city":       "City",
	"state":      "State",
	"zip":        "Zip",
	"country":    "Country",
	"address_place_id": "Address place",
	"source":     "Source",
}

type leadUpdateInput struct {
	Fields               map[string]*string
	AssignedUserID       *int64
	ClearAssignee        bool
	PreassignedBuyerID   *int64
	ClearPreassignedBuyer bool
	CustomValues         map[string]json.RawMessage
	Tags                 *[]string
}

func assigneeLabel(name *string) string {
	if name == nil || *name == "" {
		return "Unassigned"
	}
	return *name
}

func tagsLabel(tags []string) string {
	if len(tags) == 0 {
		return "None"
	}
	return strings.Join(tags, ", ")
}

func customValueLabel(raw json.RawMessage) string {
	if len(raw) == 0 {
		return "None"
	}
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		if s == "" {
			return "None"
		}
		return s
	}
	return string(raw)
}

func formatActionAt(t *time.Time) string {
	if t == nil {
		return "None"
	}
	return t.Format("1/2/2006, 3:04:05 PM")
}

func leadBuiltinValue(l *Lead, field string) string {
	switch field {
	case "first_name":
		return l.FirstName
	case "last_name":
		return l.LastName
	case "phone":
		return ptrStr(l.Phone)
	case "email":
		return ptrStr(l.Email)
	case "address":
		return ptrStr(l.Address)
	case "city":
		return ptrStr(l.City)
	case "state":
		return ptrStr(l.State)
	case "zip":
		return ptrStr(l.Zip)
	case "country":
		return ptrStr(l.Country)
	case "source":
		return ptrStr(l.Source)
	default:
		return ""
	}
}

func ptrStr(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func displayValue(s string) string {
	if s == "" {
		return "None"
	}
	return s
}

func stageChange(fromName, toName string) []auth.ImpersonationChange {
	from := fromName
	if from == "" {
		from = "None"
	}
	return []auth.ImpersonationChange{{Field: "Stage", From: from, To: toName}}
}

func preassignedBuyerLabel(name *string) string {
	if name == nil || *name == "" {
		return "None"
	}
	return *name
}

func diffLeadUpdate(before *Lead, after *Lead, in leadUpdateInput, fieldNames map[string]string) []auth.ImpersonationChange {
	var changes []auth.ImpersonationChange

	for field, newVal := range in.Fields {
		if newVal == nil {
			continue
		}
		old := leadBuiltinValue(before, field)
		if old == *newVal {
			continue
		}
		label := builtinFieldLabels[field]
		if label == "" {
			label = field
		}
		changes = append(changes, auth.ImpersonationChange{
			Field: label,
			From:  displayValue(old),
			To:    displayValue(*newVal),
		})
	}

	if in.ClearAssignee {
		from := assigneeLabel(before.AssigneeName)
		if from != "Unassigned" {
			changes = append(changes, auth.ImpersonationChange{
				Field: "Assignee", From: from, To: "Unassigned",
			})
		}
	} else if in.AssignedUserID != nil {
		from := assigneeLabel(before.AssigneeName)
		to := assigneeLabel(after.AssigneeName)
		if from != to {
			changes = append(changes, auth.ImpersonationChange{
				Field: "Assignee", From: from, To: to,
			})
		}
	}

	if in.ClearPreassignedBuyer {
		from := preassignedBuyerLabel(before.PreassignedBuyerName)
		if from != "None" {
			changes = append(changes, auth.ImpersonationChange{
				Field: "Pre-assigned buyer", From: from, To: "None",
			})
		}
	} else if in.PreassignedBuyerID != nil {
		from := preassignedBuyerLabel(before.PreassignedBuyerName)
		to := preassignedBuyerLabel(after.PreassignedBuyerName)
		if from != to {
			changes = append(changes, auth.ImpersonationChange{
				Field: "Pre-assigned buyer", From: from, To: to,
			})
		}
	}

	if in.Tags != nil {
		from := tagsLabel(before.Tags)
		to := tagsLabel(after.Tags)
		if from != to {
			changes = append(changes, auth.ImpersonationChange{
				Field: "Tags", From: from, To: to,
			})
		}
	}

	for fidStr, newRaw := range in.CustomValues {
		name := fieldNames[fidStr]
		if name == "" {
			name = fmt.Sprintf("Field %s", fidStr)
		}
		oldRaw, ok := before.CustomValues[fidStr]
		if !ok {
			oldRaw = json.RawMessage("null")
		}
		from := customValueLabel(oldRaw)
		to := customValueLabel(newRaw)
		if from != to {
			changes = append(changes, auth.ImpersonationChange{
				Field: name, From: from, To: to,
			})
		}
	}

	return changes
}

func actionAtChange(from, to *time.Time) []auth.ImpersonationChange {
	fromStr := formatActionAt(from)
	toStr := formatActionAt(to)
	if fromStr == toStr {
		return nil
	}
	return []auth.ImpersonationChange{{
		Field: "Action Date & Time", From: fromStr, To: toStr,
	}}
}

func bulkAssignChange(userName string, count int) []auth.ImpersonationChange {
	to := userName
	if to == "" {
		to = "User"
	}
	if count > 1 {
		to = fmt.Sprintf("%s (%d leads)", to, count)
	}
	return []auth.ImpersonationChange{{
		Field: "Assignee", From: "(various)", To: to,
	}}
}
