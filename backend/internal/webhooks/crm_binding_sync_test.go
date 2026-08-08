package webhooks

import "testing"

func TestMergeCRMBindingFieldMaps_skipsDuplicates(t *testing.T) {
	seen := map[string]int64{}
	added := 0

	merge := func(bindings []crmBindingFieldMap) {
		for _, b := range bindings {
			key := b.inboundSourceKey
			if fid, ok := seen[key]; ok && fid == b.customFieldID {
				continue
			}
			seen[key] = b.customFieldID
			added++
		}
	}

	merge([]crmBindingFieldMap{
		{customFieldID: 1, inboundSourceKey: "score"},
		{customFieldID: 1, inboundSourceKey: "score"},
		{customFieldID: 2, inboundSourceKey: "tier"},
	})
	if added != 2 {
		t.Fatalf("expected 2 maps added, got %d", added)
	}
}
