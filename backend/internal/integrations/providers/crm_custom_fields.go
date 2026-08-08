package providers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"regexp"
	"strings"
)

// CRMCustomField is a normalized custom field from an external CRM.
type CRMCustomField struct {
	ID       string   `json:"id"`
	Name     string   `json:"name"`
	FieldKey string   `json:"field_key"`
	Object   string   `json:"object"`
	DataType string   `json:"data_type"`
	Options  []string `json:"options,omitempty"`
}

var crmCustomFieldProviders = map[string]bool{
	"ghl":        true,
	"hubspot":    true,
	"salesforce": true,
	"pipedrive":  true,
	"zoho_crm":   true,
}

// CRMCustomFieldsSupported reports whether a provider slug supports custom field listing.
func CRMCustomFieldsSupported(slug string) bool {
	return crmCustomFieldProviders[slug]
}

// FetchCRMCustomFields loads custom fields from a connected CRM provider.
func FetchCRMCustomFields(ctx context.Context, slug string, credentials []byte, config map[string]any) ([]CRMCustomField, error) {
	switch slug {
	case "ghl":
		locationID, _ := config["location_id"].(string)
		if locationID == "" {
			return nil, fmt.Errorf("location_id is required")
		}
		token, err := ParseGHLCredentials(credentials)
		if err != nil {
			return nil, err
		}
		fields, err := FetchGHLCustomFields(ctx, token, locationID)
		if err != nil {
			return nil, err
		}
		return ghlCustomFieldsToCRM(fields), nil
	case "hubspot":
		return fetchHubSpotCustomFields(ctx, credentials)
	case "salesforce":
		return fetchSalesforceCustomFields(ctx, credentials, config)
	case "pipedrive":
		return fetchPipedriveCustomFields(ctx, credentials)
	case "zoho_crm":
		return fetchZohoCRMCustomFields(ctx, credentials, config)
	default:
		return nil, fmt.Errorf("custom field import not supported for %s", slug)
	}
}

func ghlCustomFieldsToCRM(fields []GHLCustomField) []CRMCustomField {
	out := make([]CRMCustomField, 0, len(fields))
	for _, f := range fields {
		model := strings.ToLower(strings.TrimSpace(f.Model))
		if model != "contact" && model != "opportunity" {
			continue
		}
		key := strings.TrimSpace(f.FieldKey)
		if key == "" {
			key = strings.TrimSpace(f.ID)
		}
		out = append(out, CRMCustomField{
			ID:       strings.TrimSpace(f.ID),
			Name:     strings.TrimSpace(f.Name),
			FieldKey: key,
			Object:   model,
			DataType: strings.TrimSpace(f.DataType),
		})
	}
	return out
}

func fetchHubSpotCustomFields(ctx context.Context, credentials []byte) ([]CRMCustomField, error) {
	token, err := oauthAccessToken(credentials)
	if err != nil {
		return nil, err
	}
	body, err := crmHTTPGet(ctx, hubspotBase+"/crm/v3/properties/contacts", "Bearer "+token)
	if err != nil {
		return nil, err
	}
	var parsed struct {
		Results []struct {
			Name        string `json:"name"`
			Label       string `json:"label"`
			Type        string `json:"type"`
			FieldType   string `json:"fieldType"`
			HubspotDef  bool   `json:"hubspotDefined"`
			Options     []struct {
				Label string `json:"label"`
				Value string `json:"value"`
			} `json:"options"`
		} `json:"results"`
	}
	if err := json.Unmarshal(body, &parsed); err != nil {
		return nil, err
	}
	out := make([]CRMCustomField, 0)
	for _, p := range parsed.Results {
		if p.HubspotDef {
			continue
		}
		name := strings.TrimSpace(p.Label)
		if name == "" {
			name = strings.TrimSpace(p.Name)
		}
		dataType := strings.TrimSpace(p.FieldType)
		if dataType == "" {
			dataType = strings.TrimSpace(p.Type)
		}
		var opts []string
		for _, o := range p.Options {
			if v := strings.TrimSpace(o.Value); v != "" {
				opts = append(opts, v)
			} else if l := strings.TrimSpace(o.Label); l != "" {
				opts = append(opts, l)
			}
		}
		out = append(out, CRMCustomField{
			ID:       strings.TrimSpace(p.Name),
			Name:     name,
			FieldKey: strings.TrimSpace(p.Name),
			Object:   "contact",
			DataType: dataType,
			Options:  opts,
		})
	}
	return out, nil
}

