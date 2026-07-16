//go:build windows

package terminal

import (
	"context"
	"fmt"
	"os/exec"

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
	return &windowsPTY{cpty}, nil
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
// Read, Write, Close (which terminates the child) and a same-shape Resize;
// only Wait needs its context-and-exit-code signature narrowed.
type windowsPTY struct {
	*conpty.ConPty
}

func (p *windowsPTY) Wait() error {
	_, err := p.ConPty.Wait(context.Background())
	return err
}
