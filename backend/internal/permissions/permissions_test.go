package permissions_test

import (
	"testing"

	"github.com/echayko/leadrula/backend/internal/permissions"
)

func TestResolveAdminDefault(t *testing.T) {
	e := permissions.Resolve("admin", "publisher", []byte("{}"))
	if e.LeadScope != permissions.LeadScopeAll {
		t.Fatalf("lead scope = %q", e.LeadScope)
	}
	if !e.CanNav(permissions.NavBilling) || !e.CanAction(permissions.ActionBilling) {
		t.Fatal("admin should have full access")
	}
}

func TestResolveUserDefault(t *testing.T) {
	e := permissions.Resolve("user", "publisher", []byte("{}"))
	if e.LeadScope != permissions.LeadScopeAssigned {
		t.Fatalf("lead scope = %q", e.LeadScope)
	}
	if !e.CanNav(permissions.NavBilling) {
		t.Fatal("user should have billing nav (view)")
	}
	if e.CanAction(permissions.ActionBilling) {
		t.Fatal("user should not have billing action")
	}
}

func TestResolveFollowerDefault(t *testing.T) {
	e := permissions.Resolve("follower", "buyer", []byte("{}"))
	if e.LeadScope != permissions.LeadScopeFollowed {
		t.Fatalf("lead scope = %q", e.LeadScope)
	}
}

func TestResolveOverrideNav(t *testing.T) {
	raw := []byte(`{"nav":{"billing":true}}`)
	e := permissions.Resolve("user", "publisher", raw)
	if !e.CanNav(permissions.NavBilling) {
		t.Fatal("expected billing nav override")
	}
	if e.CanAction(permissions.ActionBilling) {
		t.Fatal("nav override should not grant billing action")
	}
}

func TestResolveOverrideLeadScope(t *testing.T) {
	raw := []byte(`{"lead_scope":"all"}`)
	e := permissions.Resolve("user", "publisher", raw)
	if e.LeadScope != permissions.LeadScopeAll {
		t.Fatalf("lead scope = %q", e.LeadScope)
	}
}

func TestDeltaRoundTrip(t *testing.T) {
	preset := permissions.PresetForRole("admin", "publisher")
	effective := preset
	effective.Nav[permissions.NavBilling] = false
	effective.LeadScope = permissions.LeadScopeAssigned

	delta := permissions.Delta("admin", "publisher", effective)
	raw, err := permissions.MarshalOverrides(delta)
	if err != nil {
		t.Fatal(err)
	}
	got := permissions.Resolve("admin", "publisher", raw)
	if got.LeadScope != permissions.LeadScopeAssigned {
		t.Fatalf("lead scope = %q", got.LeadScope)
	}
	if got.CanNav(permissions.NavBilling) {
		t.Fatal("billing nav should be false")
	}
}

func TestIsFullAdmin(t *testing.T) {
	e := permissions.Resolve("admin", "publisher", []byte("{}"))
	if !e.IsFullAdmin() {
		t.Fatal("admin preset should be full admin")
	}
	e2 := permissions.Resolve("user", "publisher", []byte("{}"))
	if e2.IsFullAdmin() {
		t.Fatal("user preset should not be full admin")
	}
}

func TestResolveAssignedAndFollowedScope(t *testing.T) {
	raw := []byte(`{"lead_scope":"assigned_and_followed"}`)
	e := permissions.Resolve("user", "publisher", raw)
	if e.LeadScope != permissions.LeadScopeAssignedAndFollowed {
		t.Fatalf("lead scope = %q", e.LeadScope)
	}
	if !permissions.HasAssignedScope(e.LeadScope) || !permissions.HasFollowedScope(e.LeadScope) {
		t.Fatal("union scope should include assigned and followed")
	}
	if permissions.IsFollowedOnly(e.LeadScope) {
		t.Fatal("union scope is not followed-only")
	}
}

func TestDeltaAssignedAndFollowed(t *testing.T) {
	effective := permissions.PresetForRole("user", "publisher")
	effective.LeadScope = permissions.LeadScopeAssignedAndFollowed
	delta := permissions.Delta("user", "publisher", effective)
	if delta.LeadScope != permissions.LeadScopeAssignedAndFollowed {
		t.Fatalf("delta lead scope = %q", delta.LeadScope)
	}
}
