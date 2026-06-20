package billing

import (
	"strings"
	"testing"
)

func TestClampDeadline(t *testing.T) {
	cases := []struct{ in, want int }{
		{0, 7},
		{6, 7},
		{7, 7},
		{14, 14},
		{30, 30},
		{31, 30},
		{1000, 30},
	}
	for _, c := range cases {
		if got := clampDeadline(c.in); got != c.want {
			t.Errorf("clampDeadline(%d) = %d, want %d", c.in, got, c.want)
		}
	}
}

func TestOtherParty(t *testing.T) {
	if otherParty(partyBuyer) != partyPublisher {
		t.Errorf("otherParty(buyer) = %q, want %q", otherParty(partyBuyer), partyPublisher)
	}
	if otherParty(partyPublisher) != partyBuyer {
		t.Errorf("otherParty(publisher) = %q, want %q", otherParty(partyPublisher), partyBuyer)
	}
}

func TestCallerParty(t *testing.T) {
	d := &disputeState{buyerID: 10, publisherID: 20}

	if p, err := d.callerParty(10); err != nil || p != partyBuyer {
		t.Errorf("callerParty(buyer) = (%q, %v), want (buyer, nil)", p, err)
	}
	if p, err := d.callerParty(20); err != nil || p != partyPublisher {
		t.Errorf("callerParty(publisher) = (%q, %v), want (publisher, nil)", p, err)
	}
	if _, err := d.callerParty(999); err == nil {
		t.Error("callerParty(stranger) = nil error, want not-found error")
	}
}

func TestSafeFilename(t *testing.T) {
	cases := []struct{ in, want string }{
		{"report.pdf", "report.pdf"},
		{"  spaced name.png ", "spaced_name.png"},
		{"../../etc/passwd", ".._.._etc_passwd"},
		{`a\b\c.doc`, "a_b_c.doc"},
		{"", "file"},
		{"   ", "file"},
	}
	for _, c := range cases {
		if got := safeFilename(c.in); got != c.want {
			t.Errorf("safeFilename(%q) = %q, want %q", c.in, got, c.want)
		}
	}
	long := strings.Repeat("x", 200) + ".pdf"
	if got := safeFilename(long); len(got) != 80 {
		t.Errorf("safeFilename(long) length = %d, want 80", len(got))
	}
}
