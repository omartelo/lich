//go:build !windows

package terminal

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os/exec"
	"syscall"
	"time"

	"github.com/creack/pty"
)

// shellDumpQuiet bounds how long the read waits after the last byte before
// treating the dump as complete. It only matters once real output has
// started — see the nil quiet channel below — so a shell that is merely slow
// to speak (nvm/fnm's own init, among the common causes) is bounded solely
// by ctx, same as before this file existed. Once the shell has spoken and
// gone silent this long, whatever still holds the pty open is a background
// job its rc left running (an agent daemon, a prompt tool) with its stdio
// still wired to the terminal — not the shell itself still working.
const shellDumpQuiet = 300 * time.Millisecond

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
// this call, and its output arrives here over a channel: the moment nothing
// new arrives for shellDumpQuiet, or ctx expires, whichever comes first,
// this returns whatever has accumulated so far. The goroutine is left
// running for whatever still holds the pty — a single leaked goroutine (and
// its fd, and its zombie child once it exits) is the cost of never blocking
// boot past ctx.
func runShellDump(ctx context.Context, shell, cmdStr string, env []string) (string, <-chan struct{}, error) {
	cmd := exec.Command(shell, "-l", "-i", "-c", cmdStr)
	cmd.Env = env
	ptmx, err := pty.Start(cmd)
	if err != nil {
		return "", nil, err
	}

	type result struct {
		chunk []byte
		err   error
	}
	results := make(chan result)
	// abandoned frees the reader from a send nobody is receiving any more, so
	// that the moment its parked read finally returns it can leave; collected
	// closes when it does, and that is the only signal there is that the pty was
	// let go (ReresolveShellEnv holds it to keep the leak at one outstanding).
	abandoned := make(chan struct{})
	collected := make(chan struct{})
	go func() {
		defer close(collected)
		defer ptmx.Close()
		buf := make([]byte, 4096)
		for {
			n, readErr := ptmx.Read(buf)
			if n > 0 {
				cp := make([]byte, n)
				copy(cp, buf[:n])
				select {
				case results <- result{chunk: cp}:
				case <-abandoned:
					return
				}
			}
			if readErr != nil {
				waitErr := cmd.Wait()
				if waitErr != nil {
					readErr = waitErr
				} else if errors.Is(readErr, io.EOF) || errors.Is(readErr, syscall.EIO) {
					readErr = nil
				}
				select {
				case results <- result{err: readErr}:
				case <-abandoned:
				}
				return
			}
		}
	}()

	var out bytes.Buffer
	var quiet <-chan time.Time
	for {
		select {
		case result := <-results:
			if result.err != nil || result.chunk == nil {
				// The reader sent its last result and is on its way out, so
				// there is nothing left parked for the next resolution to wait
				// on.
				return out.String(), nil, result.err
			}
			out.Write(result.chunk)
			quiet = time.After(shellDumpQuiet)
		case <-quiet:
			close(abandoned)
			return out.String(), collected, nil
		case <-ctx.Done():
			close(abandoned)
			_ = cmd.Process.Kill()
			return out.String(), collected, ctx.Err()
		}
	}
}
