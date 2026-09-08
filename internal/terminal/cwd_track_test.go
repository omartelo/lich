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

// emitted is one pollCwd publish: the directory it read, and the host standing
// in the way of reading one.
type emitted struct {
	cwd  string
	host string
}

// waitEmit receives one publish or fails the test after a grace period.
func waitEmit(t *testing.T, emits <-chan emitted) emitted {
	t.Helper()
	select {
	case e := <-emits:
		return e
	case <-time.After(5 * time.Second):
		t.Fatal("pollCwd emitted nothing")
		return emitted{}
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

// TestParseTpgidSurvivesComm proves the foreground group is read past a comm
// holding the very characters a naive field split trips on, and that a process
// with no controlling terminal (tpgid -1, what every non-PTY child reports)
// yields no group rather than a negative pid the cwd read would chase.
func TestParseTpgidSurvivesComm(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want int
	}{
		{"plain", "42 (zsh) S 41 42 42 34816 4242 4194304 ...", 4242},
		{"comm with spaces and parens", "42 (sh (deploy)) S 41 42 42 34816 4242 0 ...", 4242},
		{"no controlling terminal", "42 (go) S 41 42 42 0 -1 0 ...", 0},
		{"truncated", "42 (zsh) S 41 42", 0},
		{"no comm close", "42 (zsh S 41 42 42 34816 4242 0", 0},
	}
	for _, tc := range cases {
		if got := parseTpgid([]byte(tc.in)); got != tc.want {
			t.Errorf("%s: parseTpgid = %d, want %d", tc.name, got, tc.want)
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
	got, host := processCwd(os.Getpid())
	if physical(t, got) != physical(t, wd) {
		t.Errorf("processCwd(self) = %q, want %q", got, wd)
	}
	if host != "" {
		t.Errorf("processCwd(self) host = %q, want none for an ordinary process", host)
	}
}

// TestProcessCwdOfDeadPidIsEmpty proves an unresolvable process degrades to
// "", which pollCwd skips instead of reporting.
func TestProcessCwdOfDeadPidIsEmpty(t *testing.T) {
	// Far beyond any real PID space (Linux pid_max < 2^22, macOS ~1e5); Windows
	// simply finds no such process to open.
	if got, host := processCwd(1 << 30); got != "" || host != "" {
		t.Errorf("processCwd(dead) = (%q, %q), want both empty", got, host)
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
	emits := make(chan emitted, 8)
	finished := make(chan struct{})
	self := func() (string, string) { return processCwd(os.Getpid()) }
	go func() {
		defer close(finished)
		pollCwd(start, tick, done, self, func(cwd, host string) {
			emits <- emitted{cwd, host}
		})
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
	if got := waitEmit(t, emits); physical(t, got.cwd) != physical(t, moved) {
		t.Errorf("emitted %q, want %q", got.cwd, moved)
	}

	// Same directory again: accepting this tick proves the change above
	// emitted exactly once.
	tick <- time.Time{}
	select {
	case e := <-emits:
		t.Errorf("unexpected emit %q for unchanged cwd", e.cwd)
	default:
	}

	close(done)
	select {
	case <-finished:
	case <-time.After(5 * time.Second):
		t.Fatal("pollCwd did not stop after done closed")
	}
}

// TestPollCwdPublishesTheHostInsteadOfAPath proves the readout goes unknown the
// moment the shell moves somewhere no reader follows, and comes back when it
// returns. The last local directory must not be published again on the way in:
// it is a real path, and naming it is the whole failure this seam exists to
// end. Driven off a hand-fed read, because the alternative is starting a tmux.
func TestPollCwdPublishesTheHostInsteadOfAPath(t *testing.T) {
	readings := make(chan emitted, 8)
	tick := make(chan time.Time)
	done := make(chan struct{})
	emits := make(chan emitted, 8)
	go pollCwd("/repo", tick, done,
		func() (string, string) { r := <-readings; return r.cwd, r.host },
		func(cwd, host string) { emits <- emitted{cwd, host} })
	t.Cleanup(func() { close(done) })

	// Entering tmux: no directory, a host, and a publish that overwrites /repo.
	readings <- emitted{"", "tmux"}
	tick <- time.Time{}
	if got := waitEmit(t, emits); got != (emitted{"", "tmux"}) {
		t.Errorf("entering tmux emitted %+v, want an empty cwd hosted by tmux", got)
	}

	// Still inside it: the host is not news twice over.
	readings <- emitted{"", "tmux"}
	tick <- time.Time{}
	readings <- emitted{"", "tmux"}
	tick <- time.Time{}
	select {
	case e := <-emits:
		t.Errorf("unexpected emit %+v while the host had not changed", e)
	default:
	}

	// Back at the shell's prompt, in the very directory tmux was started from:
	// a change, because the readout was unknown a tick ago.
	readings <- emitted{"/repo", ""}
	tick <- time.Time{}
	if got := waitEmit(t, emits); got != (emitted{"/repo", ""}) {
		t.Errorf("leaving tmux emitted %+v, want /repo with no host", got)
	}
}

// TestShellHostNamesOnlyKnownHosts pins the list itself: every name on it is
// answered with, an ordinary foreground job is not, and tmux's own comm — which
// carries its role after a colon — still matches.
func TestShellHostNamesOnlyKnownHosts(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"tmux", "tmux"},
		{"tmux: client", "tmux"},
		{"tmux: server", "tmux"},
		{"screen", "screen"},
		{"ssh", "ssh"},
		{"mosh-client", "mosh-client"},
		{"docker", "docker"},
		{"podman", "podman"},
		{"nsenter", "nsenter"},
		{"distrobox-enter", "distrobox-enter"},
		{"toolbox", "toolbox"},
		{"flatpak", "flatpak"},
		// /proc/<pid>/comm comes back with its newline attached.
		{"ssh\n", "ssh"},
		{"zsh", ""},
		{"", ""},
		{"vim", ""},
		// A prefix is not a match: these run right here.
		{"sshd", ""},
		{"dockerd", ""},
		{"tmuxinator", ""},
	}
	for _, tc := range cases {
		if got := shellHost(tc.in); got != tc.want {
			t.Errorf("shellHost(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}
