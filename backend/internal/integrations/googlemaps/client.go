package googlemaps

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

const autocompleteURL = "https://places.googleapis.com/v1/places:autocomplete"

type addressComponent struct {
	LongText  string   `json:"longText"`
	ShortText string   `json:"shortText"`
	Types     []string `json:"types"`
}

type Suggestion struct {
	PlaceID          string `json:"place_id"`
	Description      string `json:"description"`
	MainText         string `json:"main_text,omitempty"`
	SecondaryText    string `json:"secondary_text,omitempty"`
}

type ParsedAddress struct {
	PlaceID          string  `json:"place_id"`
	FormattedAddress string  `json:"formatted_address"`
	Address          string  `json:"address"`
	City             string  `json:"city"`
	State            string  `json:"state"`
	Zip              string  `json:"zip"`
	Country          string  `json:"country"`
	Lat              float64 `json:"lat"`
	Lng              float64 `json:"lng"`
}

func Autocomplete(ctx context.Context, apiKey, input, sessionToken string) ([]Suggestion, error) {
	input = strings.TrimSpace(input)
	if input == "" {
		return nil, nil
	}
	body := map[string]any{"input": input}
	if sessionToken != "" {
		body["sessionToken"] = sessionToken
	}
	raw, _ := json.Marshal(body)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, autocompleteURL, bytes.NewReader(raw))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Goog-Api-Key", apiKey)
	req.Header.Set("X-Goog-FieldMask", "suggestions.placePrediction.placeId,suggestions.placePrediction.text,suggestions.placePrediction.structuredFormat")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("google places autocomplete returned %d: %s", resp.StatusCode, trimErrBody(respBody))
	}

	var parsed struct {
		Suggestions []struct {
			PlacePrediction *struct {
				PlaceID string `json:"placeId"`
				Text    struct {
					Text string `json:"text"`
				} `json:"text"`
				StructuredFormat *struct {
					MainText struct {
						Text string `json:"text"`
					} `json:"mainText"`
					SecondaryText struct {
						Text string `json:"text"`
					} `json:"secondaryText"`
				} `json:"structuredFormat"`
			} `json:"placePrediction"`
		} `json:"suggestions"`
	}
	if err := json.Unmarshal(respBody, &parsed); err != nil {
		return nil, err
	}

	out := make([]Suggestion, 0, len(parsed.Suggestions))
	for _, s := range parsed.Suggestions {
		if s.PlacePrediction == nil || s.PlacePrediction.PlaceID == "" {
			continue
		}
		item := Suggestion{
			PlaceID:     s.PlacePrediction.PlaceID,
			Description: s.PlacePrediction.Text.Text,
		}
		if s.PlacePrediction.StructuredFormat != nil {
			item.MainText = s.PlacePrediction.StructuredFormat.MainText.Text
			item.SecondaryText = s.PlacePrediction.StructuredFormat.SecondaryText.Text
		}
		out = append(out, item)
	}
	return out, nil
}

func PlaceDetails(ctx context.Context, apiKey, placeID string) (*ParsedAddress, error) {
	placeID = strings.TrimSpace(placeID)
	if placeID == "" {
		return nil, fmt.Errorf("place_id is required")
	}
	id := strings.TrimPrefix(placeID, "places/")
	u := "https://places.googleapis.com/v1/places/" + url.PathEscape(id)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("X-Goog-Api-Key", apiKey)
	req.Header.Set("X-Goog-FieldMask", "id,formattedAddress,addressComponents,location")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("google places details returned %d: %s", resp.StatusCode, trimErrBody(respBody))
	}

	var place struct {
		ID               string `json:"id"`
		FormattedAddress string `json:"formattedAddress"`
		AddressComponents []addressComponent `json:"addressComponents"`
		Location *struct {
			Latitude  float64 `json:"latitude"`
			Longitude float64 `json:"longitude"`
		} `json:"location"`
	}
	if err := json.Unmarshal(respBody, &place); err != nil {
		return nil, err
	}

	parsed := &ParsedAddress{
		PlaceID:          normalizePlaceID(place.ID, placeID),
		FormattedAddress: place.FormattedAddress,
	}
	if place.Location != nil {
		parsed.Lat = place.Location.Latitude
		parsed.Lng = place.Location.Longitude
	}
	parseAddressComponents(parsed, place.AddressComponents)
	return parsed, nil
}

func ValidateAPIKey(ctx context.Context, apiKey string) error {
	if strings.TrimSpace(apiKey) == "" {
		return fmt.Errorf("api_key is required")
	}
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	_, err := Autocomplete(ctx, apiKey, "123 Main St", "")
	if err != nil {
		if strings.Contains(err.Error(), "403") || strings.Contains(err.Error(), "401") {
			return fmt.Errorf("invalid api key or places api not enabled")
		}
		return err
	}
	return nil
}

func parseAddressComponents(out *ParsedAddress, components []addressComponent) {
	var streetNumber, route, city, state, zip, country string
	for _, c := range components {
		for _, t := range c.Types {
			switch t {
			case "street_number":
				streetNumber = c.LongText
			case "route":
				route = c.LongText
			case "locality":
				if city == "" {
					city = c.LongText
				}
			case "postal_town":
				if city == "" {
					city = c.LongText
				}
			case "sublocality", "sublocality_level_1":
				if city == "" {
					city = c.LongText
				}
			case "administrative_area_level_2":
				if city == "" {
					city = c.LongText
				}
			case "administrative_area_level_1":
				state = c.ShortText
			case "postal_code":
				zip = c.LongText
			case "country":
				country = c.ShortText
			}
		}
	}
	switch {
	case streetNumber != "" && route != "":
		out.Address = strings.TrimSpace(streetNumber + " " + route)
	case route != "":
		out.Address = route
	case streetNumber != "":
		out.Address = streetNumber
	}
	out.City = city
	out.State = state
	out.Zip = zip
	out.Country = country
}

func normalizePlaceID(id, fallback string) string {
	id = strings.TrimPrefix(id, "places/")
	if id != "" {
		return id
	}
	return strings.TrimPrefix(fallback, "places/")
}

func trimErrBody(b []byte) string {
	s := strings.TrimSpace(string(b))
	if len(s) > 200 {
		return s[:200] + "..."
	}
	return s
}
