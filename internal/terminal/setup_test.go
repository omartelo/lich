package terminal

import (
	"strings"
	"testing"
)

// TestWrapSetup proves the wrap decision table: no script or Windows leaves
// the spec untouched, otherwise the spawn becomes sh -c with the script in a
// subshell and the original argv exec'd after it, quoted against spaces and
// embedded quotes.
func TestWrapSetup(t *testing.T) {
	base := ptySpec{
		bin:  "/opt/claude bin/claude",
		args: []string{"--resume", "it's-an-id"},
		dir:  "/wt",
		env:  []string{"HOME=/home/user"},
		cols: 80,
		rows: 24,
	}

	if got, wrapped := wrapSetup(base, "", "linux"); got.bin != base.bin || len(got.args) != 2 || wrapped {
		t.Errorf("empty script rewrote the spec: %+v (wrapped=%v)", got, wrapped)
	}
	if got, wrapped := wrapSetup(base, "pnpm i", "windows"); got.bin != base.bin || wrapped {
		t.Errorf("windows rewrote the spec: %+v (wrapped=%v)", got, wrapped)
	}

	got, wrapped := wrapSetup(base, "pnpm i", "linux")
	if got.bin != "sh" || len(got.args) != 2 || got.args[0] != "-c" || !wrapped {
		t.Fatalf("wrapped spec = %+v (wrapped=%v), want sh -c", got, wrapped)
	}
	cmd := got.args[1]
	for _, want := range []string{
		"pnpm i",
		"exec '/opt/claude bin/claude' '--resume' 'it'\\''s-an-id'",
		"[lich] worktree setup failed",
	} {
		if !strings.Contains(cmd, want) {
			t.Errorf("wrapped command missing %q:\n%s", want, cmd)
		}
	}
	if got.dir != base.dir || got.cols != base.cols || got.rows != base.rows {
		t.Errorf("wrap changed dir/size: %+v", got)
	}
}

// TestSetupWrapperMarksItsEnd pins the marker the wrapper prints between the
// script and the provider. It is the only thing that tells lich the two apart —
// the PTY and the pid are the same across the exec — and a session still
// running its setup must not be handed work (see Service.Ready).
func TestSetupWrapperMarksItsEnd(t *testing.T) {
	spec, _ := wrapSetup(ptySpec{bin: "claude", args: []string{"--name", "x"}}, "pnpm install", "linux")

	script := spec.args[1]
	marker := strings.Index(script, `printf '\033]6969;lich-setup-done\007'`)
	if marker == -1 {
		t.Fatalf("the wrapper prints no end marker: %q", script)
	}
	// Before the exec, and after the script: a marker on the wrong side of
	// either would report an agent that has not started, or never report one.
	if exec := strings.Index(script, "exec "); exec < marker {
		t.Errorf("the marker is printed after the exec that replaces this shell: %q", script)
	}
	if install := strings.Index(script, "pnpm install"); install > marker {
		t.Errorf("the marker is printed before the script runs: %q", script)
	}
}

// The escaped form printf receives has to produce the bytes the service scans
// for; two spellings of one sequence would mean a marker nobody ever matches.
func TestSetupMarkerSpellingsAgree(t *testing.T) {
	unescaped := strings.ReplaceAll(setupDoneEscaped, `\033`, "\x1b")
	unescaped = strings.ReplaceAll(unescaped, `\007`, "\x07")
	if unescaped != setupDone {
		t.Errorf("printf writes %q, the service watches for %q", unescaped, setupDone)
	}
}
