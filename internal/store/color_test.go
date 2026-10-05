package store

import "testing"

// colorOf reads one session's card colour straight off the hydration, the same
// read the window paints from.
func colorOf(t *testing.T, svc *Service, projectID, sessionID string) string {
	t.Helper()
	sessions, err := svc.sessionsOf(projectID)
	if err != nil {
		t.Fatalf("sessionsOf: %v", err)
	}
	for _, s := range sessions {
		if s.ID == sessionID {
			return s.Color
		}
	}
	t.Fatalf("session %q not in project %q", sessionID, projectID)
	return ""
}

func TestSetSessionColorPaintsAndClears(t *testing.T) {
	svc := newTestStore(t)
	_ = svc.AddProject("p1", "alpha", "/tmp/alpha")
	_ = svc.AddSession("p1", "s1", "Session 1", "", "", 2, "")

	if err := svc.SetSessionColor("s1", "blue"); err != nil {
		t.Fatalf("SetSessionColor: %v", err)
	}
	if got := colorOf(t, svc, "p1", "s1"); got != "blue" {
		t.Errorf("color = %q, want %q", got, "blue")
	}
	if err := svc.SetSessionColor("s1", ""); err != nil {
		t.Fatalf("SetSessionColor(clear): %v", err)
	}
	if got := colorOf(t, svc, "p1", "s1"); got != "" {
		t.Errorf("color after clearing = %q, want empty", got)
	}
}

// TestResumeKeepsTheColor: the colour is the user's mark on the card, and
// closing a session is not a reason to take it off.
func TestResumeKeepsTheColor(t *testing.T) {
	svc := newTestStore(t)
	_ = svc.AddProject("p1", "alpha", "/tmp/alpha")
	_ = svc.AddSession("p1", "s1", "Session 1", "", "", 2, "")
	_ = svc.SetSessionColor("s1", "green")
	if err := svc.CloseSession("p1", "s1", ""); err != nil {
		t.Fatalf("CloseSession: %v", err)
	}

	restored, err := svc.ReopenSession("s1", "s1-new")
	if err != nil {
		t.Fatalf("ReopenSession: %v", err)
	}
	if restored == nil {
		t.Fatal("ReopenSession returned no session")
	}
	if restored.Color != "green" {
		t.Errorf("resumed color = %q, want %q", restored.Color, "green")
	}
	if got := colorOf(t, svc, "p1", "s1-new"); got != "green" {
		t.Errorf("stored color after resume = %q, want %q", got, "green")
	}
}
