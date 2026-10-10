// Package restart owns how a running lich ends and how its window comes and
// goes. The backend outlives its window (Window); it ends only when stopped, by
// `lich quit`, a signal, or a restart: a detached successor is started and this
// process is stopped, freeing the pinned listener port for the successor to
// bind. The restart is what the POST /restart endpoint drives after install.sh
// replaces the binary on disk.
package restart

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"sync"
)

// WaitEnv marks a lich process spawned to succeed a restarting one. The
// listener bind path retries while this is set, because the outgoing process
// still holds the pinned port for a moment (see internal/terminal transport).
const WaitEnv = "LICH_RESTART_WAIT"

// Coordinator ends this lich: on its own (Quit), or after launching a detached
// successor (Do, Install), so this process unwinds, exits, and frees the port.
type Coordinator struct {
	mu      sync.Mutex
	stop    func()
	started bool
	exePath string
	env     []string
	// spawn is the seam for tests; it defaults to the build-tagged primitive.
	spawn func(exe string, env, args []string) error
}

// New returns a coordinator that relaunches exePath with env (plus the wait
// marker). env should be the current process environment so the successor pins
// the same listener port.
func New(exePath string, env []string) *Coordinator {
	return &Coordinator{
		exePath: exePath,
		env:     env,
		spawn:   startDetached,
	}
}

// SetStop supplies the clean exit: stop makes main close the window, unwind its
// defers and return. Called once lich is serving; a restart before that only
// spawns the successor.
func (c *Coordinator) SetStop(stop func()) {
	c.mu.Lock()
	c.stop = stop
	c.mu.Unlock()
}

// Quit ends this lich: every session goes with it, and nothing is launched in
// its place.
func (c *Coordinator) Quit() error {
	c.mu.Lock()
	stop := c.stop
	c.mu.Unlock()
	if stop == nil {
		return errors.New("quit: lich is still starting")
	}
	stop()
	return nil
}

// Do launches the successor and stops this lich. Order matters: the successor
// starts first and blocks retrying the pinned port; then this process exits,
// and the freed port lets the successor bind and open a fresh window.
func (c *Coordinator) Do() error {
	return c.launch(c.exePath, nil)
}

// Install hands replacement and relaunch to Inno Setup. It waits for this PID
// to exit before touching files, so the window and main's defers finish first.
func (c *Coordinator) Install(installer string) error {
	return c.launch(installer, installerArgs(filepath.Dir(c.exePath), os.Getpid()))
}

// installerArgs is half of a contract with build/windows/lich.iss. /UPDATEPID is
// no Inno switch: the script reads it back as {param:UPDATEPID|0} to tell an
// update from a fresh install and to wait for this process to release its exe.
// Rename it on one side only and the installer stops waiting, silently.
func installerArgs(dir string, pid int) []string {
	return []string{"/SILENT", "/NORESTART", "/CLOSEAPPLICATIONS", "/NORESTARTAPPLICATIONS",
		"/DIR=" + dir, "/UPDATEPID=" + strconv.Itoa(pid),
		"/LOG=" + filepath.Join(dir, ".lich-update-setup.log")}
}

func (c *Coordinator) launch(exe string, args []string) error {
	if c.exePath == "" {
		return errors.New("restart: executable path unknown")
	}
	// Once only: a second /restart (two install.sh runs) must not spawn a second
	// successor that would then lose the port race and burn the bind timeout.
	// The latch is set only after a successful spawn — a failed launch (say,
	// the exe mid-swap by the package manager) must leave /restart retryable,
	// not silently dead. The lock spans the spawn so concurrent calls cannot
	// both slip past the check.
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.started {
		return nil
	}
	if err := c.spawn(exe, successorEnv(c.env), args); err != nil {
		return fmt.Errorf("restart: launch successor: %w", err)
	}
	c.started = true
	if c.stop != nil {
		c.stop()
	}
	return nil
}

// successorEnv is env plus the wait marker, on a fresh slice.
func successorEnv(env []string) []string {
	return append(append([]string(nil), env...), WaitEnv+"=1")
}

// closeWindowCommand is how Windows asks a GUI process to close: taskkill
// without /F posts WM_CLOSE to the process's windows rather than ending it
// where it stands. It is resolved under SystemRoot instead of through PATH —
// what this command reaches decides whether somebody's window is closed or
// killed, and PATH is the user's to rearrange. Kept out of the build-tagged
// file so the pure logic tests on any OS.
func closeWindowCommand(getenv func(string) string, pid int) (string, []string) {
	exe := "taskkill.exe"
	if root := getenv("SystemRoot"); root != "" {
		exe = root + `\System32\taskkill.exe`
	}
	return exe, []string{"/PID", strconv.Itoa(pid)}
}
