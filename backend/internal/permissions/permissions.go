package permissions

import (
	"encoding/json"
)

const (
	LeadScopeAll                 = "all"
	LeadScopeAssigned            = "assigned"
	LeadScopeFollowed            = "followed"
	LeadScopeAssignedAndFollowed = "assigned_and_followed"

	ActionSettingsAdmin      = "settings_admin"
	ActionPipelinesRouting   = "pipelines_routing"
	ActionContractsPartners  = "contracts_partners"
	ActionBilling            = "billing"
	ActionAppointmentsManage = "appointments_manage"
)

const (
	NavDashboard     = "dashboard"
	NavLeads         = "leads"
	NavFields        = "fields"
	NavAppointments  = "appointments"
	NavCalendars     = "calendars"
	NavCalls         = "calls"
	NavBoard         = "board"
	NavPipelines     = "pipelines"
	NavBuyers        = "buyers"
	NavPublishers    = "publishers"
	NavContracts     = "contracts"
	NavCollaboration = "collaboration"
	NavSources       = "sources"
	NavWebhooks      = "webhooks"
	NavRouting       = "routing"
	NavLogs          = "logs"
	NavRoutes        = "routes"
	NavSettings      = "settings"
	NavBilling       = "billing"
	NavIntegrations  = "integrations"
)

var allActionKeys = []string{
	ActionSettingsAdmin, ActionPipelinesRouting, ActionContractsPartners,
	ActionBilling, ActionAppointmentsManage,
}

type Actions struct {
	SettingsAdmin      bool `json:"settings_admin"`
	PipelinesRouting   bool `json:"pipelines_routing"`
	ContractsPartners  bool `json:"contracts_partners"`
	Billing            bool `json:"billing"`
	AppointmentsManage bool `json:"appointments_manage"`
}

type Overrides struct {
	Nav       map[string]bool `json:"nav,omitempty"`
	LeadScope string          `json:"lead_scope,omitempty"`
	Actions   map[string]bool `json:"actions,omitempty"`
}

type Effective struct {
	Nav       map[string]bool `json:"nav"`
	LeadScope string          `json:"lead_scope"`
	Actions   Actions         `json:"actions"`
}

func (e Effective) CanNav(key string) bool {
	return e.Nav != nil && e.Nav[key]
}

func (e Effective) CanAction(key string) bool {
	switch key {
	case ActionSettingsAdmin:
		return e.Actions.SettingsAdmin
	case ActionPipelinesRouting:
		return e.Actions.PipelinesRouting
	case ActionContractsPartners:
		return e.Actions.ContractsPartners
	case ActionBilling:
		return e.Actions.Billing
	case ActionAppointmentsManage:
		return e.Actions.AppointmentsManage
	default:
		return false
	}
}

func (e Effective) IsFullAdmin() bool {
	if e.LeadScope != LeadScopeAll {
		return false
	}
	if !e.Actions.SettingsAdmin || !e.Actions.PipelinesRouting || !e.Actions.ContractsPartners ||
		!e.Actions.Billing || !e.Actions.AppointmentsManage {
		return false
	}
	for _, v := range e.Nav {
		if !v {
			return false
		}
	}
	return true
}

func ParseOverrides(raw []byte) Overrides {
	if len(raw) == 0 {
		return Overrides{}
	}
	var o Overrides
	_ = json.Unmarshal(raw, &o)
	return o
}

func MarshalOverrides(o Overrides) ([]byte, error) {
	if o.LeadScope == "" && len(o.Nav) == 0 && len(o.Actions) == 0 {
		return []byte("{}"), nil
	}
	return json.Marshal(o)
}

func FullAccess(accountType string) Effective {
	return presetForRole("admin", accountType)
}

func Resolve(role, accountType string, raw []byte) Effective {
	return merge(presetForRole(role, accountType), ParseOverrides(raw))
}

func PresetForRole(role, accountType string) Effective {
	return presetForRole(role, accountType)
}

func Delta(role, accountType string, effective Effective) Overrides {
	preset := presetForRole(role, accountType)
	var o Overrides
	if effective.LeadScope != preset.LeadScope {
		o.LeadScope = effective.LeadScope
	}
	if nav := diffBoolMap(preset.Nav, effective.Nav); len(nav) > 0 {
		o.Nav = nav
	}
	if actions := diffActionMaps(preset.Actions, effective.Actions); len(actions) > 0 {
		o.Actions = actions
	}
	return o
}

func merge(preset Effective, o Overrides) Effective {
	out := preset
	if o.LeadScope != "" {
		out.LeadScope = o.LeadScope
	}
	if len(o.Nav) > 0 {
		out.Nav = applyBoolMap(out.Nav, o.Nav)
	}
	if len(o.Actions) > 0 {
		out.Actions = applyActions(out.Actions, o.Actions)
	}
	return out
}