func fetchSalesforceCustomFields(ctx context.Context, credentials []byte, config map[string]any) ([]CRMCustomField, error) {
	var creds struct {
		AccessToken string `json:"access_token"`
		InstanceURL string `json:"instance_url"`
	}
	if err := json.Unmarshal(credentials, &creds); err != nil || creds.AccessToken == "" {
		return nil, fmt.Errorf("access_token required")
	}
	instanceURL := strings.TrimRight(creds.InstanceURL, "/")
	if instanceURL == "" {
		if u, ok := config["instance_url"].(string); ok {
			instanceURL = strings.TrimRight(u, "/")
		}
	}
	if instanceURL == "" {
		return nil, fmt.Errorf("instance_url required")
	}
	descURL := instanceURL + "/services/data/v59.0/sobjects/Lead/describe"
	body, err := crmHTTPGet(ctx, descURL, "Bearer "+creds.AccessToken)
	if err != nil {
		return nil, err
	}
	var parsed struct {
		Fields []struct {
			Name       string `json:"name"`
			Label      string `json:"label"`
			Type       string `json:"type"`
			Custom     bool   `json:"custom"`
			PicklistValues []struct {
				Value string `json:"value"`
			} `json:"picklistValues"`
		} `json:"fields"`
	}
	if err := json.Unmarshal(body, &parsed); err != nil {
		return nil, err
	}
	out := make([]CRMCustomField, 0)
	for _, f := range parsed.Fields {
		if !f.Custom {
			continue
		}
		name := strings.TrimSpace(f.Label)
		if name == "" {
			name = strings.TrimSpace(f.Name)
		}
		var opts []string
		for _, o := range f.PicklistValues {
			if v := strings.TrimSpace(o.Value); v != "" {
				opts = append(opts, v)
			}
		}
		out = append(out, CRMCustomField{
			ID:       strings.TrimSpace(f.Name),
			Name:     name,
			FieldKey: strings.TrimSpace(f.Name),
			Object:   "lead",
			DataType: strings.TrimSpace(f.Type),
			Options:  opts,
		})
	}
	return out, nil
}

func fetchPipedriveCustomFields(ctx context.Context, credentials []byte) ([]CRMCustomField, error) {
	token, err := oauthAccessToken(credentials)
	if err != nil {
		return nil, err
	}
	body, err := crmHTTPGet(ctx, pipedriveBase+"/personFields", "Bearer "+token)
	if err != nil {
		return nil, err
	}
	var parsed struct {
		Data []struct {
			ID       int    `json:"id"`
			Name     string `json:"name"`
			Key      string `json:"key"`
			FieldType string `json:"field_type"`
			Options  []struct {
				Label string `json:"label"`
			} `json:"options"`
		} `json:"data"`
		Success bool `json:"success"`
	}
	if err := json.Unmarshal(body, &parsed); err != nil {
		return nil, err
	}
	if !parsed.Success {
		return nil, fmt.Errorf("pipedrive personFields request failed")
	}
	out := make([]CRMCustomField, 0, len(parsed.Data))
	for _, f := range parsed.Data {
		key := strings.TrimSpace(f.Key)
		if key == "" {
			continue
		}
		var opts []string
		for _, o := range f.Options {
			if l := strings.TrimSpace(o.Label); l != "" {
				opts = append(opts, l)
			}
		}
		out = append(out, CRMCustomField{
			ID:       fmt.Sprintf("%d", f.ID),
			Name:     strings.TrimSpace(f.Name),
			FieldKey: key,
			Object:   "person",
			DataType: strings.TrimSpace(f.FieldType),
			Options:  opts,
		})
	}
	return out, nil
}

