//go:build windows

package winexec

import (
	"os/exec"
	"testing"

	"golang.org/x/sys/windows"
)

// The whole point on Windows: Hide must set CREATE_NO_WINDOW so lich's
// windowless GUI binary does not spawn a console window per child.
func TestHideSetsCreateNoWindow(t *testing.T) {
	cmd := exec.Command("git", "--version")
	Hide(cmd)
	if cmd.SysProcAttr == nil {
		t.Fatal("Hide left SysProcAttr nil on Windows")
	}
	if cmd.SysProcAttr.CreationFlags&windows.CREATE_NO_WINDOW == 0 {
		t.Fatalf("CREATE_NO_WINDOW not set: flags=%#x", cmd.SysProcAttr.CreationFlags)
	}
	if !cmd.SysProcAttr.HideWindow {
		t.Fatal("HideWindow not set")
	}
}
