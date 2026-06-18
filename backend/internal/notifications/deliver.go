package notifications

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"

	"github.com/echayko/leadrula/backend/internal/database"
	"github.com/jackc/pgx/v5"
)

// DeliverParams describes one notification event.
type DeliverParams struct {
	AccountID int64
	UserIDs   []int64
	EventType string
	Payload   map[string]any
}

// EmailJob is queued during Deliver and sent after the caller commits.
type EmailJob struct {
	To, Subject, Body string
}

// Deliver inserts in-app notifications inside the caller transaction and returns
// email jobs to send after commit.
func (s *Service) Deliver(ctx context.Context, q database.Querier, p DeliverParams) ([]EmailJob, error) {
	if len(p.UserIDs) == 0 {
		return nil, nil
	}
	raw, err := json.Marshal(p.Payload)
	if err != nil {
		return nil, err
	}

	personal := isPersonalEvent(p.EventType)
	var accountPrefs PrefsMap
	if !personal {
		accountPrefs, err = s.loadAccountPrefs(ctx, q, p.AccountID)
		if err != nil {
			return nil, err
		}
	}

	accountType, err := s.accounts.AccountType(ctx, q, p.AccountID)
	if err != nil {
		return nil, err
	}

	var emails []EmailJob
	for _, uid := range p.UserIDs {
		var userExists bool
		if err := q.QueryRow(ctx,
			`SELECT EXISTS(SELECT 1 FROM users WHERE id = $1)`, uid).Scan(&userExists); err != nil {
			return nil, err
		}
		if !userExists {
			log.Printf("notification skip missing user id=%d event=%s", uid, p.EventType)
			continue
		}

		ch := accountPrefs.forEvent(p.EventType)
		if personal {
			userPrefs, err := s.loadUserPrefs(ctx, q, uid)
			if err != nil {
				if errors.Is(err, pgx.ErrNoRows) {
					log.Printf("notification skip missing user prefs id=%d event=%s", uid, p.EventType)
					continue
				}
				return nil, err
			}
			ch = userPrefs.forEvent(p.EventType)
		}
		if ch.InApp {
			if _, err := q.Exec(ctx,
				`INSERT INTO notifications(user_id, type, payload) VALUES ($1,$2,$3)`,
				uid, p.EventType, raw); err != nil {
				return nil, err
			}
		}
		if ch.Email && s.email != nil {
			var email, name string
			if err := q.QueryRow(ctx,
				`SELECT email, full_name FROM users WHERE id = $1`, uid).Scan(&email, &name); err != nil {
				if errors.Is(err, pgx.ErrNoRows) {
					continue
				}
				return nil, err
			}
			if email == "" {
				continue
			}
			label := eventLabel(p.EventType, p.Payload)
			link := eventLink(s.baseURL, accountType, p.EventType)
			body := notificationEmail(s.baseURL, name, label, link)
			emails = append(emails, EmailJob{
				To:      email,
				Subject: "LeadRula: " + label,
				Body:    body,
			})
		}
	}
	return emails, nil
}

func (s *Service) SendEmails(jobs []EmailJob) {
	for _, j := range jobs {
		if s.email == nil {
			continue
		}
		if err := s.email.send(j.To, j.Subject, j.Body); err != nil {
			log.Printf("notification email failed to=%s: %v", j.To, err)
		}
	}
}

func (s *Service) loadAccountPrefs(ctx context.Context, q database.Querier, accountID int64) (PrefsMap, error) {
	raw, err := s.accounts.GetNotificationPrefs(ctx, q, accountID)
	if err != nil {
		return nil, err
	}
	return prefsFromRaw(raw), nil
}

func (s *Service) loadUserPrefs(ctx context.Context, q database.Querier, userID int64) (PrefsMap, error) {
	var raw []byte
	if err := q.QueryRow(ctx, `SELECT prefs FROM users WHERE id = $1`, userID).Scan(&raw); err != nil {
		return nil, err
	}
	return userNotifPrefsFromRaw(raw), nil
}

func prefsFromRaw(raw []byte) PrefsMap {
	m := PrefsMap{}
	if len(raw) > 0 {
		_ = json.Unmarshal(raw, &m)
	}
	return m
}

func userNotifPrefsFromRaw(raw []byte) PrefsMap {
	if len(raw) == 0 {
		return PrefsMap{}
	}
	root := map[string]json.RawMessage{}
	if json.Unmarshal(raw, &root) != nil {
		return PrefsMap{}
	}
	nested, ok := root[userPrefsKey]
	if !ok {
		return PrefsMap{}
	}
	return prefsFromRaw(nested)
}

func eventLabel(eventType string, payload map[string]any) string {
	switch eventType {
	case "new_lead":
		return "New lead received"
	case "lead_returned":
		return "A lead was returned"
	case "dispute_update":
		if payload["outcome"] == "accepted" {
			return "Dispute accepted"
		}
		if payload["outcome"] == "rejected" {
			return "Dispute rejected"
		}
		return "Dispute resolved"
	case "new_invoice":
		if amount, ok := payload["amount"].(float64); ok {
			return fmt.Sprintf("New invoice for $%.2f", amount)
		}
		return "New invoice received"
	case "collaboration_request":
		if payload["direction"] == "publisher_to_buyer" {
			return fmt.Sprintf("%v requested collaboration", payload["publisher_name"])
		}
		return fmt.Sprintf("%v invited you to collaborate", payload["buyer_name"])
	case "partnership_request":
		if payload["direction"] == "publisher_to_buyer" {
			return fmt.Sprintf("%v requested a partnership", payload["publisher_name"])
		}
		return fmt.Sprintf("%v requested a partnership", payload["buyer_name"])
	case "partnership_accepted":
		if payload["accepted_by"] == "publisher" {
			return fmt.Sprintf("%v accepted your partnership request", payload["publisher_name"])
		}
		return fmt.Sprintf("%v accepted your partnership request", payload["buyer_name"])
	case "contract_participation_pending":
		return "New contract invitation"
	case "contract_participation_accepted":
		return "Buyer accepted contract"
	case "contract_participation_declined":
		return "Buyer declined contract"
	case "contract_counter_pending":
		return "Buyer submitted a counter-offer"
	case "contract_forked":
		return "Counter-offer accepted — review new contract"
	default:
		return eventType
	}
}

func eventLink(baseURL, accountType, eventType string) string {
	prefix := "/b"
	if accountType == "publisher" {
		prefix = "/p"
	}
	switch eventType {
	case "new_lead", "lead_returned":
		return baseURL + prefix + "/leads"
	case "dispute_update", "new_invoice":
		return baseURL + "/b/billing"
	case "collaboration_request":
		return baseURL + prefix + "/collaboration"
	case "partnership_request", "partnership_accepted":
		if accountType == "buyer" {
			return baseURL + "/b/publishers"
		}
		return baseURL + "/p/buyers"
	case "contract_participation_pending", "contract_forked":
		return baseURL + "/b/contract"
	case "contract_participation_accepted", "contract_participation_declined", "contract_counter_pending":
		return baseURL + "/p/contracts"
	default:
		return baseURL
	}
}

// AssigneeIDs returns user IDs for a lead assignee, or nil when unset.
func AssigneeIDs(assignedUserID *int64) []int64 {
	if assignedUserID == nil || *assignedUserID == 0 {
		return nil
	}
	return []int64{*assignedUserID}
}
