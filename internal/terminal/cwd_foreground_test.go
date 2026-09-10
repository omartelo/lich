//go:build linux || darwin

package terminal

import (
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
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
		got, host := processCwd(pid)
		if got == inner && host == "" {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("processCwd(%d) = (%q, %q), want the nested shell's %q", pid, got, host, inner)
		}
		time.Sleep(20 * time.Millisecond)
	}
	// Pins what the pass is worth: the child itself never moved, so this cannot
	// be a run where both reads happen to agree on the same directory.
	if got := readCwd(pid); got != outer {
		t.Fatalf("readCwd(%d) = %q, want the unmoved child's %q", pid, got, outer)
	}
}

// TestReadCommNamesTheProcess proves the comm read this seam matches against
// resolves a live process — exercised against the test binary itself, whose
// name both platforms truncate (15 bytes on Linux, 16 on macOS) rather than
// refuse. And that an ordinary command is not mistaken for a shell host, which
// is the half of shellHost no list of names can assert on its own.
func TestReadCommNamesTheProcess(t *testing.T) {
	comm := readComm(os.Getpid())
	if comm == "" {
		t.Fatal(`readComm(self) = "", want the test binary's name`)
	}
	if base := filepath.Base(os.Args[0]); !strings.HasPrefix(base, comm) {
		t.Errorf("readComm(self) = %q, want a prefix of %q", comm, base)
	}
	if host := shellHost(comm); host != "" {
		t.Errorf("shellHost(%q) = %q, want no host for an ordinary process", comm, host)
	}
}

// TestReadCommOfDeadPidIsEmpty proves an unresolvable process yields no name,
// so processCwd falls through to the directory read rather than matching the
// empty string against the host list.
func TestReadCommOfDeadPidIsEmpty(t *testing.T) {
	// Far beyond any real PID space (Linux pid_max < 2^22, macOS ~1e5).
	if got := readComm(1 << 30); got != "" {
		t.Errorf("readComm(dead) = %q, want empty", got)
	}
}

// TestProcessCwdNamesAShellHostInTheForeground is the other half of the read
// the foreground group exists for: when the job the user started is one of the
// wrappers whose shell lives somewhere unreadable (shellHosts), the answer is
// that name and no path. Nothing else proves the wiring reads the foreground
// process rather than the PTY child — the two agree on every ordinary job, and
// the child is never named tmux.
//
// A copy of /bin/sh under the name is what makes it testable: comm answers what
// was executed, so the copy reads as "tmux" to the very seam a real tmux would,
// without a multiplexer, a socket or a server process in the test.
func TestProcessCwdNamesAShellHostInTheForeground(t *testing.T) {
	fake := filepath.Join(t.TempDir(), "tmux")
	sh, err := os.ReadFile("/bin/sh")
	if err != nil {
		t.Skipf("no /bin/sh to copy: %v", err)
	}
	if err := os.WriteFile(fake, sh, 0o755); err != nil {
		t.Fatalf("write %s: %v", fake, err)
	}

	cmd := exec.Command("/bin/sh", "-i")
	cmd.Dir = physical(t, t.TempDir())
	// See TestProcessCwdFollowsForegroundJob: a stranger's $ENV must not decide
	// whether this passes.
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
	go func() { _, _ = io.Copy(io.Discard, ptmx) }()

	// Typed, so the outer shell's job control hands the terminal to the copy.
	if _, err := ptmx.WriteString(fake + " -i\n"); err != nil {
		t.Fatalf("write: %v", err)
	}

	pid := cmd.Process.Pid
	deadline := time.Now().Add(20 * time.Second)
	for {
		got, host := processCwd(pid)
		if host == "tmux" && got == "" {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("processCwd(%d) = (%q, %q), want no path and the host \"tmux\"", pid, got, host)
		}
		time.Sleep(20 * time.Millisecond)
	}
}
