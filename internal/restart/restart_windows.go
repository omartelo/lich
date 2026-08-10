//go:build windows

package restart

import (
	"log/slog"
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

// terminateProcess asks the window to close, the way restart_unix.go's SIGTERM
// does, so Chromium clears its profile lock and flushes it. That flush is the
// point: the page's prefs live in the profile's localStorage, which Chromium
// commits to disk lazily, so a window that is killed rather than closed loses
// whatever was written in the seconds before the restart — a dismissed dialog
// coming back on the next launch (#199) is what that looks like.
//
// Windows has no signal to send, so the close is posted as WM_CLOSE by taskkill
// without /F. The hard kill stays as the fallback for when there is no window to
// post to (a restart in the moment before it opens): the successor is already
// waiting on the pinned port, and nothing may be left holding it.
func terminateProcess(p *os.Process) error {
	exe, args := closeWindowCommand(os.Getenv, p.Pid)
	cmd := exec.Command(exe, args...)
	// lich is built for the GUI subsystem (-H=windowsgui); without this the
	// console helper flashes a window over the app on its way out.
	cmd.SysProcAttr = &syscall.SysProcAttr{CreationFlags: windows.CREATE_NO_WINDOW}
	if err := cmd.Run(); err != nil {
		slog.Warn("close window gracefully, killing instead", "err", err)
		return p.Kill()
	}
	return nil
}
