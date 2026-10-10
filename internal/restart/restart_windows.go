//go:build windows

package restart

import (
	"os/exec"
	"syscall"

	"golang.org/x/sys/windows"
)

// startDetached launches exe detached from this console and process group so it
// outlives the restarting process, including an installer that must wait for
// this process to release its executable.
func startDetached(exe string, env, args []string) error {
	cmd := exec.Command(exe, args...)
	cmd.Env = env
	cmd.SysProcAttr = &syscall.SysProcAttr{
		CreationFlags: windows.DETACHED_PROCESS | windows.CREATE_NEW_PROCESS_GROUP,
	}
	return cmd.Start()
}
