package store

import "testing"

// TestSetSessionEntrypointOnlyReachesShellSessions is the write-side half of the
// separation: the entrypoint belongs to one terminal card, and no caller can
// park one on a provider session — where the provider *is* the entrypoint and
// nothing would ever read the value back.
func TestSetSessionEntrypointOnlyReachesShellSessions(t *testing.T) {
	svc := newTestStore(t)
	_ = svc.AddProject("p1", "alpha", "/tmp/alpha")
	_ = svc.AddSession("p1", "term", "Terminal 1", "shell", "", 2, "")
	_ = svc.AddSession("p1", "agent", "Session 1", "claude", "", 3, "")

	if err := svc.SetSessionEntrypoint("term", "lazygit"); err != nil {
		t.Fatalf("SetSessionEntrypoint on a shell session: %v", err)
	}
	if err := svc.SetSessionEntrypoint("agent", "lazygit"); err != nil {
		t.Fatalf("SetSessionEntrypoint on a provider session errored: %v", err)
	}
	if err := svc.SetSessionEntrypoint("gone", "lazygit"); err != nil {
		t.Fatalf("SetSessionEntrypoint on a missing session errored: %v", err)
	}

	if got := svc.SessionEntrypoint("term"); got != "lazygit" {
		t.Errorf("shell session entrypoint = %q, want lazygit", got)
	}
	if got := svc.SessionEntrypoint("agent"); got != "" {
		t.Errorf("provider session entrypoint = %q, want it refused", got)
	}
	if got := svc.SessionEntrypoint("gone"); got != "" {
		t.Errorf("missing session entrypoint = %q, want empty", got)
	}
}

// TestSetSessionEntrypointTrimsAndClears pins the two ends of the dialog's one
// field: a pasted command keeps no surrounding whitespace (it is spliced into a
// shell script), and emptying the field is how a card goes back to a plain
// shell — there is no separate clear.
func TestSetSessionEntrypointTrimsAndClears(t *testing.T) {
	svc := newTestStore(t)
	_ = svc.AddProject("p1", "alpha", "/tmp/alpha")
	_ = svc.AddSession("p1", "term", "Terminal 1", "shell", "", 2, "")

	_ = svc.SetSessionEntrypoint("term", "  lazydocker\n")
	if got := svc.SessionEntrypoint("term"); got != "lazydocker" {
		t.Errorf("entrypoint = %q, want it trimmed", got)
	}

	_ = svc.SetSessionEntrypoint("term", "   ")
	if got := svc.SessionEntrypoint("term"); got != "" {
		t.Errorf("entrypoint = %q, want a blank field to clear it", got)
	}
}

// TestSessionsCarryTheirEntrypoint proves the window is told which terminals run
// something. The card reads it to prefill the dialog and to name what a renamed
// terminal actually runs, so a workspace that restores without it loses the
// answer for every card the user renamed.
func TestSessionsCarryTheirEntrypoint(t *testing.T) {
	svc := newTestStore(t)
	_ = svc.AddProject("p1", "alpha", "/tmp/alpha")
	_ = svc.AddSession("p1", "term", "Terminal 1", "shell", "", 2, "")
	_ = svc.SetSessionEntrypoint("term", "k9s")

	projects, err := svc.LoadState()
	if err != nil {
		t.Fatalf("LoadState: %v", err)
	}
	if len(projects) != 1 || len(projects[0].Sessions) != 1 {
		t.Fatalf("LoadState = %+v, want one project with one session", projects)
	}
	if got := projects[0].Sessions[0].Entrypoint; got != "k9s" {
		t.Errorf("restored entrypoint = %q, want k9s", got)
	}
}

