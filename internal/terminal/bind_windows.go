//go:build windows

package terminal

import (
	"errors"

	"golang.org/x/sys/windows"
)

// addrInUse reports whether a bind failed because the port is taken — the one
// error worth retrying, since only a taken port can free itself. Windows
// surfaces this as WSAEADDRINUSE, which syscall.Errno.Is does not map to the
// portable syscall.EADDRINUSE, so the check names the real constant.
func addrInUse(err error) bool {
	return errors.Is(err, windows.WSAEADDRINUSE)
}
