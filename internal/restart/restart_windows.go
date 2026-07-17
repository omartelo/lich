//go:build windows

package restart

import (
	"os"
	"os/exec"
	"syscall"

	"golang.org/x/sys/windows"
)

// startDetached launches exe detached from this console and process group so it
// outlives the restarting process. Windows installs self-apply through
// internal/appupdate rather than this path, so this exists mainly to compile.
func startDetached(exe string, env []string) error {
	cmd := exec.Command(exe)
	cmd.Env = env
	cmd.SysProcAttr = &syscall.SysProcAttr{
		CreationFlags: windows.DETACHED_PROCESS | windows.CREATE_NEW_PROCESS_GROUP,
	}
	return cmd.Start()
}

// terminateProcess ends the window process. Windows has no graceful signal to a
// GUI child here, so this is a hard kill.
func terminateProcess(p *os.Process) error {
	return p.Kill()
}
