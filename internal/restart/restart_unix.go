//go:build !windows

package restart

import (
	"os/exec"
	"syscall"
)

// StartDetached launches exe in its own session (setsid) so it outlives this
// process and the PTY that triggered the restart. stdio is left nil — the
// successor logs to its own file (main's logging.Init), like any lich launch.
func StartDetached(exe string, env, args []string) error {
	cmd := exec.Command(exe, args...)
	cmd.Env = env
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	return cmd.Start()
}
