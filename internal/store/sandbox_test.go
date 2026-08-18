package store

import (
	"testing"

	"github.com/omartelo/lich/internal/providers"
)

// sandboxProject is a store holding one project rooted at path, which is what
// the worktree scope is measured against.
func sandboxProject(t *testing.T, path string) *Service {
	t.Helper()
	svc := newTestStore(t)
	if err := svc.AddProject("p1", "alpha", path); err != nil {
		t.Fatalf("AddProject: %v", err)
	}
	return svc
}

func TestSandboxLevelPrefersTheProjectOverTheGlobal(t *testing.T) {
	svc := sandboxProject(t, "/work/alpha")
	key := sandboxKey(providers.Claude)

	if got := svc.SandboxLevel(providers.Claude, "p1"); got != SandboxOff {
		t.Errorf("unset level = %q, want %q", got, SandboxOff)
	}
	if err := svc.SetSetting(key, "", SandboxEverywhere); err != nil {
		t.Fatalf("SetSetting: %v", err)
	}
	if got := svc.SandboxLevel(providers.Claude, "p1"); got != SandboxEverywhere {
		t.Errorf("global level = %q, want %q", got, SandboxEverywhere)
	}
	if err := svc.SetSetting(key, "p1", SandboxWorktrees); err != nil {
		t.Fatalf("SetSetting: %v", err)
	}
	if got := svc.SandboxLevel(providers.Claude, "p1"); got != SandboxWorktrees {
		t.Errorf("project level = %q, want %q", got, SandboxWorktrees)
	}
	// Another project still reads the global value.
	if got := svc.SandboxLevel(providers.Claude, "p2"); got != SandboxEverywhere {
		t.Errorf("sibling project level = %q, want %q", got, SandboxEverywhere)
	}
}

// A value from some other feature, a typo, or a rung that used to exist must
// read as off — never as a rung that confines a session nobody asked to confine.
func TestSandboxLevelRejectsUnknownValues(t *testing.T) {
	svc := sandboxProject(t, "/work/alpha")
	for _, value := range []string{"true", "on", "ON", "Everywhere", "always", " worktrees"} {
		if err := svc.SetSetting(sandboxKey(providers.Codex), "", value); err != nil {
			t.Fatalf("SetSetting: %v", err)
		}
		if got := svc.SandboxLevel(providers.Codex, "p1"); got != SandboxOff {
			t.Errorf("level %q read as %q, want %q", value, got, SandboxOff)
		}
	}
}

func TestSandboxDefaultPerRungAndCheckout(t *testing.T) {
	const root = "/work/alpha"
	tests := []struct {
		level    string
		cwd      string
		confined bool
	}{
		{SandboxOff, root, false},
		{SandboxOff, "/work/wt", false},
		{SandboxAsk, root, false},
		// Ask hands the decision to whoever opens the session. A caller with
		// nowhere to ask gets the answer closing the dialog would give.
		{SandboxAsk, "/work/wt", false},
		{SandboxWorktrees, root, false},
		{SandboxWorktrees, root + "/", false},
		{SandboxWorktrees, "/work/wt", true},
		{SandboxEverywhere, root, true},
		{SandboxEverywhere, "/work/wt", true},
	}
	for _, tt := range tests {
		svc := sandboxProject(t, root)
		if err := svc.SetSetting(sandboxKey(providers.Claude), "", tt.level); err != nil {
			t.Fatalf("SetSetting: %v", err)
		}
		if got := svc.SandboxDefault(providers.Claude, "p1", tt.cwd); got != tt.confined {
			t.Errorf("rung %q in %q = %v, want %v", tt.level, tt.cwd, got, tt.confined)
		}
	}
}

// A project whose path cannot be read cannot be told apart from its worktrees,
// and the worktree rung is the one that would confine on that guess.
func TestSandboxDefaultWithoutAProjectPath(t *testing.T) {
	svc := newTestStore(t)
	if err := svc.SetSetting(sandboxKey(providers.Claude), "", SandboxWorktrees); err != nil {
		t.Fatalf("SetSetting: %v", err)
	}
	if svc.SandboxDefault(providers.Claude, "missing", "/work/wt") {
		t.Error("a session whose project lich cannot place was confined on a guess")
	}
}

func TestSessionSandboxRoundTrip(t *testing.T) {
	svc := sandboxProject(t, "/work/alpha")
	if err := svc.AddSession("p1", "s1", "one", providers.Claude, "", 0, ""); err != nil {
		t.Fatalf("AddSession: %v", err)
	}
	if got := svc.SessionSandbox("s1"); got != "" {
		t.Errorf("a new session = %q, want no override", got)
	}
	for _, want := range []string{SessionConfined, SessionUnconfined} {
		if err := svc.SetSessionSandbox("s1", want); err != nil {
			t.Fatalf("SetSessionSandbox(%q): %v", want, err)
		}
		if got := svc.SessionSandbox("s1"); got != want {
			t.Errorf("SessionSandbox = %q, want %q", got, want)
		}
	}
	// Anything else clears the override rather than parking a value no reader
	// understands.
	if err := svc.SetSessionSandbox("s1", "maybe"); err != nil {
		t.Fatalf("SetSessionSandbox: %v", err)
	}
	if got := svc.SessionSandbox("s1"); got != "" {
		t.Errorf("an unknown answer stored as %q, want cleared", got)
	}
}

// The answer arrives with the insert because the PTY reads it on the first
// spawn: written afterwards it would race the card the insert puts on screen.
func TestAddSessionCarriesTheSandboxAnswer(t *testing.T) {
	svc := sandboxProject(t, "/work/alpha")
	if err := svc.AddSession("p1", "s1", "one", providers.Claude, "", 0, SessionConfined); err != nil {
		t.Fatalf("AddSession: %v", err)
	}
	if got := svc.SessionSandbox("s1"); got != SessionConfined {
		t.Errorf("SessionSandbox = %q, want %q", got, SessionConfined)
	}
	// An unusable answer is stored as no answer, so the rung decides rather than
	// a value no reader understands.
	if err := svc.AddSession("p1", "s2", "two", providers.Claude, "", 1, "yes please"); err != nil {
		t.Fatalf("AddSession: %v", err)
	}
	if got := svc.SessionSandbox("s2"); got != "" {
		t.Errorf("SessionSandbox = %q, want cleared", got)
	}
}

// A delegated session is opened by another session, which has nobody to ask.
func TestAddSessionFromLeavesTheAnswerToTheRung(t *testing.T) {
	svc := sandboxProject(t, "/work/alpha")
	if err := svc.AddSessionFrom("p1", "s1", "one", providers.Claude, "", 0, "parent", "Parent"); err != nil {
		t.Fatalf("AddSessionFrom: %v", err)
	}
	if got := svc.SessionSandbox("s1"); got != "" {
		t.Errorf("a delegated session recorded %q, want no override", got)
	}
}

func TestSessionSandboxOfAMissingRowIsNotAnError(t *testing.T) {
	svc := sandboxProject(t, "/work/alpha")
	if err := svc.SetSessionSandbox("gone", SessionConfined); err != nil {
		t.Errorf("SetSessionSandbox on a missing row: %v", err)
	}
	if got := svc.SessionSandbox("gone"); got != "" {
		t.Errorf("SessionSandbox of a missing row = %q, want \"\"", got)
	}
}
