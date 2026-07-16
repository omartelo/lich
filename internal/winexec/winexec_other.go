//go:build !windows

package winexec

import "os/exec"

// Hide is a no-op off Windows, where a console child has no window to hide.
func Hide(cmd *exec.Cmd) {}
