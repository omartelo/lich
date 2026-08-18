package terminal

import (
	"os"
	"os/exec"
	"path/filepath"
	"slices"
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
		got := wrapSandbox(baseSpec(), providers.Claude, tt.home, tt.confined)
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
	got := wrapSandbox(baseSpec(), providers.Claude, t.TempDir(), true)
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
