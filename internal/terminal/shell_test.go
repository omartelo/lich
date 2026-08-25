package terminal

import (
	"strings"
	"testing"
)

// TestWindowsShellsPreferPowerShell7 pins the order a Windows session resolves
// its shell in, and the absence that is the whole point of the list: cmd.exe is
// not a shell lich opens a session in, so a regression that puts it back — or
// that reads COMSPEC, which names it on every box — fails here rather than on a
// user's machine.
func TestWindowsShellsPreferPowerShell7(t *testing.T) {
	if len(windowsShells) != 2 || windowsShells[0] != "pwsh.exe" || windowsShells[1] != "powershell.exe" {
		t.Errorf("windowsShells = %v, want pwsh.exe before powershell.exe", windowsShells)
	}
	for _, shell := range windowsShells {
		if strings.Contains(strings.ToLower(shell), "cmd") {
			t.Errorf("cmd.exe is back in the shell list: %v", windowsShells)
		}
	}
}
