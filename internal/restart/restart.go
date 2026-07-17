// Package restart relaunches lich in place: it starts a detached successor
// process and closes the current window so this process exits and frees the
// pinned listener port for the successor to bind. It is what the POST /restart
// endpoint drives after install.sh replaces the binary on disk.
package restart

import (
	"errors"
	"fmt"
	"os"
	"sync"
)

// WaitEnv marks a lich process spawned to succeed a restarting one. The
// listener bind path retries while this is set, because the outgoing process
// still holds the pinned port for a moment (see internal/terminal transport).
const WaitEnv = "LICH_RESTART_WAIT"

// Coordinator restarts this lich: launch a detached successor, then terminate
// the window so this process unwinds, exits, and frees the port.
type Coordinator struct {
	mu      sync.Mutex
	window  *os.Process
	started bool
	exePath string
	env     []string
	// seams for tests; default to the build-tagged process primitives.
	spawn     func(exe string, env []string) error
	terminate func(p *os.Process) error
}

// New returns a coordinator that relaunches exePath with env (plus the wait
// marker). env should be the current process environment so the successor pins
// the same listener port.
func New(exePath string, env []string) *Coordinator {
	return &Coordinator{
		exePath:   exePath,
		env:       env,
		spawn:     startDetached,
		terminate: terminateProcess,
	}
}

// SetWindow records the Chromium process whose exit ends this lich's lifecycle.
// Called once the window is up; a restart before that only spawns the successor.
func (c *Coordinator) SetWindow(p *os.Process) {
	c.mu.Lock()
	c.window = p
	c.mu.Unlock()
}

// Do launches the successor and closes the window. Order matters: the successor
// starts first and blocks retrying the pinned port; then the window dies, this
// process exits, and the freed port lets the successor bind and open a fresh
// window.
func (c *Coordinator) Do() error {
	if c.exePath == "" {
		return errors.New("restart: executable path unknown")
	}
	// Once only: a second /restart (two install.sh runs) must not spawn a second
	// successor that would then lose the port race and burn the bind timeout.
	c.mu.Lock()
	if c.started {
		c.mu.Unlock()
		return nil
	}
	c.started = true
	win := c.window
	c.mu.Unlock()

	if err := c.spawn(c.exePath, successorEnv(c.env)); err != nil {
		return fmt.Errorf("restart: launch successor: %w", err)
	}
	if win != nil {
		if err := c.terminate(win); err != nil {
			return fmt.Errorf("restart: close window: %w", err)
		}
	}
	return nil
}

// successorEnv is env plus the wait marker, on a fresh slice.
func successorEnv(env []string) []string {
	return append(append([]string(nil), env...), WaitEnv+"=1")
}
