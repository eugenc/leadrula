package providers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const ghlV2BaseURL = "https://services.leadconnectorhq.com"

type ghlHTTPResult struct {
	Status int
	Body   []byte
}

func ghlDo(ctx context.Context, method, path string, token, locationID string, body any) (ghlHTTPResult, error) {
	var bodyBytes []byte
	if body != nil {
		var err error
		bodyBytes, err = json.Marshal(body)
		if err != nil {
			return ghlHTTPResult{}, err
		}
	}
	fullURL := ghlV2BaseURL + path
	req, err := http.NewRequestWithContext(ctx, method, fullURL, bytes.NewReader(bodyBytes))
	if err != nil {
		return ghlHTTPResult{}, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Version", ghlAPIVersion)
	req.Header.Set("Accept", "application/json")
	if locationID != "" {
		req.Header.Set("locationId", locationID)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return ghlHTTPResult{}, err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	return ghlHTTPResult{Status: resp.StatusCode, Body: raw}, nil
}

func ghlErrorMessage(res ghlHTTPResult) string {
	var parsed struct {
		Message string `json:"message"`
		Error   string `json:"error"`
	}
	_ = json.Unmarshal(res.Body, &parsed)
	msg := strings.TrimSpace(parsed.Message)
	if msg == "" {
		msg = strings.TrimSpace(parsed.Error)
	}
	if msg == "" {
		msg = strings.TrimSpace(string(res.Body))
	}
	if len(msg) > 200 {
		msg = msg[:200]
	}
	if msg == "" {
		return fmt.Sprintf("ghl returned %d", res.Status)
	}
	return fmt.Sprintf("ghl returned %d: %s", res.Status, msg)
}

func ghlUpsertContact(ctx context.Context, token string, cfg GHLConfig, payload DeliveryPayload) (string, *DeliveryResult, error) {
	contact := buildGHLContactBody(cfg, payload)
	res, err := ghlDo(ctx, http.MethodPost, "/contacts/upsert", token, cfg.LocationID, contact)
	mapped := AnyMapToMapped(contact)
	result := &DeliveryResult{
		HTTPStatus: res.Status,
		Raw:        res.Body,
		Request:    marshalRequestLog(mapped),
	}
	if err != nil {
		return "", result, err
	}
	if res.Status < 200 || res.Status >= 300 {
		return "", result, fmt.Errorf("%s", ghlErrorMessage(res))
	}
	contactID := ghlExtractContactID(res.Body)
	if contactID == "" {
		return "", result, fmt.Errorf("ghl contact upsert returned no contact id")
	}
	result.ExternalID = contactID
	return contactID, result, nil
}

func ghlExtractContactID(body []byte) string {
	var parsed struct {
		Contact struct {
			ID string `json:"id"`
		} `json:"contact"`
		ID string `json:"id"`
	}
	_ = json.Unmarshal(body, &parsed)
	if parsed.Contact.ID != "" {
		return parsed.Contact.ID
	}
	return parsed.ID
}

func ghlCreateOpportunity(ctx context.Context, token string, cfg GHLConfig, contactID, pipelineID, stageID string, payload DeliveryPayload) (*DeliveryResult, error) {
	name := resolveGHLTitleTemplate(cfg.OpportunityTitleTemplate, payload)
	if name == "" {
		name = defaultOpportunityTitle(payload)
	}
	opp := map[string]any{
		"pipelineId":        pipelineID,
		"pipelineStageId":   stageID,
		"locationId":        cfg.LocationID,
		"contactId":         contactID,
		"name":              name,
		"status":            "open",
		"source":            payload.Source,
	}
	applyGHLOpportunityStandardFields(opp, cfg.OpportunityStandardFields, payload)
	if fields := ghlCustomFieldsPayload(cfg.OutboundFieldMap, payload, "opportunity"); len(fields) > 0 {
		opp["customFields"] = fields
	}
	res, err := ghlDo(ctx, http.MethodPost, "/opportunities/", token, cfg.LocationID, opp)
	mapped := AnyMapToMapped(opp)
	result := &DeliveryResult{
		HTTPStatus: res.Status,
		Raw:        res.Body,
		Request:    marshalRequestLog(mapped),
	}
	if err != nil {
		return result, err
	}
	if res.Status < 200 || res.Status >= 300 {
		if ghlIsDuplicateOpportunity(res) {
			return result, nil
		}
		return result, fmt.Errorf("%s", ghlErrorMessage(res))
	}
	return result, nil
}

func ghlIsDuplicateOpportunity(res ghlHTTPResult) bool {
	if res.Status != 400 {
		return false
	}
	var parsed struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	}
	_ = json.Unmarshal(res.Body, &parsed)
	return parsed.Code == "OPPORTUNITY_NO_DUPLICATE" ||
		strings.Contains(strings.ToLower(parsed.Message), "duplicate opportunity")
}

func ghlCreateAppointment(ctx context.Context, token string, cfg GHLConfig, contactID string, payload DeliveryPayload) (*DeliveryResult, error) {
	datetimeStr := resolveGHLFieldSourceValue(cfg.AppointmentDatetime, payload)
	duration := appointmentDurationMinutes(cfg.AppointmentStandardFields)
	startISO, endISO, err := parseAppointmentTimes(datetimeStr, cfg.AppointmentTimezone, duration)
	if err != nil {
		return nil, err
	}
	title := resolveGHLTitleTemplate(cfg.AppointmentTitleTemplate, payload)
	if title == "" {
		title = defaultAppointmentTitle(payload)
	}
	notes := ""
	if cfg.AppointmentNotes != nil {
		notes = resolveGHLFieldSourceValue(*cfg.AppointmentNotes, payload)
	}
	assignedUserID, err := ghlCalendarAssignedUserID(ctx, token, cfg)
	if err != nil {
		return nil, err
	}
	if v := strings.TrimSpace(resolveGHLFieldSourceValue(cfg.AppointmentStandardFields.AssignedUserID, payload)); v != "" {
		assignedUserID = v
	}
	event := map[string]any{
		"calendarId":               cfg.CalendarID,
		"locationId":               cfg.LocationID,
		"contactId":                contactID,
		"title":                    title,
		"appointmentStatus":        "confirmed",
		"startTime":                startISO,
		"endTime":                  endISO,
		"ignoreFreeSlotValidation": true,
		"assignedUserId":           assignedUserID,
	}
	applyGHLAppointmentStandardFields(event, cfg.AppointmentStandardFields, payload)
	if notes != "" {
		event["notes"] = notes
	}
	res, err := ghlDo(ctx, http.MethodPost, "/calendars/events/appointments", token, cfg.LocationID, event)
	mapped := AnyMapToMapped(event)
	result := &DeliveryResult{
		HTTPStatus: res.Status,
		Raw:        res.Body,
		Request:    marshalRequestLog(mapped),
	}
	if err != nil {
		return result, err
	}
	if res.Status < 200 || res.Status >= 300 {
		return result, fmt.Errorf("%s", ghlErrorMessage(res))
	}
	return result, nil
}

func marshalRequestLog(mapped map[string]string) []byte {
	b, _ := json.Marshal(mapped)
	return b
}

type GHLPipeline struct {
	ID     string      `json:"id"`
	Name   string      `json:"name"`
	Stages []GHLStage  `json:"stages"`
}

type GHLStage struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type GHLCalendar struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

func ghlListPipelines(ctx context.Context, token, locationID string) ([]GHLPipeline, error) {
	path := "/opportunities/pipelines?locationId=" + url.QueryEscape(locationID)
	res, err := ghlDo(ctx, http.MethodGet, path, token, locationID, nil)
	if err != nil {
		return nil, err
	}
	if res.Status < 200 || res.Status >= 300 {
		return nil, fmt.Errorf("%s", ghlErrorMessage(res))
	}
	var parsed struct {
		Pipelines []GHLPipeline `json:"pipelines"`
	}
	if err := json.Unmarshal(res.Body, &parsed); err != nil {
		return nil, err
	}
	return parsed.Pipelines, nil
}

func ghlCalendarAssignedUserID(ctx context.Context, token string, cfg GHLConfig) (string, error) {
	if cfg.CalendarID == "" {
		return "", fmt.Errorf("calendar_id required")
	}
	path := "/calendars/" + url.PathEscape(cfg.CalendarID)
	res, err := ghlDo(ctx, http.MethodGet, path, token, cfg.LocationID, nil)
	if err != nil {
		return "", err
	}
	if res.Status < 200 || res.Status >= 300 {
		return "", fmt.Errorf("%s", ghlErrorMessage(res))
	}
	var parsed struct {
		Calendar struct {
			TeamMembers []struct {
				UserID    string `json:"userId"`
				IsPrimary bool   `json:"isPrimary"`
			} `json:"teamMembers"`
		} `json:"calendar"`
	}
	if err := json.Unmarshal(res.Body, &parsed); err != nil {
		return "", err
	}
	for _, m := range parsed.Calendar.TeamMembers {
		if m.IsPrimary && m.UserID != "" {
			return m.UserID, nil
		}
	}
	for _, m := range parsed.Calendar.TeamMembers {
		if m.UserID != "" {
			return m.UserID, nil
		}
	}
	return "", fmt.Errorf("no team member on GHL calendar %s", cfg.CalendarID)
}

func ghlListCalendars(ctx context.Context, token, locationID string) ([]GHLCalendar, error) {
	path := "/calendars/?locationId=" + url.QueryEscape(locationID)
	res, err := ghlDo(ctx, http.MethodGet, path, token, locationID, nil)
	if err != nil {
		return nil, err
	}
	if res.Status < 200 || res.Status >= 300 {
		return nil, fmt.Errorf("%s", ghlErrorMessage(res))
	}
	var parsed struct {
		Calendars []GHLCalendar `json:"calendars"`
	}
	if err := json.Unmarshal(res.Body, &parsed); err != nil {
		return nil, err
	}
	return parsed.Calendars, nil
}

func ghlTestConnection(ctx context.Context, token, locationID string) error {
	_, err := ghlListPipelines(ctx, token, locationID)
	return err
}

func FetchGHLPipelines(ctx context.Context, token, locationID string) ([]GHLPipeline, error) {
	return ghlListPipelines(ctx, token, locationID)
}

func FetchGHLCalendars(ctx context.Context, token, locationID string) ([]GHLCalendar, error) {
	return ghlListCalendars(ctx, token, locationID)
}

type GHLCustomField struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	FieldKey string `json:"fieldKey"`
	Model    string `json:"model"`
	DataType string `json:"dataType"`
}

