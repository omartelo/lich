//go:build linux || darwin

package terminal

import (
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
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
// A process named tmux is what makes it testable, and this binary hard linked
// under that name is one: it reads as a host to the very seam a real tmux would,
// without a multiplexer, a socket or a server anywhere in the test.
func TestProcessCwdNamesAShellHostInTheForeground(t *testing.T) {
	// A hard link to this test binary, which is the one executable that is both
	// nameable and runnable on both OSes. The two other shapes are dead ends,
	// and each was measured against the macOS runner:
	//
	//   - A copy of /bin/sh is killed at exec there. It is a platform binary
	//     whose signature is anchored to the system trust cache, and a copy of
	//     it outside that cache fails validation.
	//   - A symlink to /bin/sh runs, but answers "sh". Linux takes comm from
	//     the path handed to execve, so the link reads as "tmux"; macOS takes
	//     p_comm from the binary the lookup resolved to, so it does not.
	//
	// A hard link has no target to resolve: the directory entry is the name,
	// and the inode is a binary that already runs here. /bin/sh cannot be hard
	// linked on macOS at all — it lives on the sealed system volume and the
	// temp dir does not.
	fake := filepath.Join(t.TempDir(), "tmux")
	if err := os.Link(os.Args[0], fake); err != nil {
		t.Skipf("cannot link the test binary: %v", err)
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
	// Kept rather than discarded: when the linked shell does not take the
	// terminal, the reason is a line the shell printed and nothing else says
	// it. The read runs on its own goroutine, so the transcript is guarded.
	var mu sync.Mutex
	var transcript strings.Builder
	go func() {
		buf := make([]byte, 512)
		for {
			n, err := ptmx.Read(buf)
			mu.Lock()
			transcript.Write(buf[:n])
			mu.Unlock()
			if err != nil {
				return
			}
		}
	}()

	// Typed, so the outer shell's job control hands the terminal to the link.
	// The assignment rides in front of the command rather than in cmd.Env: the
	// outer shell must not match the stub's guard itself.
	if _, err := ptmx.WriteString(
		hostStubVar + "=1 " + fake + " -test.run=" + hostStubTest + "\n",
	); err != nil {
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
			mu.Lock()
			said := transcript.String()
			mu.Unlock()
			t.Fatalf("processCwd(%d) = (%q, %q), want no path and the host %q; the shell said:\n%s",
				pid, got, host, "tmux", said)
		}
		time.Sleep(20 * time.Millisecond)
	}
}

// hostStubVar and hostStubTest name the stand-in the test above runs: this same
// binary, hard linked under a shell host's name, re-entered to sit in the
// foreground and do nothing. Spelled once, because the link is executed by a
// string typed at a shell and no compiler checks that string.
const (
	hostStubVar  = "LICH_SHELL_HOST_STUB"
	hostStubTest = "TestStandsInForAShellHost"
)

// TestStandsInForAShellHost is not a test of anything. It is the body the hard
// link runs so that a live process named tmux can hold the terminal while
// processCwd reads it, which is the one thing a real multiplexer would provide
// and nothing else in this package can fake.
//
// It answers to the guard alone, so an ordinary run of this package skips it.
// What normally ends it is the caller closing the PTY master, which hangs the
// terminal up and signals this process along with it. The sleep is only the
// bound for the run where that does not arrive: three times the caller's own
// deadline, so it can never be what a passing run is waiting on.
func TestStandsInForAShellHost(t *testing.T) {
	if os.Getenv(hostStubVar) == "" {
		t.Skip("runs only as the hard-linked stand-in")
	}
	time.Sleep(time.Minute)
}
