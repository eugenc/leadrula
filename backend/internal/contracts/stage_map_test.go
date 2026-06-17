package contracts

import "testing"

func TestBuildStageMaps_byPosition(t *testing.T) {
	buyer := []stageRow{{id: 1, stageType: "open"}, {id: 2, stageType: "won"}}
	pub := []stageRow{{id: 10, stageType: "open"}, {id: 20, stageType: "won"}}
	maps, err := buildStageMaps(buyer, pub, 0)
	if err != nil {
		t.Fatal(err)
	}
	if maps[1] != 10 || maps[2] != 20 {
		t.Fatalf("maps = %v", maps)
	}
}

func TestBuildStageMaps_byType_actionStage(t *testing.T) {
	buyer := []stageRow{
		{id: 1, stageType: "standard"},
		{id: 2, stageType: "action"},
		{id: 3, stageType: "won"},
	}
	pub := []stageRow{
		{id: 10, stageType: "standard"},
		{id: 20, stageType: "action"},
		{id: 30, stageType: "won"},
	}
	maps, err := buildStageMaps(buyer, pub, 0)
	if err != nil {
		t.Fatal(err)
	}
	if maps[2] != 20 {
		t.Fatalf("action stage map = %d want 20", maps[2])
	}
}

func TestBuildStageMaps_requiresWonOnBuyer(t *testing.T) {
	buyer := []stageRow{{id: 1, stageType: "open"}}
	pub := []stageRow{{id: 10, stageType: "open"}, {id: 20, stageType: "won"}}
	if _, err := buildStageMaps(buyer, pub, 0); err == nil {
		t.Fatal("expected error when buyer pipeline has no won stage")
	}
}

func TestBuildStageMaps_pubWithoutWon_usesReturnFallback(t *testing.T) {
	buyer := []stageRow{
		{id: 1, stageType: "action"},
		{id: 2, stageType: "won"},
	}
	pub := []stageRow{
		{id: 10, stageType: "action"},
		{id: 11, stageType: "standard"},
	}
	maps, err := buildStageMaps(buyer, pub, 11)
	if err != nil {
		t.Fatal(err)
	}
	if maps[2] != 11 {
		t.Fatalf("buyer won maps to return stage = %d want 11", maps[2])
	}
}

func TestBuildStageMaps_pubWithoutWon_usesLastStage(t *testing.T) {
	buyer := []stageRow{
		{id: 1, stageType: "action"},
		{id: 2, stageType: "won"},
	}
	pub := []stageRow{
		{id: 10, stageType: "action"},
		{id: 11, stageType: "standard"},
	}
	maps, err := buildStageMaps(buyer, pub, 0)
	if err != nil {
		t.Fatal(err)
	}
	if maps[2] != 11 {
		t.Fatalf("buyer won maps to last pub stage = %d want 11", maps[2])
	}
}

func TestPublisherWonStage(t *testing.T) {
	pub := []stageRow{{id: 10, stageType: "standard"}, {id: 11, stageType: "action"}}
	id, err := publisherWonStage(pub, 99)
	if err != nil {
		t.Fatal(err)
	}
	if id != 99 {
		t.Fatalf("fallback = %d want 99", id)
	}
}
