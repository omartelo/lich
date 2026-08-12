//go:build !windows

package terminal

import (
	"errors"
	"syscall"
)

// addrInUse reports whether a bind failed because the port is taken — the one
// error worth retrying, since only a taken port can free itself.
func addrInUse(err error) bool {
	return errors.Is(err, syscall.EADDRINUSE)
}
