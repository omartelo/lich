//go:build !windows

package terminal

import (
	"os"
	"os/exec"

	"github.com/creack/pty"
)

// startPTY starts spec's child attached to a fresh PTY sized cols x rows.
func startPTY(spec ptySpec) (ptyHandle, error) {
	cmd := exec.Command(spec.bin, spec.args...)
	cmd.Dir = spec.dir
	cmd.Env = spec.env
	ptmx, err := pty.StartWithSize(cmd, winsize(spec.cols, spec.rows))
	if err != nil {
		return nil, err
	}
	return &unixPTY{File: ptmx, cmd: cmd}, nil
}

// unixPTY pairs the PTY master file creack/pty returns (which carries
// Read/Write) with the child it drives: Wait reaps it after the master hits
// EOF, and Close also kills it so hanging up ends the session.
type unixPTY struct {
	*os.File
	cmd *exec.Cmd
}

func (p *unixPTY) Resize(cols, rows int) error {
	return pty.Setsize(p.File, winsize(cols, rows))
}

func (p *unixPTY) Pid() int { return p.cmd.Process.Pid }

// Wait reaps the child and reports its exit status. ProcessState carries it
// even when Wait errors, because a non-zero exit is itself an *exec.ExitError;
// os.ProcessState.ExitCode already answers noExitStatus for a child killed by a
// signal.
func (p *unixPTY) Wait() (int, error) {
	err := p.cmd.Wait()
	if p.cmd.ProcessState == nil {
		return noExitStatus, err
	}
	return p.cmd.ProcessState.ExitCode(), err
}

func (p *unixPTY) Close() error {
	err := p.File.Close()
	if p.cmd.Process != nil {
		_ = p.cmd.Process.Kill()
	}
	return err
}

func winsize(cols, rows int) *pty.Winsize {
	return &pty.Winsize{Rows: uint16(rows), Cols: uint16(cols)}
}
