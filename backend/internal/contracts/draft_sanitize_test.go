package contracts

import "testing"

func TestSanitizePublisherDirectContractParams_stripsBuyerPipelineFromPublisherPayload(t *testing.T) {
	counterparty := int64(99)
	p := CreateParams{
		BuyerID:         42,
		BuyerPipelineID: 10,
		ReturnStageID:   20,
		Compensations: []CompensationParams{
			{
				Kind:                   "flat_rate",
				CounterpartyPipelineID: &counterparty,
			},
		},
	}
	got := sanitizePublisherDirectContractParams(p, nil)
	if got.BuyerPipelineID != 0 {
		t.Fatalf("BuyerPipelineID = %d, want 0", got.BuyerPipelineID)
	}
	if got.ReturnStageID != 0 {
		t.Fatalf("ReturnStageID = %d, want 0", got.ReturnStageID)
	}
	if got.Compensations[0].CounterpartyPipelineID != nil {
		t.Fatal("expected counterparty pipeline cleared on compensation")
	}
}

func TestSanitizePublisherDirectContractParams_preservesExistingBuyerPipeline(t *testing.T) {
	existingPipeline := int64(55)
	existing := &Contract{BuyerPipelineID: &existingPipeline}
	p := CreateParams{
		BuyerID:         42,
		BuyerPipelineID: 10,
		ReturnStageID:   20,
	}
	got := sanitizePublisherDirectContractParams(p, existing)
	if got.BuyerPipelineID != existingPipeline {
		t.Fatalf("BuyerPipelineID = %d, want %d preserved", got.BuyerPipelineID, existingPipeline)
	}
}

func TestSanitizePublisherDirectContractParams_openOfferUnchanged(t *testing.T) {
	counterparty := int64(99)
	p := CreateParams{
		BuyerID:         0,
		BuyerPipelineID: 10,
		Compensations: []CompensationParams{
			{CounterpartyPipelineID: &counterparty},
		},
	}
	got := sanitizePublisherDirectContractParams(p, nil)
	if got.BuyerPipelineID != 10 {
		t.Fatalf("BuyerPipelineID = %d, want 10 for open offer", got.BuyerPipelineID)
	}
	if got.Compensations[0].CounterpartyPipelineID == nil || *got.Compensations[0].CounterpartyPipelineID != counterparty {
		t.Fatal("expected open offer compensation counterparty preserved")
	}
}
