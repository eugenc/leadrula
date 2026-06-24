package appointments

import (
	"testing"
	"time"
)

func TestIntervalsOverlap(t *testing.T) {
	loc := time.UTC
	base := time.Date(2026, 6, 24, 9, 0, 0, 0, loc)
	a := timeInterval{start: base, duration: 30, buffer: 10}
	b := timeInterval{start: base.Add(40 * time.Minute), duration: 30, buffer: 0}
	if intervalsOverlap(a, b) {
		t.Fatal("expected no overlap with 10m buffer gap")
	}
	c := timeInterval{start: base.Add(35 * time.Minute), duration: 15, buffer: 0}
	if !intervalsOverlap(a, c) {
		t.Fatal("expected overlap")
	}
}
