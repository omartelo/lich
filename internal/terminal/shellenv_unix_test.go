//go:build !windows

package terminal

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// TestRunShellDumpTimeoutDegrades pins the pty trap: a pty master blocks on
// Read until the child exits, so a hung interactive shell (a stuck rc
// script, a shell dropped into its own prompt) must still be cut off by ctx
// rather than hanging ResolveShellEnv past its timeout ceiling.
func TestRunShellDumpTimeoutDegrades(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	start := time.Now()
	_, _, _ = runShellDump(ctx, "/bin/sh", "sleep 30", os.Environ())
	if elapsed := time.Since(start); elapsed > 2*time.Second {
		t.Fatalf("runShellDump took %v, want it cut off near the 200ms ctx deadline", elapsed)
	}
}

// TestRunShellDumpBackgroundJobDoesNotBlockDeadline pins the trap a killed
// shell alone does not cover: an rc file that backgrounds a job (an
// ssh-agent, a prompt tool) leaves its stdio wired to the pty, so the slave
// end stays open — and a read waiting on EOF blocks on it — after the shell
// we spawned has already exited. Measured before this test existed: with
// only the shell process killed on ctx, the read did not return until the
// backgrounded sleep itself did, 30s later, ignoring a 200ms ctx entirely.
func TestRunShellDumpBackgroundJobDoesNotBlockDeadline(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	start := time.Now()
	_, _, _ = runShellDump(ctx, "/bin/sh", "(sleep 30 &); exit 0", os.Environ())
	if elapsed := time.Since(start); elapsed > 2*time.Second {
		t.Fatalf("runShellDump took %v, want it cut off near the 200ms ctx deadline despite the backgrounded job", elapsed)
	}
}

// TestRunShellDumpBackgroundJobStillYieldsOutput pins the other half: a
// background job must not cost the resolution its output. The dump has
// already been fully printed before the job is backgrounded, so the quiet
// window (shellDumpQuiet) — not the far longer ctx — is what ends the read,
// and what it returns still carries what the shell said.
func TestRunShellDumpBackgroundJobStillYieldsOutput(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	start := time.Now()
	out, _, err := runShellDump(ctx, "/bin/sh", "echo REAL_OUTPUT=yes; (sleep 30 &); exit 0", os.Environ())
	elapsed := time.Since(start)
	if err != nil {
		t.Fatalf("runShellDump error: %v", err)
	}
	if !strings.Contains(out, "REAL_OUTPUT=yes") {
		t.Fatalf("output missing despite the backgrounded job: %q", out)
	}
	if elapsed > 2*time.Second {
		t.Fatalf("runShellDump took %v, want it end near the %v quiet window rather than the 5s ctx", elapsed, shellDumpQuiet)
	}
}

func TestRunShellDumpSurfacesShellFailure(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if _, _, err := runShellDump(ctx, "/bin/sh", "exit 7", os.Environ()); err == nil {
		t.Fatal("runShellDump: want the shell's exit error")
	}
}

// TestReresolveShellEnvSingleFlightWhileReaderParked pins the bound on the
// leak: an rc that backgrounds a job holds the pty open after the shell itself
// has gone, so the reader stays parked and uninterruptible. A second press must
// be refused there and then rather than spawn a second shell — one outstanding
// reader is the ceiling, however many times the button is pressed.
func TestReresolveShellEnvSingleFlightWhileReaderParked(t *testing.T) {
	t.Cleanup(func() { noteParkedReader(nil) })
	// Absolute: the resolution hands the shell the base env, whose PATH is the
	// one being replaced and resolves nothing.
	sleep, err := exec.LookPath("sleep")
	if err != nil {
		t.Skip("sleep not installed")
	}
	dir := t.TempDir()
	spawns := filepath.Join(dir, "spawns")
	fake := filepath.Join(dir, "fakeshell")
	// The dump is printed in full and then the pty is held open — the shape a
	// prompt tool or agent an rc hands off to leaves behind. The quiet window
	// ends the read 300ms later with its reader still blocked on that pty.
	// Held by the shell process itself rather than by a job it backgrounds:
	// whether a background job keeps the slave open is the shell's and the
	// platform's business, and this is the same edge either way.
	script := "#!/bin/sh\n" +
		"echo x >> " + spawns + "\n" +
		"echo " + shellEnvSentinel + "\n" +
		"echo PATH=/late/install\n" +
		"exec " + sleep + " 30\n"
	if err := os.WriteFile(fake, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("SHELL", fake)

	if _, err := ReresolveShellEnv([]string{"PATH=/orig"}); err != nil {
		t.Fatalf("first resolution: %v", err)
	}
	if _, err := ReresolveShellEnv([]string{"PATH=/orig"}); err == nil {
		t.Fatal("second resolution: want a refusal while the reader is parked")
	}

	if got := spawnCount(t, spawns); got != 1 {
		t.Fatalf("the shell ran %d times, want 1", got)
	}
}

// TestShellDumpParkedClearsWhenCollected: the refusal lasts exactly as long as
// the read does. Whatever holds the pty letting go is the only thing that ends
// one, and nothing reports it — so the next attempt is where it is noticed.
func TestShellDumpParkedClearsWhenCollected(t *testing.T) {
	t.Cleanup(func() { noteParkedReader(nil) })
	collected := make(chan struct{})
	noteParkedReader(collected)

	if !shellDumpParked() {
		t.Fatal("shellDumpParked() = false while the reader is still parked")
	}
	close(collected)
	if shellDumpParked() {
		t.Fatal("shellDumpParked() = true after the reader was collected")
	}
	if shellDumpParked() {
		t.Fatal("shellDumpParked() = true on a second look, want the reader forgotten")
	}
}

func spawnCount(t *testing.T, path string) int {
	t.Helper()
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read spawn log: %v", err)
	}
	return strings.Count(string(body), "x")
}
