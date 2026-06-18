package notifications

const userPrefsKey = "notification_prefs"

// ChannelPrefs toggles delivery for one event.
type ChannelPrefs struct {
	InApp bool `json:"in_app"`
	Email bool `json:"email"`
}

// PrefsMap maps event type → channel toggles.
type PrefsMap map[string]ChannelPrefs

func defaultChannelPrefs() ChannelPrefs {
	return ChannelPrefs{InApp: true, Email: false}
}

func (m PrefsMap) forEvent(event string) ChannelPrefs {
	if m == nil {
		return defaultChannelPrefs()
	}
	if p, ok := m[event]; ok {
		return p
	}
	return defaultChannelPrefs()
}

func accountEvents(accountType string) []string {
	switch accountType {
	case "buyer":
		return []string{
			"dispute_update", "new_invoice", "collaboration_request",
			"partnership_request", "partnership_accepted",
			"contract_participation_pending", "contract_forked",
		}
	case "publisher":
		return []string{
			"collaboration_request", "partnership_request", "partnership_accepted",
			"contract_participation_accepted", "contract_participation_declined",
			"contract_counter_pending",
		}
	default:
		return nil
	}
}

func personalEvents() []string {
	return []string{"new_lead", "lead_returned"}
}

func isPersonalEvent(event string) bool {
	switch event {
	case "new_lead", "lead_returned":
		return true
	default:
		return false
	}
}

func fillDefaults(stored PrefsMap, events []string) PrefsMap {
	out := make(PrefsMap, len(events))
	for _, e := range events {
		out[e] = stored.forEvent(e)
	}
	return out
}

func mergePrefs(existing, patch PrefsMap) PrefsMap {
	out := make(PrefsMap, len(existing)+len(patch))
	for k, v := range existing {
		out[k] = v
	}
	for k, v := range patch {
		out[k] = v
	}
	return out
}
