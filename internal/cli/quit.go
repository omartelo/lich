package cli

import (
	"context"
	"fmt"
	"net"
	"time"

	"github.com/omartelo/lich/internal/system"
)

// quitWait bounds waiting for a quitting lich to let go of its port. Most of it
// is the window it closes first, which gets five seconds to flush its profile
// (internal/restart closeWait); the rest is the database closing behind it.
const quitWait = 15 * time.Second

// quitPoll is how often the port is tried while lich is on its way out.
const quitPoll = 100 * time.Millisecond

// quit ends the running lich, sessions and all, and returns once it is gone, so
// a script can launch it again straight after. Closing the window no longer
// does this: the backend outlives its window.
func (c *client) quit(args []string) error {
	flags := newFlagSet("quit")
	if err := c.parse(flags, args); err != nil {
		return err
	}
	if flags.NArg() != 0 {
		return usageError("quit")
	}
	port, _, err := c.coordinates()
	if err != nil {
		return err
	}
	asked := c.call(context.Background(), "system.Quit", []any{system.QuitOptions{}}, shortCall, nil)
	// A lich that exits fast enough takes the reply down with it, so a failed
	// call is only a failure while lich is still there to have refused it.
	if asked != nil && listening(port) {
		return asked
	}
	if !gone(port, quitWait) {
		return fmt.Errorf("lich was asked to quit and still listens on port %s after %s", port, quitWait)
	}
	fmt.Fprintln(c.stdout, "lich has quit.")
	return nil
}

// gone waits up to wait for nothing to accept on port.
func gone(port string, wait time.Duration) bool {
	deadline := time.Now().Add(wait)
	for listening(port) {
		if time.Now().After(deadline) {
			return false
		}
		time.Sleep(quitPoll)
	}
	return true
}

func listening(port string) bool {
	conn, err := net.DialTimeout("tcp", net.JoinHostPort("127.0.0.1", port), quitPoll)
	if err != nil {
		return false
	}
	_ = conn.Close()
	return true
}
