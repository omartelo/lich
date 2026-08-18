package terminal

import (
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

// The binaries a confined session has to reach are the one it runs and the lich
// binary it calls back through, both resolved to a real path.
func TestExecutablesResolveTheSpawnAndLich(t *testing.T) {
	paths := executables("sh")
	if len(paths) == 0 {
		t.Fatal("sh resolved to nothing")
	}
	for _, path := range paths {
		if path == "sh" {
			t.Errorf("an unresolved name reached the mount list: %v", paths)
		}
	}
	if got := executables("lich-no-such-binary-anywhere"); len(got) != 0 && got[0] != "" {
		for _, path := range got {
			if path == "lich-no-such-binary-anywhere" {
				t.Errorf("a binary that is not on PATH reached the mount list: %v", got)
			}
		}
	}
}

// The verdict is written back on every spawn, including for a session nobody
// was asked about — it is what the card reads to draw its mark, and what keeps a
// session running the way it opened once the rung moves under it.
func TestConfinedRecordsItsVerdict(t *testing.T) {
	tests := []struct {
		row  string
		rung bool
		want string
	}{
		{"", true, store.SessionConfined},
		{"", false, store.SessionUnconfined},
		{store.SessionConfined, false, store.SessionConfined},
		{store.SessionUnconfined, true, store.SessionUnconfined},
	}
	for _, tt := range tests {
		recorded := ""
		s := stubSandboxStore{row: tt.row, rungAnswer: tt.rung, recorded: &recorded}
		confined(s, "s1", providers.Claude, "p1", "/work/wt")
		if recorded != tt.want {
			t.Errorf("row %q rung %v recorded %q, want %q", tt.row, tt.rung, recorded, tt.want)
		}
	}
}
