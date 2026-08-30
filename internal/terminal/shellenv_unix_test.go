//go:build !windows

package terminal

import (
	"context"
	"os"
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
	_, _ = runShellDump(ctx, "/bin/sh", "sleep 30", os.Environ())
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
	_, _ = runShellDump(ctx, "/bin/sh", "(sleep 30 &); exit 0", os.Environ())
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
	out, err := runShellDump(ctx, "/bin/sh", "echo REAL_OUTPUT=yes; (sleep 30 &); exit 0", os.Environ())
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
