package store

import (
	"bytes"
	"log/slog"
	"strings"
	"testing"
	"time"
)

// TestSetSessionScheduleParksOnePromptPerSession pins the shape the card is
// drawn from: one slot, replaced rather than queued, and read back through the
// same hydration call every other session field arrives on.
func TestSetSessionScheduleParksOnePromptPerSession(t *testing.T) {
	svc := newTestStore(t)
	_ = svc.AddProject("p1", "alpha", "/tmp/alpha")
	_ = svc.AddSession("p1", "s1", "Session 1", "claude", "", 2, "")

	if err := svc.SetSessionSchedule("s1", 1700000000, "  run the release checklist\n"); err != nil {
		t.Fatalf("SetSessionSchedule: %v", err)
	}
	if err := svc.SetSessionSchedule("gone", 1700000000, "nobody"); err != nil {
		t.Fatalf("SetSessionSchedule on a missing session errored: %v", err)
	}
	got := onlySession(t, svc)
	if got.ScheduledAt != 1700000000 || got.ScheduledPrompt != "run the release checklist" {
		t.Fatalf("scheduled = %d %q, want the trimmed prompt at 1700000000",
			got.ScheduledAt, got.ScheduledPrompt)
	}

	_ = svc.SetSessionSchedule("s1", 1700000060, "and the changelog")
	if got := onlySession(t, svc); got.ScheduledAt != 1700000060 || got.ScheduledPrompt != "and the changelog" {
		t.Fatalf("scheduled = %d %q, want the second one to have replaced the first",
			got.ScheduledAt, got.ScheduledPrompt)
	}
}

// Both halves have to clear it: the window cancels by sending no time, and a
// prompt of nothing but whitespace is not something to type at anyone.
func TestSetSessionScheduleClears(t *testing.T) {
	for name, clear := range map[string]func(*Service) error{
		"no time":   func(svc *Service) error { return svc.SetSessionSchedule("s1", 0, "still here") },
		"no prompt": func(svc *Service) error { return svc.SetSessionSchedule("s1", 1700000000, "   ") },
	} {
		t.Run(name, func(t *testing.T) {
			svc := newTestStore(t)
			_ = svc.AddProject("p1", "alpha", "/tmp/alpha")
			_ = svc.AddSession("p1", "s1", "Session 1", "claude", "", 2, "")
			_ = svc.SetSessionSchedule("s1", 1700000000, "run it")

			if err := clear(svc); err != nil {
				t.Fatalf("clear: %v", err)
			}
			if got := onlySession(t, svc); got.ScheduledAt != 0 || got.ScheduledPrompt != "" {
				t.Fatalf("scheduled = %d %q, want it cleared", got.ScheduledAt, got.ScheduledPrompt)
			}
		})
	}
}

// The prompt is typed into a TUI a character at a time when it comes due, so
// the size it is refused at is a real boundary — pinned as a literal either
// side of it rather than derived from the constant, which would follow the
// constant wherever it moved.
func TestSetSessionScheduleRefusesAnOversizedPrompt(t *testing.T) {
	svc := newTestStore(t)
	_ = svc.AddProject("p1", "alpha", "/tmp/alpha")
	_ = svc.AddSession("p1", "s1", "Session 1", "claude", "", 2, "")

	if err := svc.SetSessionSchedule("s1", 1700000000, strings.Repeat("x", 8192)); err != nil {
		t.Fatalf("8192 bytes refused: %v", err)
	}
	if err := svc.SetSessionSchedule("s1", 1700000000, strings.Repeat("x", 8193)); err == nil {
		t.Fatal("8193 bytes accepted, want it refused before it reaches a PTY")
	}
	if got := onlySession(t, svc); len(got.ScheduledPrompt) != 8192 {
		t.Fatalf("stored prompt is %d bytes, want the refused one not to have landed",
			len(got.ScheduledPrompt))
	}
}

func onlySession(t *testing.T, svc *Service) Session {
	t.Helper()
	projects, err := svc.LoadState()
	if err != nil {
		t.Fatalf("LoadState: %v", err)
	}
	if len(projects) != 1 || len(projects[0].Sessions) != 1 {
		t.Fatalf("LoadState = %+v, want one project with one session", projects)
	}
	return projects[0].Sessions[0]
}

