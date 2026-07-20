package terminal

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// waitEmit receives one emitted cwd or fails the test after a grace period.
func waitEmit(t *testing.T, emits <-chan string) string {
	t.Helper()
	select {
	case cwd := <-emits:
		return cwd
	case <-time.After(5 * time.Second):
		t.Fatal("pollCwd emitted nothing")
		return ""
	}
}

// TestProcessCwdReadsSelf proves the /proc read resolves a live process's
// working directory — exercised against the test process itself.
func TestProcessCwdReadsSelf(t *testing.T) {
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd: %v", err)
	}
	if got := processCwd(os.Getpid()); got != wd {
		t.Errorf("processCwd(self) = %q, want %q", got, wd)
	}
}

// TestProcessCwdOfDeadPidIsEmpty proves an unreadable /proc entry degrades to
// "", which pollCwd skips instead of reporting.
func TestProcessCwdOfDeadPidIsEmpty(t *testing.T) {
	// PIDs are capped by /proc/sys/kernel/pid_max (< 2^22 by default); one far
	// beyond it can never name a live process.
	if got := processCwd(1 << 30); got != "" {
		t.Errorf("processCwd(dead) = %q, want empty", got)
	}
}

// TestPollCwdEmitsOnlyOnChange drives pollCwd tick by tick against the test
// process: an unchanged directory stays silent, a chdir is reported exactly
// once, and closing done ends the loop.
func TestPollCwdEmitsOnlyOnChange(t *testing.T) {
	start, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd: %v", err)
	}
	// /proc reports the resolved directory, so compare against the symlink-free
	// form of the temp dir.
	tmp, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatalf("EvalSymlinks: %v", err)
	}

	tick := make(chan time.Time)
	done := make(chan struct{})
	emits := make(chan string, 8)
	finished := make(chan struct{})
	go func() {
		defer close(finished)
		pollCwd(os.Getpid(), start, tick, done, func(cwd string) { emits <- cwd })
	}()

	// Unchanged directory: the tick is consumed without an emit. tick is
	// unbuffered, so each send returns only after the previous one was handled.
	tick <- time.Time{}

	t.Chdir(tmp)
	tick <- time.Time{}
	if got := waitEmit(t, emits); got != tmp {
		t.Errorf("emitted %q, want %q", got, tmp)
	}

	// Same directory again: accepting this tick proves the change above
	// emitted exactly once.
	tick <- time.Time{}
	select {
	case cwd := <-emits:
		t.Errorf("unexpected emit %q for unchanged cwd", cwd)
	default:
	}

	close(done)
	select {
	case <-finished:
	case <-time.After(5 * time.Second):
		t.Fatal("pollCwd did not stop after done closed")
	}
}
