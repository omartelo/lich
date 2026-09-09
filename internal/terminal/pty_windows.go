//go:build windows

package terminal

import (
	"context"
	"fmt"
	"log/slog"
	"os/exec"
	"sync"

	"github.com/UserExistsError/conpty"
	"golang.org/x/sys/windows"
)

// setConsoleCtrlHandler is not in x/sys/windows; the call below is the only
// use lich has for it.
var setConsoleCtrlHandler = windows.NewLazySystemDLL("kernel32.dll").NewProc("SetConsoleCtrlHandler")

// heedCtrlC clears the "ignore Ctrl+C" attribute this process may carry, once.
// A process started by a service — a scheduler, a CI runner — inherits that
// attribute and hands it to every process it starts, and a child that ignores
// Ctrl+C never sees the one a close sends: the close waits out closeGrace and
// kills it anyway. A child inherits the attribute as it stands when it is
// created, so clearing it before the first spawn covers every session after.
// It is the process default otherwise, so this changes nothing for a lich
// started from a desktop.
var heedCtrlC = sync.OnceFunc(func() {
	if ok, _, err := setConsoleCtrlHandler.Call(0, 0); ok == 0 {
		slog.Warn("clear the inherited Ctrl+C ignore", "error", err)
	}
})

// startPTY starts spec's child attached to a fresh ConPTY sized cols x rows.
func startPTY(spec ptySpec) (ptyHandle, error) {
	heedCtrlC()
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
// the read loop, freed by the same close, reaps it again (see stream). The Unix
// seam guards its own repeat Close for a milder reason — os.File.Close only
// reports os.ErrClosed the second time; here the second call hands Windows
// handles it has already reissued.
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

// Close sends Ctrl+C and kills the child only if it outstays closeGrace, once.
// Cancel the pending Wait before taking its lock: conpty polls the process
// handle on a one-second tick, so acquiring the lock can take another second.
func (p *windowsPTY) Close() error {
	p.cancelWait()
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.closed {
		return nil
	}
	p.closed = true
	// Send the terminal's Ctrl+C key through ConPTY so the child's console
	// handles it without attaching the backend to another session's console.
	if _, err := p.Write([]byte{ctrlC}); err == nil {
		ctx, cancel := context.WithTimeout(context.Background(), closeGrace)
		defer cancel()
		if _, err := p.ConPty.Wait(ctx); err != nil && ctx.Err() == nil {
			slog.Warn("wait for Ctrl+C exit", "pid", p.Pid(), "error", err)
		}
	}
	return p.ConPty.Close()
}
