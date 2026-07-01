package auth

import (
	"testing"

	"github.com/echayko/leadrula/backend/internal/permissions"
)

func TestLeadScope_emptyPermsFallsBackToPreset(t *testing.T) {
	p := &Principal{Role: "admin", AccountType: "publisher"}
	if p.LeadScope() != permissions.LeadScopeAll {
		t.Fatalf("admin lead scope = %q, want all", p.LeadScope())
	}

	p = &Principal{Role: "user", AccountType: "publisher"}
	if p.LeadScope() != permissions.LeadScopeAssigned {
		t.Fatalf("user lead scope = %q, want assigned", p.LeadScope())
	}

	p = &Principal{Role: "follower", AccountType: "buyer"}
	if p.LeadScope() != permissions.LeadScopeFollowed {
		t.Fatalf("follower lead scope = %q, want followed", p.LeadScope())
	}
}

func TestLeadScope_explicitPermsOverridePreset(t *testing.T) {
	p := &Principal{
		Role:        "user",
		AccountType: "publisher",
		Perms:       permissions.Effective{LeadScope: permissions.LeadScopeAll},
	}
	if p.LeadScope() != permissions.LeadScopeAll {
		t.Fatalf("lead scope = %q, want all from overrides", p.LeadScope())
	}
}
