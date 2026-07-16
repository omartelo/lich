//go:build windows

package terminal

import (
	"errors"
	"os/exec"
)

// startPTY is the Windows side of the PTY seam. ConPTY is the platform
// primitive it must wrap; until that lands, the package compiles on Windows
// and every session start fails with this error instead.
func startPTY(_ *exec.Cmd, _, _ int) (ptyHandle, error) {
	return nil, errors.New("terminal sessions need ConPTY support, not implemented yet")
}
