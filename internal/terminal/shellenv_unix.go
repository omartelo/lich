//go:build !windows

package terminal

import (
	"context"
	"errors"
	"io"
	"os"
	"os/exec"
	"strings"
	"syscall"

	"github.com/creack/pty"
	"golang.org/x/sys/unix"
)

// runShellDump runs cmdStr under shell -l -i, attached to a pty rather than a
// pipe: an rc file guarded on `[ -t 0 ]`/`tty -s` — the common guard around a
// version manager's init (nvm, fnm) — only loads when stdin looks like a
// terminal.
//
// The read cannot be bounded by closing the pty or by SetReadDeadline: both
// were measured against a read genuinely blocked in the read syscall and
// neither interrupts it — a Close from another goroutine returns without
// error while the read stays parked in the kernel regardless (Linux does not
// interrupt an in-flight blocking read by closing the fd from elsewhere),
// and SetReadDeadline never reaches a syscall that never goes non-blocking
// here. So the read is driven from a goroutine that is allowed to outlive
// this call, and its output arrives here over a channel: once end has been
// printed and the shell has exited, the shell exits early, or ctx expires,
// whichever comes first, this returns whatever has accumulated so far. The goroutine is left running for
// whatever still holds the pty (a background job its rc left wired to the
// terminal: an agent daemon, a prompt tool), and a single leaked goroutine
// (and its fd, and its zombie child once it exits) is the cost of never
// blocking boot past ctx.
//
// Completion is end appearing, never a silence: a shell can go quiet for long
// stretches mid-dump: on macOS the PATH search for `env` blocks on a
// privacy-protected folder (~/Documents) until the user answers the prompt.
func runShellDump(ctx context.Context, shell, cmdStr, end string, env []string) (string, <-chan struct{}, error) {
	cmd := exec.Command(shell, "-l", "-i", "-c", cmdStr)
	cmd.Env = env
	ptmx, err := pty.Start(cmd)
	if err != nil {
		return "", nil, err
	}

	// Waited apart from the reader so the end of the dump can ask whether the
	// shell is gone without waiting on a read that may never return.
	exited := make(chan struct{})
	var waitErr error
	go func() {
		waitErr = cmd.Wait()
		close(exited)
	}()

	results := make(chan shellDumpResult)
	// abandoned frees the reader from a send nobody is receiving any more, so
	// that the moment its parked read finally returns it can leave; collected
	// closes when it does, and that is the only signal there is that the pty was
	// let go (ReresolveShellEnv holds it to keep the leak at one outstanding).
	abandoned := make(chan struct{})
	collected := make(chan struct{})
	go readShellDump(ptmx, results, abandoned, collected, exited, &waitErr)

	var out strings.Builder
	sawEnd := false
	// Nil until the end marker arrives. Handing the reader back as parked the
	// moment it does refuses the very next resolution for the few milliseconds
	// the shell takes to exit, so the end instead waits for the shell to go.
	var shellGone <-chan struct{}
	for {
		select {
		case result := <-results:
			if result.chunk == nil {
				if sawEnd {
					return out.String(), nil, nil
				}
				return out.String(), nil, result.err
			}
			out.Write(result.chunk)
			if !sawEnd && strings.Contains(out.String(), end) {
				sawEnd, shellGone = true, exited
			}
		case <-shellGone:
			// The shell closed its side of the pty on its way out, so with no
			// other holder the read has already been answered and its final
			// result is on the way; a job still holding the pty parks it.
			if ptyHungUp(ptmx) {
				shellGone = nil
				continue
			}
			close(abandoned)
			return out.String(), collected, nil
		case <-ctx.Done():
			close(abandoned)
			if sawEnd {
				return out.String(), collected, nil
			}
			_ = cmd.Process.Kill()
			return out.String(), collected, ctx.Err()
		}
	}
}

type shellDumpResult struct {
	chunk []byte
	err   error
}

func readShellDump(ptmx *os.File, results chan<- shellDumpResult, abandoned <-chan struct{}, collected chan<- struct{}, exited <-chan struct{}, waitErr *error) {
	defer close(collected)
	defer ptmx.Close()
	buf := make([]byte, 4096)
	for {
		n, readErr := ptmx.Read(buf)
		if n > 0 {
			cp := make([]byte, n)
			copy(cp, buf[:n])
			select {
			case results <- shellDumpResult{chunk: cp}:
			case <-abandoned:
				return
			}
		}
		if readErr != nil {
			<-exited
			if *waitErr != nil {
				readErr = *waitErr
			} else if errors.Is(readErr, io.EOF) || errors.Is(readErr, syscall.EIO) {
				readErr = nil
			}
			select {
			case results <- shellDumpResult{err: readErr}:
			case <-abandoned:
			}
			return
		}
	}
}

// ptyHungUp reports that no process holds the pty's slave side any more, which
// is what guarantees a read on the master returns rather than parks. A poll
// that fails answers no: a reader wrongly called parked costs one refused
// re-check, one wrongly called free would be waited on forever.
// Control rather than Fd: Fd flips the file to blocking under the reader.
func ptyHungUp(ptmx *os.File) bool {
	raw, err := ptmx.SyscallConn()
	if err != nil {
		return false
	}
	hungUp := false
	ctlErr := raw.Control(func(fd uintptr) {
		fds := []unix.PollFd{{Fd: int32(fd), Events: unix.POLLIN}}
		n, pollErr := unix.Poll(fds, 0)
		hungUp = pollErr == nil && n > 0 && fds[0].Revents&unix.POLLHUP != 0
	})
	return ctlErr == nil && hungUp
}