// TestReopenWorktreeSessionCarriesSpawnOverrides pins the two overrides a resume
// used to drop. Both are documented to survive every later spawn of a session,
// and the park/resume cycle is the one path that keeps a card's identity while
// changing its id — so a reinsert that forgot them put the provider back on its
// default model and the terminal back on a bare shell, with nothing on screen
// saying so.
func TestReopenWorktreeSessionCarriesSpawnOverrides(t *testing.T) {
	svc := newTestStore(t)
	_ = svc.AddProject("p1", "alpha", "/tmp/alpha")
	_ = svc.AddSession("p1", "base", "Session 1", "claude", "", 2, "")

	_ = svc.AddSession("p1", "wt1", "worker", "claude", "/wt/foo", 3, "")
	_ = svc.SetSessionModel("wt1", "opus")
	_ = svc.CloseSession("p1", "wt1", "base")

	_ = svc.AddSession("p1", "sh1", "Terminal 1", "shell", "/wt/bar", 4, "")
	_ = svc.SetSessionEntrypoint("sh1", "lazygit")
	_ = svc.CloseSession("p1", "sh1", "base")

	if _, err := svc.ReopenWorktreeSession("p1", "/wt/foo", "wt2"); err != nil {
		t.Fatalf("ReopenWorktreeSession (provider): %v", err)
	}
	if got := svc.SessionModel("wt2"); got != "opus" {
		t.Errorf("resumed model = %q, want opus", got)
	}

	restored, err := svc.ReopenWorktreeSession("p1", "/wt/bar", "sh2")
	if err != nil {
		t.Fatalf("ReopenWorktreeSession (shell): %v", err)
	}
	if got := svc.SessionEntrypoint("sh2"); got != "lazygit" {
		t.Errorf("resumed entrypoint = %q, want lazygit", got)
	}
	if restored == nil || restored.Entrypoint != "lazygit" {
		t.Errorf("restored = %+v, want the entrypoint reported to the window", restored)
	}
}

// TestSetRunEntrypointMarksTheRunCard is the write behind the one Run card a
// checkout gets (internal/spawn.Run): the command and the mark land together, so
// the window can find that card again by the row rather than by guessing at an
// entrypoint. A terminal the user aimed at a command by hand stays unmarked and
// holds no slot.
func TestSetRunEntrypointMarksTheRunCard(t *testing.T) {
	svc := newTestStore(t)
	_ = svc.AddProject("p1", "alpha", "/tmp/alpha")
	_ = svc.AddSession("p1", "run", "task dev", "shell", "/wt/foo", 2, "")
	_ = svc.AddSession("p1", "term", "Terminal 1", "shell", "", 3, "")
	_ = svc.AddSession("p1", "agent", "Session 1", "claude", "", 4, "")

	if err := svc.SetRunEntrypoint("run", "  task dev\n"); err != nil {
		t.Fatalf("SetRunEntrypoint: %v", err)
	}
	if err := svc.SetRunEntrypoint("agent", "task dev"); err != nil {
		t.Fatalf("SetRunEntrypoint on a provider session errored: %v", err)
	}
	_ = svc.SetSessionEntrypoint("term", "lazygit")

	projects, err := svc.LoadState()
	if err != nil {
		t.Fatalf("LoadState: %v", err)
	}
	marks := map[string]bool{}
	commands := map[string]string{}
	for _, sess := range projects[0].Sessions {
		marks[sess.ID] = sess.Run
		commands[sess.ID] = sess.Entrypoint
	}
	if !marks["run"] || commands["run"] != "task dev" {
		t.Errorf("run card = (%v, %q), want marked and trimmed", marks["run"], commands["run"])
	}
	if marks["term"] || commands["term"] != "lazygit" {
		t.Errorf("hand-set terminal = (%v, %q), want the command without the mark",
			marks["term"], commands["term"])
	}
	if marks["agent"] {
		t.Error("provider session was marked a run card; the kind clause should refuse it")
	}
}

// TestReopenCarriesTheRunMark: a resumed Run card is still its checkout's, or the
// next Run would open a second one onto the same port.
func TestReopenCarriesTheRunMark(t *testing.T) {
	svc := newTestStore(t)
	_ = svc.AddProject("p1", "alpha", "/tmp/alpha")
	_ = svc.AddSession("p1", "base", "Session 1", "claude", "", 2, "")
	_ = svc.AddSession("p1", "run", "task dev", "shell", "/wt/foo", 3, "")
	_ = svc.SetRunEntrypoint("run", "task dev")
	_ = svc.CloseSession("p1", "run", "base")

	restored, err := svc.ReopenWorktreeSession("p1", "/wt/foo", "run2")
	if err != nil {
		t.Fatalf("ReopenWorktreeSession: %v", err)
	}
	if restored == nil || !restored.Run {
		t.Fatalf("restored = %+v, want the run mark reported to the window", restored)
	}
	projects, err := svc.LoadState()
	if err != nil {
		t.Fatalf("LoadState: %v", err)
	}
	for _, sess := range projects[0].Sessions {
		if sess.ID == "run2" && !sess.Run {
			t.Error("resumed row lost its run mark")
		}
	}
}
