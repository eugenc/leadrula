package providers

import "testing"

func TestInferCRMStageType(t *testing.T) {
	tests := []struct {
		name   string
		stage  CRMStage
		isLast bool
		want   string
	}{
		{
			name:  "won signal",
			stage: CRMStage{Name: "Closed Won", IsWon: true},
			want:  "won",
		},
		{
			name:  "closed lost",
			stage: CRMStage{Name: "Lost", IsClosedLost: true, IsClosed: true},
			want:  "disqualification",
		},
		{
			name:  "standard",
			stage: CRMStage{Name: "Qualified"},
			want:  "standard",
		},
		{
			name:   "last stage name heuristic",
			stage:  CRMStage{Name: "Closed Won"},
			isLast: true,
			want:   "won",
		},
		{
			name:   "last stage without signal stays standard",
			stage:  CRMStage{Name: "Negotiation"},
			isLast: true,
			want:   "standard",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := InferCRMStageType(tc.stage, tc.isLast)
			if got != tc.want {
				t.Fatalf("got %q want %q", got, tc.want)
			}
		})
	}
}

func TestMergePipelineStageMapEntries(t *testing.T) {
	existing := []GHLPipelineStageMapEntry{
		{LeadrulaPipelineID: 1, LeadrulaStageID: 10, GHLPipelineID: "p1", GHLPipelineStageID: "s1"},
	}
	add := []GHLPipelineStageMapEntry{
		{LeadrulaPipelineID: 1, LeadrulaStageID: 10, GHLPipelineID: "p2", GHLPipelineStageID: "s2"},
		{LeadrulaPipelineID: 2, LeadrulaStageID: 20, GHLPipelineID: "p1", GHLPipelineStageID: "s3"},
	}
	merged := MergePipelineStageMapEntries(existing, add)
	if len(merged) != 2 {
		t.Fatalf("len=%d want 2", len(merged))
	}
	if merged[1].LeadrulaPipelineID != 2 || merged[1].GHLPipelineStageID != "s3" {
		t.Fatalf("unexpected second entry: %+v", merged[1])
	}
}

func TestApplyHubSpotStageSignals(t *testing.T) {
	st := CRMStage{Name: "Closed Won"}
	applyHubSpotStageSignals(&st, map[string]string{"isClosed": "true", "probability": "1.0"})
	if !st.IsWon || !st.IsClosed {
		t.Fatalf("expected won closed stage: %+v", st)
	}
}

func TestCRMPipelineImportSupported(t *testing.T) {
	if !CRMPipelineImportSupported("ghl") {
		t.Fatal("ghl should be supported")
	}
	if CRMPipelineImportSupported("salesforce") {
		t.Fatal("salesforce should not be supported in v1")
	}
}
