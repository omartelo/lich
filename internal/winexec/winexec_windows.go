//go:build windows

// Package winexec suppresses the console window Windows would otherwise
// allocate for every console tool lich shells out to (git, gh, claude). lich's
// Windows binary is built for the GUI subsystem (-H=windowsgui) and so has no
// console of its own; without CREATE_NO_WINDOW, each such child pops a fresh
// console window on the user's desktop — and the git-status poll spawning one
// every few seconds per session buries the screen. Off Windows this is a no-op.
package winexec

import (
	"os/exec"
	"syscall"

	"golang.org/x/sys/windows"
)

// Hide sets the creation flag that keeps cmd from allocating a console window.
func Hide(cmd *exec.Cmd) {
	if cmd.SysProcAttr == nil {
		cmd.SysProcAttr = &syscall.SysProcAttr{}
	}
	cmd.SysProcAttr.HideWindow = true
	cmd.SysProcAttr.CreationFlags |= windows.CREATE_NO_WINDOW
}
