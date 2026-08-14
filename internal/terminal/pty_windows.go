//go:build windows

package terminal

import (
	"context"
	"fmt"
	"os/exec"
	"sync"

	"github.com/UserExistsError/conpty"
	"golang.org/x/sys/windows"
)

// startPTY starts spec's child attached to a fresh ConPTY sized cols x rows.
func startPTY(spec ptySpec) (ptyHandle, error) {
	line, err := commandLine(spec.bin, spec.args)
	if err != nil {
		return nil, err
	}
	cpty, err := conpty.Start(
		line,
		conpty.ConPtyDimensions(spec.cols, spec.rows),
		conpty.ConPtyWorkDir(spec.dir),
		conpty.ConPtyEnv(spec.env),
	)
	if err != nil {
		return nil, err
	}
	waitCtx, cancelWait := context.WithCancel(context.Background())
	return &windowsPTY{ConPty: cpty, waitCtx: waitCtx, cancelWait: cancelWait}, nil
}

// commandLine turns bin+args into the single quoted command line
// CreateProcess expects. The binary goes through exec.LookPath because
// CreateProcess resolves neither PATHEXT nor script wrappers — and npm ships
// Claude Code as claude.cmd, which only cmd.exe can run.
func commandLine(bin string, args []string) (string, error) {
	path, err := exec.LookPath(bin)
	if err != nil {
		return "", fmt.Errorf("resolve %q: %w", bin, err)
	}
	return windows.ComposeCommandLine(wrapArgv(path, args)), nil
}

// windowsPTY adapts a ConPTY to the seam. The embedded type already carries
// Read, Write and a same-shape Resize; Wait and Close are wrapped because
// conpty's cannot be called twice or overlap each other.
//
// conpty's Close is not idempotent: it calls ClosePseudoConsole on the HPCON
// and CloseHandle on six handles unconditionally, and its Wait reads the
// process handle among them. A second Close therefore hands CloseHandle values
// Windows has already reissued to whatever this process opened next — the
// loopback listener's socket, the log file, another session's pipes — and the
// app stops answering with nothing panicking and nothing logged. Every
// user-driven close reaches that second call: Service.Close closes the PTY and
// the read loop, freed by the same close, reaps it again (see stream). The
// Unix seam is *os.File, whose repeat Close is the harmless no-op stream's
// comment describes; this one has to be made into it.
type windowsPTY struct {
	*conpty.ConPty
	waitCtx    context.Context
	cancelWait context.CancelFunc

	mu     sync.Mutex
	closed bool
}

// Wait reaps the child and reports its exit status, or returns immediately once
// Close has taken the handles it would read — there is no status to read then,
// and none after a wait that failed either.
func (p *windowsPTY) Wait() (int, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.closed {
		return noExitStatus, nil
	}
	code, err := p.ConPty.Wait(p.waitCtx)
	if err != nil {
		return noExitStatus, err
	}
	return int(code), nil
}

// Close hangs up and terminates the child, once. The pending Wait is cancelled
// before the lock is taken because it only returns when the child exits, and
// the child exits because of this very call — conpty polls the process handle
// on a one-second tick, so that is the longest a close waits for it.
func (p *windowsPTY) Close() error {
	p.cancelWait()
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.closed {
		return nil
	}
	p.closed = true
	return p.ConPty.Close()
}
