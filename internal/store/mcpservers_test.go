package store

import (
	"slices"
	"testing"

	"github.com/omartelo/lich/internal/providers"
)

// sessionOf reads one session back out of the hydration the window loads, which
// is the only route these names take to the card.
func sessionOf(t *testing.T, svc *Service, sessionID string) Session {
	t.Helper()
	projects, err := svc.LoadState()
	if err != nil {
		t.Fatalf("LoadState: %v", err)
	}
	for _, project := range projects {
		for _, session := range project.Sessions {
			if session.ID == sessionID {
				return session
			}
		}
	}
	t.Fatalf("no session %q in the loaded state", sessionID)
	return Session{}
}

// The spawn writes the names and the hydration reads them back: a page reload
// finds the PTY already running, so there is no second spawn to report them.
func TestSessionMCPServersRoundTrip(t *testing.T) {
	svc := sandboxProject(t, "/work/alpha")
	if err := svc.AddSession("p1", "s1", "one", providers.OMP, "", 0, ""); err != nil {
		t.Fatalf("AddSession: %v", err)
	}
	if got := sessionOf(t, svc, "s1").MCPServers; got != nil {
		t.Errorf("an unspawned session = %v, want none", got)
	}

	want := []string{"ai-memory", "lich"}
	if err := svc.SetSessionMCPServers("s1", want); err != nil {
		t.Fatalf("SetSessionMCPServers: %v", err)
	}
	if got := sessionOf(t, svc, "s1").MCPServers; !slices.Equal(got, want) {
		t.Errorf("MCPServers = %v, want %v", got, want)
	}
}

// A session that reached nothing and a session nobody spawned read the same
// way, because the card does the same thing with both: shows the name whole.
func TestSessionMCPServersClearsOnAnEmptyList(t *testing.T) {
	svc := sandboxProject(t, "/work/alpha")
	if err := svc.AddSession("p1", "s1", "one", providers.OMP, "", 0, ""); err != nil {
		t.Fatalf("AddSession: %v", err)
	}
	if err := svc.SetSessionMCPServers("s1", []string{"lich"}); err != nil {
		t.Fatalf("SetSessionMCPServers: %v", err)
	}
	if err := svc.SetSessionMCPServers("s1", nil); err != nil {
		t.Fatalf("SetSessionMCPServers(nil): %v", err)
	}
	if got := sessionOf(t, svc, "s1").MCPServers; got != nil {
		t.Errorf("MCPServers = %v, want none", got)
	}
}

// A row written by a lich that spelled this column differently is not a crash
// and not a guess: it reads as no servers, which is what an unspawned row says.
func TestDecodeMCPServersRefusesWhatItCannotRead(t *testing.T) {
	for _, encoded := range []string{"", "not json", `{"lich":true}`} {
		if got := decodeMCPServers(encoded); got != nil {
			t.Errorf("decodeMCPServers(%q) = %v, want none", encoded, got)
		}
	}
}
