//go:build windows

package terminal

import "os/exec"

// defaultShell is spawned for "shell" sessions when nothing else resolves. It
// is the shell every Windows install carries; a box without it fails the spawn
// with the path it looked for, which is the honest answer.
const defaultShell = "powershell.exe"

// userShell returns the best shell on $PATH, "" when none of them resolves.
// Windows has no $SHELL, and the variable that comes closest is not one — see
// windowsShells. The resolved path is returned rather than the name: it is what
// startPTY has to look up anyway, and asking $PATH once keeps the answer stable
// for the life of the session.
func userShell() string {
	for _, shell := range windowsShells {
		if path, err := exec.LookPath(shell); err == nil {
			return path
		}
	}
	return ""
}
