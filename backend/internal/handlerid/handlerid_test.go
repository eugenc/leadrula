package handlerid

import (
	"strings"
	"testing"
)

func TestGenerate_accountLength(t *testing.T) {
	id := Generate("B")
	if !strings.HasPrefix(id, "B-") {
		t.Fatalf("prefix: got %q", id)
	}
	suffix := strings.TrimPrefix(id, "B-")
	if len(suffix) != DefaultLength {
		t.Fatalf("suffix length: got %d want %d (%q)", len(suffix), DefaultLength, id)
	}
	for _, c := range suffix {
		if !strings.ContainsRune(alphabet, c) {
			t.Fatalf("invalid char %q in %q", c, id)
		}
	}
}

func TestGenerateContract_length(t *testing.T) {
	id := GenerateContract()
	if !strings.HasPrefix(id, "C-") {
		t.Fatalf("prefix: got %q", id)
	}
	suffix := strings.TrimPrefix(id, "C-")
	if len(suffix) != ContractLength {
		t.Fatalf("suffix length: got %d want %d (%q)", len(suffix), ContractLength, id)
	}
	for _, c := range suffix {
		if !strings.ContainsRune(alphabet, c) {
			t.Fatalf("invalid char %q in %q", c, id)
		}
	}
}