func fetchZohoCRMCustomFields(ctx context.Context, credentials []byte, config map[string]any) ([]CRMCustomField, error) {
	token, err := oauthAccessToken(credentials)
	if err != nil {
		return nil, err
	}
	domain, _ := config["api_domain"].(string)
	if domain == "" {
		domain = "com"
	}
	base := "https://www.zohoapis." + strings.TrimPrefix(domain, ".")
	fieldsURL := base + "/crm/v6/settings/fields?module=Leads"
	body, err := crmHTTPGet(ctx, fieldsURL, "Zoho-oauthtoken "+token)
	if err != nil {
		return nil, err
	}
	var parsed struct {
		Fields []struct {
			ID         string `json:"id"`
			APIName    string `json:"api_name"`
			FieldLabel string `json:"field_label"`
			DataType   string `json:"data_type"`
			CustomField bool  `json:"custom_field"`
			PickListValues []struct {
				DisplayValue string `json:"display_value"`
			} `json:"pick_list_values"`
		} `json:"fields"`
	}
	if err := json.Unmarshal(body, &parsed); err != nil {
		return nil, err
	}
	out := make([]CRMCustomField, 0)
	for _, f := range parsed.Fields {
		if !f.CustomField {
			continue
		}
		name := strings.TrimSpace(f.FieldLabel)
		if name == "" {
			name = strings.TrimSpace(f.APIName)
		}
		var opts []string
		for _, o := range f.PickListValues {
			if v := strings.TrimSpace(o.DisplayValue); v != "" {
				opts = append(opts, v)
			}
		}
		out = append(out, CRMCustomField{
			ID:       strings.TrimSpace(f.ID),
			Name:     name,
			FieldKey: strings.TrimSpace(f.APIName),
			Object:   "lead",
			DataType: strings.TrimSpace(f.DataType),
			Options:  opts,
		})
	}
	return out, nil
}

// MapCRMFieldType maps a CRM field type to a Leadrula custom field type.
func MapCRMFieldType(providerSlug, dataType string) string {
	dt := strings.ToLower(strings.TrimSpace(dataType))
	switch providerSlug {
	case "ghl":
		switch dt {
		case "numerical", "number", "monetary", "float", "integer":
			return "number"
		case "date":
			return "date"
		case "datetime", "date_time":
			return "datetime"
		case "single_options", "multiple_options", "select", "dropdown", "radio", "checkbox_list":
			return "dropdown"
		case "checkbox", "boolean":
			return "checkbox"
		default:
			return "text"
		}
	case "hubspot":
		switch dt {
		case "number":
			return "number"
		case "date":
			return "date"
		case "datetime":
			return "datetime"
		case "select", "radio", "checkbox", "booleancheckbox", "enumeration":
			if dt == "booleancheckbox" || dt == "checkbox" {
				return "checkbox"
			}
			return "dropdown"
		default:
			return "text"
		}
	case "salesforce":
		switch dt {
		case "double", "int", "currency", "percent":
			return "number"
		case "date":
			return "date"
		case "datetime":
			return "datetime"
		case "boolean":
			return "checkbox"
		case "picklist", "multipicklist":
			return "dropdown"
		default:
			return "text"
		}
	case "pipedrive":
		switch dt {
		case "int", "double", "monetary":
			return "number"
		case "date":
			return "date"
		case "time", "timerange":
			return "datetime"
		case "enum", "set":
			return "dropdown"
		case "boolean":
			return "checkbox"
		default:
			return "text"
		}
	case "zoho_crm":
		switch dt {
		case "integer", "bigint", "double", "currency", "percent":
			return "number"
		case "date":
			return "date"
		case "datetime":
			return "datetime"
		case "boolean":
			return "checkbox"
		case "picklist", "multiselectpicklist":
			return "dropdown"
		default:
			return "text"
		}
	default:
		return "text"
	}
}

// CRMInboundSourceKey returns the webhook payload key for inbound field mapping.
func CRMInboundSourceKey(providerSlug string, field CRMCustomField) string {
	key := strings.TrimSpace(field.FieldKey)
	if key == "" {
		key = strings.TrimSpace(field.ID)
	}
	switch providerSlug {
	case "ghl":
		return key
	case "hubspot":
		return key
	case "salesforce":
		return key
	case "pipedrive":
		return "current." + key
	case "zoho_crm":
		return "data." + key
	default:
		return key
	}
}

