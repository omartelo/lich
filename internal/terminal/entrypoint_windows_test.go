//go:build windows

package terminal

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

const (
	entrypointMarker = "lich-entrypoint-ran"
	profileMarker    = "lich-profile-ran"
)

// TestWrapEntrypointLoadsNoProfileOnARealShell is the half only a Windows runner
// can answer. wrapEntrypoint's own test pins the argv; that -NoProfile in it
// actually keeps $PROFILE out of the command's way is PowerShell's behaviour,
// and the parity this feature promises rests on it.
//
// The control run is the load-bearing part: the same argv without -NoProfile
// has to show the profile running. Without it, a profile written to the wrong
// path — PowerShell resolves it per host and per major version — would look
// exactly like a profile correctly skipped, and the test would pass forever
// while proving nothing.
func TestWrapEntrypointLoadsNoProfileOnARealShell(t *testing.T) {
	shell := userShell()
	if shell == "" {
		t.Skip("no PowerShell on $PATH")
	}
	installProfile(t, shell)

	args := wrapEntrypoint(ptySpec{bin: shell}, KindShell, `Write-Output "`+entrypointMarker+`"`, "windows").args

	control := runShell(t, shell, args[1:])
	if !strings.Contains(control, profileMarker) || !strings.Contains(control, entrypointMarker) {
		t.Fatalf("control run reached neither the profile nor the command, so this test proves nothing: %q", control)
	}

	out := runShell(t, shell, args)
	if !strings.Contains(out, entrypointMarker) {
		t.Errorf("the entrypoint did not run: %q", out)
	}
	if strings.Contains(out, profileMarker) {
		t.Errorf("$PROFILE ran despite -NoProfile: %q", out)
	}
}

// installProfile writes a profile at the path this very shell reports, and takes
// it away again. It skips rather than overwrite one that is already there: this
// suite runs on a developer's own machine as readily as on a runner, and their
// PowerShell profile is not ours to clobber.
func installProfile(t *testing.T, shell string) {
	t.Helper()
	path := strings.TrimSpace(runShell(t, shell, []string{"-NoProfile", "-Command", "$PROFILE.CurrentUserCurrentHost"}))
	if path == "" {
		t.Skip("this PowerShell reports no per-user profile path")
	}
	if _, err := os.Stat(path); err == nil {
		t.Skip("a PowerShell profile is already installed here; not overwriting it")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("create profile directory: %v", err)
	}
	if err := os.WriteFile(path, []byte(`Write-Output "`+profileMarker+"\"\n"), 0o644); err != nil {
		t.Fatalf("write profile: %v", err)
	}
	t.Cleanup(func() { os.Remove(path) })
}

// runShell runs the shell to completion and answers with everything it printed.
// stdin is the null device, which is what ends the prompt -NoExit leaves behind;
// the deadline is the backstop for a PowerShell that reads it differently, and
// the output is returned either way because the markers are the answer, not the
// exit status.
func runShell(t *testing.T, shell string, args []string) string {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	var out bytes.Buffer
	cmd := exec.CommandContext(ctx, shell, args...)
	cmd.Stdout = &out
	cmd.Stderr = &out
	if err := cmd.Run(); err != nil && ctx.Err() == nil {
		t.Fatalf("run %s %v: %v (output: %q)", shell, args, err, out.String())
	}
	return out.String()
}
