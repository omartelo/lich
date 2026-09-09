package terminal

import (
	"strings"
	"testing"

	"github.com/omartelo/lich/internal/project"
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

// TestSetupSkippedNotice pins the line a Windows session gets in place of the
// setup it will not run, and the pairing that makes it honest: the script
// wrapSetup refuses is exactly the script this announces, so neither OS can end
// up both skipping the setup and saying nothing about it.
func TestSetupSkippedNotice(t *testing.T) {
	for _, tt := range []struct{ script, goos string }{
		{"pnpm i", "linux"},
		{"pnpm i", "darwin"},
		{"", "windows"},
		{"", "linux"},
	} {
		if got := setupSkippedNotice(tt.script, tt.goos); got != "" {
			t.Errorf("setupSkippedNotice(%q, %q) = %q, want silence", tt.script, tt.goos, got)
		}
	}

	got := setupSkippedNotice("pnpm i", "windows")
	for _, want := range []string{
		"[lich] setup skipped:",
		project.SetupScriptPath,
		"PowerShell",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("notice %q is missing %q", got, want)
		}
	}
	// A PTY's newline: without the carriage return the provider draws from the
	// column the notice ended in.
	if !strings.HasSuffix(got, "\r\n") {
		t.Errorf("notice %q does not end a PTY line", got)
	}

	for _, goos := range []string{"linux", "darwin", "windows"} {
		_, wrapped := wrapSetup(ptySpec{bin: "claude"}, "pnpm i", goos)
		if wrapped == (setupSkippedNotice("pnpm i", goos) != "") {
			t.Errorf("goos %q: wrapped=%v and the notice disagree about who speaks", goos, wrapped)
		}
	}
}
