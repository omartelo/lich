//go:build !windows

package terminal

import (
	"os"
	"os/exec"

	"github.com/creack/pty"
)

// startPTY starts cmd attached to a fresh PTY sized cols x rows.
func startPTY(cmd *exec.Cmd, cols, rows int) (ptyHandle, error) {
	ptmx, err := pty.StartWithSize(cmd, winsize(cols, rows))
	if err != nil {
		return nil, err
	}
	return &unixPTY{ptmx}, nil
}

// unixPTY adds Resize to the PTY master file creack/pty returns, which
// already carries Read/Write/Close.
type unixPTY struct {
	*os.File
}

func (p *unixPTY) Resize(cols, rows int) error {
	return pty.Setsize(p.File, winsize(cols, rows))
}

func winsize(cols, rows int) *pty.Winsize {
	return &pty.Winsize{Rows: uint16(rows), Cols: uint16(cols)}
}
