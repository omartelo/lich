//go:build !windows

package terminal

import (
	"bytes"
	"context"
	"os/exec"
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
func runShellDump(ctx context.Context, shell, cmdStr string, env []string) (string, error) {
	cmd := exec.Command(shell, "-l", "-i", "-c", cmdStr)
	cmd.Env = env
	ptmx, err := pty.Start(cmd)
	if err != nil {
		return "", err
	}

	chunks := make(chan []byte)
	go func() {
		defer cmd.Wait()
		defer ptmx.Close()
		buf := make([]byte, 4096)
		for {
			n, err := ptmx.Read(buf)
			if n > 0 {
				cp := make([]byte, n)
				copy(cp, buf[:n])
				chunks <- cp
			}
			if err != nil {
				close(chunks)
				return
			}
		}
	}()

	var out bytes.Buffer
	var quiet <-chan time.Time
	for {
		select {
		case b, ok := <-chunks:
			if !ok {
				return out.String(), nil
			}
			out.Write(b)
			quiet = time.After(shellDumpQuiet)
		case <-quiet:
			return out.String(), nil
		case <-ctx.Done():
			_ = cmd.Process.Kill()
			return out.String(), ctx.Err()
		}
	}
}
