//go:build linux || darwin

package terminal

import (
	"io"
	"os/exec"
	"testing"
	"time"

	"github.com/creack/pty"
)

// TestProcessCwdFollowsForegroundJob proves the readout moves with the nested
// shell the user started rather than with the PTY child, which never left the
// directory it was spawned in. That is the one case the direct-child read
// cannot answer and the only reason this seam reads a foreground group at all.
//
// Driven through a real PTY and the shell's own job control: tpgid on Linux
// and e_tpgid on macOS are set by a shell handing the terminal to a job, and
// nothing about that is reachable without one.
func TestProcessCwdFollowsForegroundJob(t *testing.T) {
	outer := physical(t, t.TempDir())
	inner := physical(t, t.TempDir())

	cmd := exec.Command("/bin/sh", "-i")
	cmd.Dir = outer
	// A stranger's rc file must not get a say in whether this passes: sh reads
	// whatever $ENV points at.
	cmd.Env = []string{"PATH=/bin:/usr/bin", "TERM=dumb", "ENV=", "PS1=$ "}
	ptmx, err := pty.Start(cmd)
	if err != nil {
		t.Fatalf("pty.Start: %v", err)
	}
	t.Cleanup(func() {
		_ = ptmx.Close()
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
	})
	// A shell whose output has nowhere to go blocks once the PTY buffer fills,
	// and a blocked shell never reaches the cd being measured.
	go func() { _, _ = io.Copy(io.Discard, ptmx) }()

	// Typed rather than spawned directly: only a shell's job control puts the
	// nested shell in its own process group and gives it the terminal. Bytes
	// typed before the shell is ready wait in the line discipline, so there is
	// no readiness handshake to get wrong.
	if _, err := ptmx.WriteString("/bin/sh -i\ncd " + inner + "\n"); err != nil {
		t.Fatalf("write: %v", err)
	}

	pid := cmd.Process.Pid
	deadline := time.Now().Add(20 * time.Second)
	for {
		got := processCwd(pid)
		if got == inner {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("processCwd(%d) = %q, want the nested shell's %q", pid, got, inner)
		}
		time.Sleep(20 * time.Millisecond)
	}
	// Pins what the pass is worth: the child itself never moved, so this cannot
	// be a run where both reads happen to agree on the same directory.
	if got := readCwd(pid); got != outer {
		t.Fatalf("readCwd(%d) = %q, want the unmoved child's %q", pid, got, outer)
	}
}
