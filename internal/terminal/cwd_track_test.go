// Exercises the live-cwd read on every platform that implements it: /proc on
// Linux, proc_pidinfo on macOS, the PEB walk on Windows. Always against the
// test process itself, whose cwd t.Chdir controls; expectations compare with
// os.Getwd(), which reads the same kernel state processCwd does.
//go:build linux || darwin || windows

package terminal

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// physical resolves path's symlinks, the form the kernel reports a process
// cwd in. os.Getwd may return the logical path instead — t.Chdir sets $PWD,
// and macOS reaches its temp dir through /var → /private/var — so both sides
// of every comparison go through this.
func physical(t *testing.T, path string) string {
	t.Helper()
	resolved, err := filepath.EvalSymlinks(path)
	if err != nil {
		t.Fatalf("EvalSymlinks(%q): %v", path, err)
	}
	return resolved
}

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

// TestDosPathRunesRejectsUnusableLengths proves a DosPath byte length that no
// UTF-16 buffer can have is refused outright rather than sized down to an
// empty buffer — the Windows walk reads into &buf[0], so a zero-length buffer
// there panics and, on its bare goroutine, takes the whole app with it.
func TestDosPathRunesRejectsUnusableLengths(t *testing.T) {
	cases := []struct {
		name string
		in   uint16
		want int
	}{
		{"empty", 0, 0},
		{"odd", 1, 0},
		{"odd garbage", 4097, 0},
		{"one code unit", 2, 1},
		{"C:\\", 6, 3},
		{"max even", 65534, 32767},
	}
	for _, tc := range cases {
		if got := dosPathRunes(tc.in); got != tc.want {
			t.Errorf("%s: dosPathRunes(%d) = %d, want %d", tc.name, tc.in, got, tc.want)
		}
	}
}

// TestProcessCwdReadsSelf proves the platform read resolves a live process's
// working directory — exercised against the test process itself.
func TestProcessCwdReadsSelf(t *testing.T) {
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd: %v", err)
	}
	if got := processCwd(os.Getpid()); physical(t, got) != physical(t, wd) {
		t.Errorf("processCwd(self) = %q, want %q", got, wd)
	}
}

// TestProcessCwdOfDeadPidIsEmpty proves an unresolvable process degrades to
// "", which pollCwd skips instead of reporting.
func TestProcessCwdOfDeadPidIsEmpty(t *testing.T) {
	// Far beyond any real PID space (Linux pid_max < 2^22, macOS ~1e5); Windows
	// simply finds no such process to open.
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

	t.Chdir(t.TempDir())
	// Expect Getwd rather than the TempDir value (Windows may hand out 8.3
	// short names), resolved to its physical form (see physical).
	moved, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd: %v", err)
	}
	tick <- time.Time{}
	if got := waitEmit(t, emits); physical(t, got) != physical(t, moved) {
		t.Errorf("emitted %q, want %q", got, moved)
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
