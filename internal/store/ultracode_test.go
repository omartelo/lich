package store

import (
	"testing"

	"github.com/omartelo/lich/internal/providers"
)

func TestSessionUltracodeRoundTrips(t *testing.T) {
	svc := newTestStore(t)
	_ = svc.AddProject("p1", "alpha", "/tmp/alpha")
	_ = svc.AddSession("p1", "s1", "Session 1", "claude", "", 2, "")
	_ = svc.AddSession("p1", "s2", "Session 2", "claude", "", 3, "")

	if err := svc.SetSessionUltracode("s1"); err != nil {
		t.Fatalf("SetSessionUltracode: %v", err)
	}
	if !svc.SessionUltracode("s1") {
		t.Error("SessionUltracode(s1) = false, want true")
	}
	if svc.SessionUltracode("s2") {
		t.Error("SessionUltracode(s2) = true, want false for a session opened without it")
	}
	if svc.SessionUltracode("ghost") {
		t.Error("SessionUltracode(ghost) = true, want false")
	}
}

// TestReopenWorktreeSessionCarriesUltracode pins the park/resume cycle, which
// keeps a card's identity under a new id: a reinsert that dropped the flag would
// respawn the session without ultracode and nothing on screen would say so.
func TestReopenWorktreeSessionCarriesUltracode(t *testing.T) {
	svc := newTestStore(t)
	_ = svc.AddProject("p1", "alpha", "/tmp/alpha")
	_ = svc.AddSession("p1", "base", "Session 1", "claude", "", 2, "")
	_ = svc.AddSession("p1", "wt1", "worker", "claude", "/wt/foo", 3, "")
	_ = svc.SetSessionUltracode("wt1")
	_ = svc.CloseSession("p1", "wt1", "base")

	if _, err := svc.ReopenWorktreeSession("p1", "/wt/foo", "wt2"); err != nil {
		t.Fatalf("ReopenWorktreeSession: %v", err)
	}
	if !svc.SessionUltracode("wt2") {
		t.Error("resumed session lost ultracode")
	}
}

func TestUltracodeIsOnOnlyForTheLiteralTrue(t *testing.T) {
	svc := newTestStore(t)
	if svc.Ultracode(providers.Claude) {
		t.Error("unconfigured provider runs ultracode, want off")
	}
	_ = svc.SetSetting("provider.claude.ultracode", globalScope, "yes")
	if svc.Ultracode(providers.Claude) {
		t.Error(`"yes" turns ultracode on, want only "true" to`)
	}
	_ = svc.SetSetting("provider.claude.ultracode", globalScope, "true")
	if !svc.Ultracode(providers.Claude) {
		t.Error("configured provider does not run ultracode, want on")
	}
	if svc.Ultracode(providers.Codex) {
		t.Error("another provider follows Claude's setting, want its own")
	}
	_ = svc.SetSetting("provider.claude.ultracode", "p1", "false")
	if !svc.Ultracode(providers.Claude) {
		t.Error("a project-scoped value overrides the global one, want global only")
	}
}

// TestInheritUltracodeFollowsTheForkedConversation pins the lookup a fork makes:
// by the provider conversation id it branches, never by anything the fork's own
// row carries.
func TestInheritUltracodeFollowsTheForkedConversation(t *testing.T) {
	svc := newTestStore(t)
	_ = svc.AddProject("p1", "alpha", "/tmp/alpha")
	_ = svc.AddSession("p1", "parent", "Session 1", "claude", "", 2, "")
	_ = svc.AddSession("p1", "plain", "Session 2", "claude", "", 3, "")
	_ = svc.AddSession("p1", "fork1", "Session 3", "claude", "/wt/a", 4, "")
	_ = svc.AddSession("p1", "fork2", "Session 4", "claude", "/wt/b", 5, "")
	_ = svc.SetProviderSession("parent", "conv-on")
	_ = svc.SetProviderSession("plain", "conv-off")
	_ = svc.SetSessionUltracode("parent")

	if err := svc.InheritUltracode("fork1", "conv-on"); err != nil {
		t.Fatalf("InheritUltracode: %v", err)
	}
	if !svc.SessionUltracode("fork1") {
		t.Error("fork of an ultracode conversation = off, want on")
	}
	if err := svc.InheritUltracode("fork2", "conv-off"); err != nil {
		t.Fatalf("InheritUltracode: %v", err)
	}
	if svc.SessionUltracode("fork2") {
		t.Error("fork of a plain conversation = on, want off")
	}
}
