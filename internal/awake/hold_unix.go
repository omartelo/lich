//go:build darwin || linux

package awake

import (
	"fmt"
	"log/slog"
	"os/exec"
)

// hold runs the OS's own inhibitor around a `cat` reading a pipe lich holds.
// Closing that pipe is the release: cat sees EOF and exits, the inhibitor
// exits behind it and the OS drops the assertion, and the same thing happens
// on its own when lich dies, however it dies, so a crash never leaves a
// machine that cannot sleep. Killing the inhibitor directly would not do:
// neither tool forwards a signal to the child it is waiting on.
func hold() (func(), error) {
	argv := holdArgv()
	cmd := exec.Command(argv[0], argv[1:]...)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, err
	}
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("%s: %w", argv[0], err)
	}
	return func() {
		_ = stdin.Close()
		// The one place an inhibitor that never took its lock shows: it exits
		// non-zero at once without running cat (systemd-inhibit with no logind
		// does), Start still succeeds, and only Wait carries the verdict.
		if err := cmd.Wait(); err != nil {
			slog.Warn("keep awake ended", "cmd", argv[0], "err", err)
		}
	}, nil
}