func presetForRole(role, accountType string) Effective {
	nav := defaultNavForAccount(accountType)
	actions := Actions{}
	leadScope := LeadScopeAssigned

	switch role {
	case "admin":
		leadScope = LeadScopeAll
		actions = Actions{
			SettingsAdmin: true, PipelinesRouting: true, ContractsPartners: true,
			Billing: true, AppointmentsManage: true,
		}
		for k := range nav {
			nav[k] = true
		}
	case "follower":
		leadScope = LeadScopeFollowed
		for k := range nav {
			if !adminOnlyNav(k, accountType) {
				nav[k] = true
			}
		}
	default:
		for k := range nav {
			if !adminOnlyNav(k, accountType) {
				nav[k] = true
			}
		}
	}
	return Effective{Nav: nav, LeadScope: leadScope, Actions: actions}
}

func defaultNavForAccount(accountType string) map[string]bool {
	keys := []string{
		NavDashboard, NavLeads, NavFields, NavAppointments, NavCalendars, NavCalls,
		NavBoard, NavPipelines, NavContracts, NavCollaboration, NavWebhooks,
		NavSettings, NavBilling, NavIntegrations,
	}
	switch accountType {
	case "publisher":
		keys = append(keys, NavBuyers, NavSources, NavRouting, NavLogs)
	case "buyer":
		keys = append(keys, NavPublishers, NavRoutes, NavLogs)
	}
	nav := make(map[string]bool, len(keys))
	for _, k := range keys {
		nav[k] = false
	}
	return nav
}

func adminOnlyNav(key, accountType string) bool {
	switch key {
	case NavFields, NavPipelines, NavCollaboration, NavWebhooks, NavIntegrations:
		return true
	case NavBuyers, NavSources, NavRouting:
		return accountType == "publisher"
	case NavLogs:
		return true
	default:
		return false
	}
}

func applyBoolMap(base, patch map[string]bool) map[string]bool {
	out := make(map[string]bool, len(base))
	for k, v := range base {
		out[k] = v
	}
	for k, v := range patch {
		if _, ok := out[k]; ok {
			out[k] = v
		}
	}
	return out
}

func diffBoolMap(preset, effective map[string]bool) map[string]bool {
	out := map[string]bool{}
	for k, pv := range preset {
		if ev, ok := effective[k]; ok && ev != pv {
			out[k] = ev
		}
	}
	return out
}

func actionsToMap(a Actions) map[string]bool {
	return map[string]bool{
		ActionSettingsAdmin:      a.SettingsAdmin,
		ActionPipelinesRouting:   a.PipelinesRouting,
		ActionContractsPartners:  a.ContractsPartners,
		ActionBilling:            a.Billing,
		ActionAppointmentsManage: a.AppointmentsManage,
	}
}

func mapToActions(m map[string]bool) Actions {
	return Actions{
		SettingsAdmin:      m[ActionSettingsAdmin],
		PipelinesRouting:   m[ActionPipelinesRouting],
		ContractsPartners:  m[ActionContractsPartners],
		Billing:            m[ActionBilling],
		AppointmentsManage: m[ActionAppointmentsManage],
	}
}

func applyActions(base Actions, patch map[string]bool) Actions {
	m := actionsToMap(base)
	for k, v := range patch {
		if _, ok := m[k]; ok {
			m[k] = v
		}
	}
	return mapToActions(m)
}

func diffActionMaps(preset Actions, effective Actions) map[string]bool {
	pm := actionsToMap(preset)
	em := actionsToMap(effective)
	return diffBoolMap(pm, em)
}

func HasAssignedScope(scope string) bool {
	return scope == LeadScopeAll || scope == LeadScopeAssigned || scope == LeadScopeAssignedAndFollowed
}

func HasFollowedScope(scope string) bool {
	return scope == LeadScopeFollowed || scope == LeadScopeAssignedAndFollowed
}

func IsFollowedOnly(scope string) bool {
	return scope == LeadScopeFollowed
}

func ValidateOverrides(accountType string, o Overrides) error {
	if o.LeadScope != "" && o.LeadScope != LeadScopeAll && o.LeadScope != LeadScopeAssigned &&
		o.LeadScope != LeadScopeFollowed && o.LeadScope != LeadScopeAssignedAndFollowed {
		return errInvalid("invalid lead_scope")
	}
	navKeys := defaultNavForAccount(accountType)
	for k := range o.Nav {
		if _, ok := navKeys[k]; !ok {
			return errInvalid("unknown nav key: " + k)
		}
	}
	for k := range o.Actions {
		valid := false
		for _, ak := range allActionKeys {
			if k == ak {
				valid = true
				break
			}
		}
		if !valid {
			return errInvalid("unknown action key: " + k)
		}
	}
	return nil
}

type validationError string

func (e validationError) Error() string { return string(e) }

func errInvalid(msg string) error { return validationError(msg) }

func ToMap(e Effective) map[string]any {
	return map[string]any{
		"nav":        e.Nav,
		"lead_scope": e.LeadScope,
		"actions": map[string]bool{
			ActionSettingsAdmin:      e.Actions.SettingsAdmin,
			ActionPipelinesRouting:   e.Actions.PipelinesRouting,
			ActionContractsPartners:  e.Actions.ContractsPartners,
			ActionBilling:            e.Actions.Billing,
			ActionAppointmentsManage: e.Actions.AppointmentsManage,
		},
	}
}
