package terminal

import (
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/omartelo/lich/internal/providers"
	"github.com/omartelo/lich/internal/sandbox"
	"github.com/omartelo/lich/internal/store"
)

// stubSandboxStore answers what confined asks, and records what it writes back.
type stubSandboxStore struct {
	row        string
	rungAnswer bool
	recorded   *string
}

func (s stubSandboxStore) SessionSandbox(string) string       { return s.row }
func (s stubSandboxStore) SandboxDefault(_, _, _ string) bool { return s.rungAnswer }

func (s stubSandboxStore) SetSessionSandbox(_, sandbox string) error {
	if s.recorded != nil {
		*s.recorded = sandbox
	}
	return nil
}

// The row wins in both directions: a session opened confined stays confined
// when the rung is turned off under it, and one opened on the machine is not
// confined later because the rung moved.
func TestConfinedPrefersTheSessionRow(t *testing.T) {
	tests := []struct {
		row  string
		rung bool
		want bool
	}{
		{"", false, false},
		{"", true, true},
		{store.SessionConfined, false, true},
		{store.SessionUnconfined, true, false},
		// A value no reader understands is not an answer, so the rung decides.
		{"maybe", true, true},
	}
	for _, tt := range tests {
		got := confined(stubSandboxStore{row: tt.row, rungAnswer: tt.rung}, "s1", providers.Claude, "p1", "/work/wt")
		if got != tt.want {
			t.Errorf("confined(row %q, rung %v) = %v, want %v", tt.row, tt.rung, got, tt.want)
		}
	}
}

func baseSpec() ptySpec {
	return ptySpec{bin: "/bin/sh", args: []string{"-c", "exec claude"}, dir: t0dir}
}

// t0dir is a directory every platform has, so the spec under test names
// something real without a fixture.
const t0dir = "/tmp"

func TestWrapSandboxLeavesAnUnconfinedSpawnAlone(t *testing.T) {
	original := baseSpec()
	for _, tt := range []struct {
		name     string
		home     string
		confined bool
	}{
		{"not confined", "/home/u", false},
		{"no home to empty", "", true},
	} {
		got := wrapSandbox(baseSpec(), providers.Claude, tt.home, "", tt.confined)
		if got.bin != original.bin || !slices.Equal(got.args, original.args) {
			t.Errorf("%s: spawn rewritten to %q %v", tt.name, got.bin, got.args)
		}
	}
}

func TestWrapSandboxKeepsTheCommandAndItsArguments(t *testing.T) {
	if !sandbox.Available() {
		t.Skip("no sandbox backend on this machine")
	}
	original := baseSpec()
	got := wrapSandbox(baseSpec(), providers.Claude, t.TempDir(), "", true)
	if got.bin == original.bin {
		t.Fatalf("the spawn was not confined: still %q", got.bin)
	}
	// The provider's own argv has to survive intact and in order, at the end —
	// an argument absorbed by the sandbox binary is a session that never starts.
	// The last occurrence, not the first: the binary is also mounted into the
	// sandbox, so its path appears in the mount list before it appears as the
	// command.
	at := lastIndex(got.args, original.bin)
	if at < 0 {
		t.Fatalf("%q is not in the confined argv: %v", original.bin, got.args)
	}
	if want := append([]string{original.bin}, original.args...); !slices.Equal(got.args[at:], want) {
		t.Errorf("confined argv tail = %v, want %v", got.args[at:], want)
	}
	// Everything else about the spawn belongs to the PTY, not to the sandbox.
	if got.dir != original.dir {
		t.Errorf("start directory moved to %q", got.dir)
	}
}

// lastIndex is the position of the final element equal to value, or -1.
func lastIndex(args []string, value string) int {
	for i := len(args) - 1; i >= 0; i-- {
		if args[i] == value {
			return i
		}
	}
	return -1
}

