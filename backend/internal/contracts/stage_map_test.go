package contracts

import "testing"

func TestBuildStageMaps_byPosition(t *testing.T) {
	buyer := []stageRow{{id: 1, stageType: "open"}, {id: 2, stageType: "won"}}
	pub := []stageRow{{id: 10, stageType: "open"}, {id: 20, stageType: "won"}}
	maps, err := buildStageMaps(buyer, pub)
	if err != nil {
		t.Fatal(err)
	}
	if maps[1] != 10 || maps[2] != 20 {
		t.Fatalf("maps = %v", maps)
	}
}

func TestBuildStageMaps_requiresWonOnBoth(t *testing.T) {
	buyer := []stageRow{{id: 1, stageType: "open"}}
	pub := []stageRow{{id: 10, stageType: "open"}, {id: 20, stageType: "won"}}
	if _, err := buildStageMaps(buyer, pub); err == nil {
		t.Fatal("expected error when buyer pipeline has no won stage")
	}
}
