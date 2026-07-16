//go:build !windows

package winexec

import (
	"os/exec"
	"testing"
)

// Off Windows there is no console window to hide, so Hide must leave the
// command untouched — anything else would be a portability surprise.
func TestHideIsNoOp(t *testing.T) {
	cmd := exec.Command("git", "--version")
	Hide(cmd)
	if cmd.SysProcAttr != nil {
		t.Fatalf("Hide set SysProcAttr off Windows: %#v", cmd.SysProcAttr)
	}
}