type GHLCustomFieldResponse struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	FieldKey string `json:"field_key"`
	Model    string `json:"model"`
	DataType string `json:"data_type"`
}

func GHLCustomFieldsToResponse(fields []GHLCustomField) []GHLCustomFieldResponse {
	out := make([]GHLCustomFieldResponse, 0, len(fields))
	for _, f := range fields {
		out = append(out, GHLCustomFieldResponse{
			ID:       f.ID,
			Name:     f.Name,
			FieldKey: f.FieldKey,
			Model:    f.Model,
			DataType: f.DataType,
		})
	}
	return out
}

func ghlListCustomFields(ctx context.Context, token, locationID string) ([]GHLCustomField, error) {
	path := "/locations/" + url.PathEscape(locationID) + "/customFields?model=all"
	res, err := ghlDo(ctx, http.MethodGet, path, token, locationID, nil)
	if err != nil {
		return nil, err
	}
	if res.Status < 200 || res.Status >= 300 {
		return nil, fmt.Errorf("%s", ghlErrorMessage(res))
	}
	var parsed struct {
		CustomFields []GHLCustomField `json:"customFields"`
	}
	if err := json.Unmarshal(res.Body, &parsed); err != nil {
		return nil, err
	}
	return parsed.CustomFields, nil
}

func FetchGHLCustomFields(ctx context.Context, token, locationID string) ([]GHLCustomField, error) {
	return ghlListCustomFields(ctx, token, locationID)
}
