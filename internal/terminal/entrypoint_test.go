package terminal

import (
	"strings"
	"testing"

	"github.com/omartelo/lich/internal/providers"
)

// entrypointSpec is a shell session's spawn before the wrap: the user's shell,
// no arguments, exactly what resolveCommand hands spawnSession.
func entrypointSpec() ptySpec {
	return ptySpec{
		bin:  "/usr/bin/zsh",
		dir:  "/home/user/dev/lich",
		env:  []string{"HOME=/home/user"},
		cols: 80,
		rows: 24,
	}
}

// TestWrapEntrypointLeavesOtherSessionsAlone is the separation this feature is
// built around: an entrypoint reaches the one shell session it was typed on and
// nothing else. The provider cases are the ones that matter — a value that
// arrived on a provider row (a stale row, a caller that skipped the store's own
// WHERE clause) must not rewrite that spawn, because a provider's argv is the
// whole session.
func TestWrapEntrypointLeavesOtherSessionsAlone(t *testing.T) {
	base := entrypointSpec()
	cases := []struct {
		name, kind, entrypoint, goos string
	}{
		{"no entrypoint", KindShell, "", "linux"},
		{"whitespace only", KindShell, "   \n\t ", "linux"},
		{"claude session", providers.Claude, "lazygit", "linux"},
		{"codex session", providers.Codex, "lazygit", "linux"},
		{"opencode session", providers.OpenCode, "lazygit", "linux"},
		{"omp session", providers.OMP, "lazygit", "linux"},
		{"crush session", providers.Crush, "lazygit", "linux"},
		{"windows", KindShell, "lazygit", "windows"},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			got := wrapEntrypoint(base, tt.kind, tt.entrypoint, tt.goos)
			if got.bin != base.bin || len(got.args) != 0 {
				t.Errorf("spawn rewritten: bin=%q args=%v", got.bin, got.args)
			}
		})
	}
}

// TestWrapEntrypointRunsThenExecsTheShell pins the two halves of the contract:
// the command runs, and the shell replaces it afterwards rather than the PTY
// dying with it. The exec argument is quoted, so a shell installed under a path
// with a space is still the shell the user lands in.
func TestWrapEntrypointRunsThenExecsTheShell(t *testing.T) {
	spec := entrypointSpec()
	spec.bin = "/opt/my shells/zsh"

	got := wrapEntrypoint(spec, KindShell, "  lazydocker  ", "linux")

	if got.bin != spec.bin {
		t.Errorf("bin = %q, want the session's own shell %q", got.bin, spec.bin)
	}
	if len(got.args) != 2 || got.args[0] != "-c" {
		t.Fatalf("args = %v, want the shell's own -c", got.args)
	}
	script := got.args[1]
	run := strings.Index(script, "lazydocker")
	exec := strings.Index(script, "exec '/opt/my shells/zsh'")
	if run == -1 || exec == -1 {
		t.Fatalf("script runs the command then execs the shell, got: %q", script)
	}
	if exec < run {
		t.Errorf("the shell replaces the command before it runs: %q", script)
	}
	if got.dir != spec.dir || got.cols != spec.cols || got.rows != spec.rows {
		t.Errorf("wrap changed dir/size: %+v", got)
	}
}

// TestWrapEntrypointSurvivesATrailingComment is why the exec sits on a line of
// its own. A command ending in a comment — or in a `\` continuation — would
// otherwise swallow the exec, and the card would die with the tool instead of
// falling back to a prompt.
func TestWrapEntrypointSurvivesATrailingComment(t *testing.T) {
	for _, entrypoint := range []string{"lazygit # the good one", "pnpm dev \\"} {
		script := wrapEntrypoint(entrypointSpec(), KindShell, entrypoint, "linux").args[1]
		if !strings.Contains(script, "\nexec ") {
			t.Errorf("exec is not on its own line after %q: %q", entrypoint, script)
		}
	}
}

// TestWrapEntrypointComposesWithSetup proves the order a fresh worktree terminal
// carrying both gets: the project's setup script, then the command, then the
// shell — and that wrapSetup's end marker still lands between the script and
// everything the user asked for, so a session is never handed work mid-install.
func TestWrapEntrypointComposesWithSetup(t *testing.T) {
	spec := wrapEntrypoint(entrypointSpec(), KindShell, "lazygit", "linux")
	spec, wrapped := wrapSetup(spec, "pnpm install", "linux")
	if !wrapped {
		t.Fatal("setup wrap skipped")
	}

	script := spec.args[1]
	install := strings.Index(script, "pnpm install")
	marker := strings.Index(script, "lich-setup-done")
	tool := strings.Index(script, "lazygit")
	if install == -1 || marker == -1 || tool == -1 {
		t.Fatalf("composed script lost a stage: %q", script)
	}
	if !(install < marker && marker < tool) {
		t.Errorf("stages out of order (install=%d marker=%d tool=%d): %q", install, marker, tool, script)
	}
}
