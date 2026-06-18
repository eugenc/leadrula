package googlemaps

import "testing"

func TestParseAddressComponents(t *testing.T) {
	components := []addressComponent{
		{LongText: "123", ShortText: "123", Types: []string{"street_number"}},
		{LongText: "Main Street", ShortText: "Main St", Types: []string{"route"}},
		{LongText: "Miami", ShortText: "Miami", Types: []string{"locality"}},
		{ShortText: "FL", Types: []string{"administrative_area_level_1"}},
		{LongText: "33101", ShortText: "33101", Types: []string{"postal_code"}},
	}
	out := &ParsedAddress{}
	parseAddressComponents(out, components)
	if out.Address != "123 Main Street" {
		t.Fatalf("address = %q", out.Address)
	}
	if out.City != "Miami" || out.State != "FL" || out.Zip != "33101" {
		t.Fatalf("city/state/zip = %q %q %q", out.City, out.State, out.Zip)
	}
}