// A park is not a cancellation: the row is the only copy of the prompt, so the
// resume that re-keys the session under a fresh id has to carry it over — both
// into the database the relay reads and into the row the window redraws the
// card from.
func TestReopenSessionCarriesTheSchedule(t *testing.T) {
	svc := newTestStore(t)
	_ = svc.AddProject("p1", "alpha", "/tmp/alpha")
	_ = svc.AddSession("p1", "base", "Session 1", "claude", "", 2, "")
	_ = svc.AddSession("p1", "wt1", "worker", "claude", "/wt/foo", 3, "")
	_ = svc.SetSessionSchedule("wt1", 1700000000, "run the release checklist")
	_ = svc.CloseSession("p1", "wt1", "base")

	restored, err := svc.ReopenWorktreeSession("p1", "/wt/foo", "wt2")
	if err != nil {
		t.Fatalf("ReopenWorktreeSession: %v", err)
	}
	if restored == nil || restored.ScheduledAt != 1700000000 ||
		restored.ScheduledPrompt != "run the release checklist" {
		t.Fatalf("restored = %+v, want the schedule reported to the window", restored)
	}
	if got := sessionByID(t, svc, "wt2"); got.ScheduledAt != 1700000000 ||
		got.ScheduledPrompt != "run the release checklist" {
		t.Fatalf("resumed row = %d %q, want the prompt still parked on it",
			got.ScheduledAt, got.ScheduledPrompt)
	}
}

// Deleting a session for good is the one exit where the prompt is forfeited
// rather than delayed, and the log line is all that is left of what the user
// wrote — every door that removes a row has to leave one.
func TestDeletingASessionLogsTheForfeitedSchedule(t *testing.T) {
	for name, remove := range map[string]func(*Service) error{
		"delete":  func(svc *Service) error { return svc.DeleteSession("p1", "wt1", "base") },
		"forget":  func(svc *Service) error { _ = svc.CloseSession("p1", "wt1", "base"); return svc.ForgetSession("wt1") },
		"purge":   func(svc *Service) error { return svc.PurgeWorktreeSessions("p1", "/wt/foo") },
		"project": func(svc *Service) error { return svc.DeleteProject("p1") },
	} {
		t.Run(name, func(t *testing.T) {
			svc := newTestStore(t)
			_ = svc.AddProject("p1", "alpha", "/tmp/alpha")
			_ = svc.AddSession("p1", "base", "Session 1", "claude", "", 2, "")
			_ = svc.AddSession("p1", "wt1", "worker", "claude", "/wt/foo", 3, "")
			_ = svc.SetSessionSchedule("wt1", 1700000000, "run the release checklist")
			logged := captureLogs(t)

			if err := remove(svc); err != nil {
				t.Fatalf("remove: %v", err)
			}
			got := logged.String()
			if !strings.Contains(got, "forfeited") || !strings.Contains(got, "worker") ||
				!strings.Contains(got, "run the release checklist") {
				t.Fatalf("log = %q, want the forfeited prompt named with its session", got)
			}
		})
	}
}

// The log is the record; this is the half the user sees. Every door that
// removes a row for good has to reach the person who parked the prompt, because
// the row was the only copy of it.
func TestDeletingASessionReportsTheForfeitedSchedule(t *testing.T) {
	for name, remove := range map[string]func(*Service) error{
		"delete":  func(svc *Service) error { return svc.DeleteSession("p1", "wt1", "base") },
		"forget":  func(svc *Service) error { _ = svc.CloseSession("p1", "wt1", "base"); return svc.ForgetSession("wt1") },
		"purge":   func(svc *Service) error { return svc.PurgeWorktreeSessions("p1", "/wt/foo") },
		"project": func(svc *Service) error { return svc.DeleteProject("p1") },
	} {
		t.Run(name, func(t *testing.T) {
			svc := newTestStore(t)
			var lost []ForfeitedSchedule
			svc.SetScheduleForfeited(func(f ForfeitedSchedule) { lost = append(lost, f) })
			_ = svc.AddProject("p1", "alpha", "/tmp/alpha")
			_ = svc.AddSession("p1", "base", "Session 1", "claude", "", 2, "")
			_ = svc.AddSession("p1", "wt1", "worker", "claude", "/wt/foo", 3, "")
			_ = svc.SetSessionSchedule("wt1", 1700000000, "run the release checklist")

			if err := remove(svc); err != nil {
				t.Fatalf("remove: %v", err)
			}
			if len(lost) != 1 {
				t.Fatalf("reported = %+v, want the one forfeited prompt", lost)
			}
			if lost[0].Label != "worker" || lost[0].At != 1700000000 ||
				lost[0].Prompt != "run the release checklist" {
				t.Fatalf("reported = %+v, want the session, its time and its prompt", lost[0])
			}
		})
	}
}

