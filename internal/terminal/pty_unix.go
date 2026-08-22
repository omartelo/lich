//go:build !windows

package terminal

import (
	"os"
	"os/exec"
	"syscall"
	"time"

	"github.com/creack/pty"
)

// closeGrace is how long a hang-up waits for the signalled child to leave on
// its own before killing it. An agent killed outright never runs its own exit
// path: Claude Code, for one, treats a session that dies within ten seconds of
// its first frame as a fullscreen renderer that failed to start, and two of
// those turn its fullscreen renderer off for every session on the machine.
const closeGrace = time.Second

// startPTY starts spec's child attached to a fresh PTY sized cols x rows.
func startPTY(spec ptySpec) (ptyHandle, error) {
	cmd := exec.Command(spec.bin, spec.args...)
	cmd.Dir = spec.dir
	cmd.Env = spec.env
	ptmx, err := pty.StartWithSize(cmd, winsize(spec.cols, spec.rows))
	if err != nil {
		return nil, err
	}
	p := &unixPTY{File: ptmx, cmd: cmd, exited: make(chan struct{})}
	go p.reap()
	return p, nil
}

// unixPTY pairs the PTY master file creack/pty returns (which carries
// Read/Write) with the child it drives: a goroutine reaps that child the
// moment it exits, so Close can tell a child that left from one that has to be
// made to.
type unixPTY struct {
	*os.File
	cmd *exec.Cmd

	// exited is closed once the child is reaped; code and err are written
	// before that close and read only after it.
	exited chan struct{}
	code   int
	err    error
}

func (p *unixPTY) Resize(cols, rows int) error {
	return pty.Setsize(p.File, winsize(cols, rows))
}

func (p *unixPTY) Pid() int { return p.cmd.Process.Pid }

// reap waits for the child and records its exit status. ProcessState carries it
// even when Wait errors, because a non-zero exit is itself an *exec.ExitError;
// os.ProcessState.ExitCode already answers noExitStatus for a child killed by a
// signal.
func (p *unixPTY) reap() {
	p.err = p.cmd.Wait()
	p.code = noExitStatus
	if p.cmd.ProcessState != nil {
		p.code = p.cmd.ProcessState.ExitCode()
	}
	close(p.exited)
}

// Wait reports the child's exit status once it has been reaped.
func (p *unixPTY) Wait() (int, error) {
	<-p.exited
	return p.code, p.err
}

// Close hangs up: the child is asked to leave with SIGTERM and killed only if
// it outstays closeGrace. The master is closed last so the departing child's
// final bytes — the escape sequences that put the terminal back the way it was
// — still reach the reader.
func (p *unixPTY) Close() error {
	if p.cmd.Process != nil && p.cmd.Process.Signal(syscall.SIGTERM) == nil {
		select {
		case <-p.exited:
		case <-time.After(closeGrace):
			_ = p.cmd.Process.Kill()
		}
	}
	return p.File.Close()
}

func winsize(cols, rows int) *pty.Winsize {
	return &pty.Winsize{Rows: uint16(rows), Cols: uint16(cols)}
}