// PrepareCRMInboundFlat normalizes CRM inbound payloads for custom field mapping.
func PrepareCRMInboundFlat(providerSlug string, flat map[string]any) map[string]any {
	if flat == nil {
		return map[string]any{}
	}
	switch providerSlug {
	case "ghl":
		return PrepareGHLInboundFlat(flat, nil)
	case "hubspot":
		return flattenHubSpotInbound(flat)
	case "pipedrive":
		return flattenPipedriveInbound(flat)
	case "zoho_crm":
		return flattenZohoInbound(flat)
	default:
		return flat
	}
}

func flattenHubSpotInbound(flat map[string]any) map[string]any {
	out := map[string]any{}
	for k, v := range flat {
		out[k] = v
	}
	if props, ok := flat["properties"].(map[string]any); ok {
		for k, v := range props {
			if _, exists := out[k]; !exists {
				out[k] = v
			}
		}
	}
	if name, ok := flat["propertyName"].(string); ok && strings.TrimSpace(name) != "" {
		if val, ok := flat["propertyValue"]; ok {
			out[name] = val
		}
	}
	return out
}

func flattenPipedriveInbound(flat map[string]any) map[string]any {
	out := map[string]any{}
	for k, v := range flat {
		out[k] = v
	}
	current, ok := flat["current"].(map[string]any)
	if !ok {
		return out
	}
	for k, v := range current {
		out["current."+k] = v
	}
	if custom, ok := current["custom_fields"].(map[string]any); ok {
		for k, v := range custom {
			out["current."+k] = v
		}
	}
	return out
}

func flattenZohoInbound(flat map[string]any) map[string]any {
	out := map[string]any{}
	for k, v := range flat {
		out[k] = v
	}
	data, ok := flat["data"].(map[string]any)
	if !ok {
		return out
	}
	for k, v := range data {
		out["data."+k] = v
	}
	return out
}

// FetchSalesforceLeadCustomValues loads custom field values for a Salesforce Lead.
func FetchSalesforceLeadCustomValues(ctx context.Context, credentials []byte, config map[string]any, externalID string, fieldKeys []string) (map[string]any, error) {
	if len(fieldKeys) == 0 || strings.TrimSpace(externalID) == "" {
		return map[string]any{}, nil
	}
	var creds struct {
		AccessToken string `json:"access_token"`
		InstanceURL string `json:"instance_url"`
	}
	if err := json.Unmarshal(credentials, &creds); err != nil || creds.AccessToken == "" {
		return nil, fmt.Errorf("access_token required")
	}
	instanceURL := strings.TrimRight(creds.InstanceURL, "/")
	if instanceURL == "" {
		if u, ok := config["instance_url"].(string); ok {
			instanceURL = strings.TrimRight(u, "/")
		}
	}
	if instanceURL == "" {
		return nil, fmt.Errorf("instance_url required")
	}
	fields := strings.Join(fieldKeys, ",")
	leadURL := instanceURL + "/services/data/v59.0/sobjects/Lead/" + url.PathEscape(externalID) + "?fields=" + url.QueryEscape(fields)
	body, err := crmHTTPGet(ctx, leadURL, "Bearer "+creds.AccessToken)
	if err != nil {
		return nil, err
	}
	var record map[string]any
	if err := json.Unmarshal(body, &record); err != nil {
		return nil, err
	}
	out := map[string]any{}
	for _, key := range fieldKeys {
		if v, ok := record[key]; ok && v != nil {
			out[key] = v
		}
	}
	return out, nil
}

var slugNonAlnum = regexp.MustCompile(`[^a-z0-9]+`)

// SlugFieldKey converts a display name to a Leadrula field_key.
func SlugFieldKey(name string) string {
	s := strings.ToLower(strings.TrimSpace(name))
	s = slugNonAlnum.ReplaceAllString(s, "_")
	s = strings.Trim(s, "_")
	return s
}