// The desktop channel is what reaches a forfeit nobody was at the window for,
// and it has two lines to say it in (internal/system, Notify): the session in
// the headline, the prompt and its time under it.
func TestForfeitedScheduleNotice(t *testing.T) {
	summary, detail := ForfeitedSchedule{
		Label: "worker", At: 1700000000, Prompt: "run the release checklist",
	}.Notice()

	if !strings.Contains(summary, `"worker"`) {
		t.Fatalf("summary = %q, want the session named", summary)
	}
	if !strings.Contains(detail, "run the release checklist") {
		t.Fatalf("detail = %q, want the prompt that was lost", detail)
	}
	if want := time.Unix(1700000000, 0).Format(forfeitClock); !strings.Contains(detail, want) {
		t.Fatalf("detail = %q, want the time it was due (%s)", detail, want)
	}
}

// A session with nothing parked on it is removed in silence, and so is a park:
// the row comes back with the card, prompt and all.
func TestClosingOrDeletingWithoutAScheduleReportsNothing(t *testing.T) {
	for name, remove := range map[string]func(*Service) error{
		"nothing parked": func(svc *Service) error {
			return svc.DeleteSession("p1", "wt1", "base")
		},
		"parked, not deleted": func(svc *Service) error {
			_ = svc.SetSessionSchedule("wt1", 1700000000, "run the release checklist")
			return svc.CloseSession("p1", "wt1", "base")
		},
	} {
		t.Run(name, func(t *testing.T) {
			svc := newTestStore(t)
			reported := 0
			svc.SetScheduleForfeited(func(ForfeitedSchedule) { reported++ })
			_ = svc.AddProject("p1", "alpha", "/tmp/alpha")
			_ = svc.AddSession("p1", "base", "Session 1", "claude", "", 2, "")
			_ = svc.AddSession("p1", "wt1", "worker", "claude", "/wt/foo", 3, "")

			if err := remove(svc); err != nil {
				t.Fatalf("remove: %v", err)
			}
			if reported != 0 {
				t.Fatalf("reported %d forfeits, want none", reported)
			}
		})
	}
}

// A session with nothing parked on it is removed in silence: a warning per
// deleted card would bury the one that means something.
func TestDeletingAnUnscheduledSessionLogsNothing(t *testing.T) {
	svc := newTestStore(t)
	_ = svc.AddProject("p1", "alpha", "/tmp/alpha")
	_ = svc.AddSession("p1", "base", "Session 1", "claude", "", 2, "")
	_ = svc.AddSession("p1", "wt1", "worker", "claude", "/wt/foo", 3, "")
	logged := captureLogs(t)

	if err := svc.DeleteSession("p1", "wt1", "base"); err != nil {
		t.Fatalf("DeleteSession: %v", err)
	}
	if got := logged.String(); strings.Contains(got, "forfeited") {
		t.Fatalf("log = %q, want nothing said about a session with no schedule", got)
	}
}

// captureLogs redirects the default logger for one test and hands back what it
// collected.
func captureLogs(t *testing.T) *bytes.Buffer {
	t.Helper()
	var buf bytes.Buffer
	previous := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})))
	t.Cleanup(func() { slog.SetDefault(previous) })
	return &buf
}

func sessionByID(t *testing.T, svc *Service, id string) Session {
	t.Helper()
	projects, err := svc.LoadState()
	if err != nil {
		t.Fatalf("LoadState: %v", err)
	}
	for _, p := range projects {
		for _, sess := range p.Sessions {
			if sess.ID == id {
				return sess
			}
		}
	}
	t.Fatalf("LoadState = %+v, want a session %q", projects, id)
	return Session{}
}
