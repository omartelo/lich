//go:build !windows

package restart

import (
	"os"
	"os/exec"
	"syscall"
)

// startDetached launches exe in its own session (setsid) so it outlives this
// process and the PTY that triggered the restart. stdio is left nil — the
// successor logs to its own file (main's logging.Init), like any lich launch.
func startDetached(exe string, env []string) error {
	cmd := exec.Command(exe)
	cmd.Env = env
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	return cmd.Start()
}

// terminateProcess asks the window to close gracefully (SIGTERM) so Chromium
// clears its profile lock before exiting — a clean exit unwinds main's defers.
func terminateProcess(p *os.Process) error {
	return p.Signal(syscall.SIGTERM)
}