// What a confined spawn needs mounted is the directory holding each binary, not
// the binary — binding the executable itself is what fails the spawn when it is
// a symlink under a directory already mounted (sandbox.BinaryDirs). The missing
// binary case is BinaryDirs' own (TestBinaryDirsOfAMissingBinary); what is here
// is that this caller asks for both binaries and passes the answer through.
func TestExecutablesAreDirectoriesOnDisk(t *testing.T) {
	dirs := executables("sh")
	if len(dirs) == 0 {
		t.Fatal("sh resolved to nothing")
	}
	for _, dir := range dirs {
		info, err := os.Stat(dir)
		if err != nil {
			t.Errorf("%q is not on disk: %v", dir, err)
			continue
		}
		if !info.IsDir() {
			t.Errorf("%q is a file; a mount of it would fail the spawn", dir)
		}
	}
	// The shell's own directory, and the lich binary's — the session calls back
	// through it, and its MCP server runs it.
	shell, err := exec.LookPath("sh")
	if err != nil {
		t.Skip("no sh on PATH")
	}
	if !slices.Contains(dirs, filepath.Dir(shell)) {
		t.Errorf("the spawned binary's directory is missing from %v", dirs)
	}
	if lich := lichBin(); lich != "" {
		if !slices.Contains(dirs, filepath.Dir(lich)) {
			t.Errorf("the lich binary's directory is missing from %v", dirs)
		}
	}
}

// TestSessionDropDirCreatesTheDirectory: the bind is built when the PTY spawns
// and the first file is dropped long after, so the directory has to be there
// already — a bind of a source that does not exist is dropped from the spec,
// and the copy would then land where the session cannot read it.
func TestSessionDropDirCreatesTheDirectory(t *testing.T) {
	copies := t.TempDir()

	got := sessionDropDir(copies, "s1", true)

	if want := filepath.Join(copies, "s1"); got != want {
		t.Fatalf("sessionDropDir = %q, want %q", got, want)
	}
	if info, err := os.Stat(got); err != nil || !info.IsDir() {
		t.Fatalf("sessionDropDir did not create %s (%v)", got, err)
	}
}

// A session with nowhere to keep copies still spawns: it loses its drops, never
// its card. An unconfined one wants no directory at all — it opens the copy at
// the path it was written to, like any other file.
func TestSessionDropDirWithoutABase(t *testing.T) {
	for _, tt := range []struct {
		base, session string
		confined      bool
	}{
		{"", "s1", true},
		{t.TempDir(), "", true},
		{t.TempDir(), "s1", false},
	} {
		if got := sessionDropDir(tt.base, tt.session, tt.confined); got != "" {
			t.Errorf("sessionDropDir(%q, %q, %v) = %q, want none", tt.base, tt.session, tt.confined, got)
		}
	}
	base := t.TempDir()
	if _, err := os.Stat(filepath.Join(base, "s1")); err == nil {
		t.Error("an unconfined session left a copies directory behind")
	}
}

// TestWrapSandboxMountsTheSessionsCopies is the drop's half of the sandbox: a
// file dragged onto a confined terminal is copied by lich outside it, so the
// path pasted at the prompt only names something the agent can open if the
// directory holding it is bound in.
func TestWrapSandboxMountsTheSessionsCopies(t *testing.T) {
	if !sandbox.Available() {
		t.Skip("no sandbox backend on this machine")
	}
	copies := sessionDropDir(t.TempDir(), "s1", true)

	got := wrapSandbox(baseSpec(), providers.Claude, t.TempDir(), copies, true)

	// Substring, not an argv element: the two backends spell a mounted path in
	// their own vocabulary — bubblewrap takes it as its own argument
	// (`--ro-bind dir dir`), seatbelt writes it into the profile it is handed as
	// one string (`(allow file-read* (subpath "dir"))`). What the spawn has to
	// carry is the path, not a shape only one of them uses.
	if !slices.ContainsFunc(got.args, func(arg string) bool { return strings.Contains(arg, copies) }) {
		t.Fatalf("the session's copies dir %q is not in the confined spawn: %v", copies, got.args)
	}
}
