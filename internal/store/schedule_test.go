package store

import (
	"strings"
	"testing"
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
