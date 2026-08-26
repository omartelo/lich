//go:build linux

package sandbox

import (
	"os"
	"path/filepath"
	"strings"
)

// x11SocketDir is where an X server puts the socket for each display it serves.
const x11SocketDir = "/tmp/.X11-unix"

// displaySockets returns the display server sockets a confined session is given
// back, and they are there for one thing: the clipboard.
//
// Nothing else in this package needs a display. But copying out of an agent's
// TUI and pasting a screenshot into it are the agent's own work, not the
// terminal's — Claude Code shells out to wl-copy, wl-paste, xclip and xsel for
// both — and a private home that empties XDG_RUNTIME_DIR takes the compositor's
// socket with it, so every one of those tools fails to connect. Copy and image
// attach are basic terminal behaviour; a session that loses them is not a
// confined session, it is a broken one. ai-jail, which this sandbox is a port
// of, hands over the Wayland socket the same way.
//
// The cost is the honest one: a session that can reach the clipboard can read
// whatever the user copied into it, a password manager's paste included, and
// write it back. That is the trade the clipboard is, and it is written down in
// docs/ceilings.md.
//
// Wayland wins when the host has it, and X11 is the fallback rather than a
// second mount, because the X socket is the wider hole of the two: X clients
// are not isolated from one another, so a session given the X socket can watch
// every other client's input. A host running both (Xwayland) has no need of it.
func displaySockets(home string) []string {
	if socket := waylandSocket(); socket != "" {
		return []string{socket}
	}
	return x11Sockets(x11SocketDir, home)
}

// waylandSocket resolves the compositor's socket from the environment the way
// the Wayland spec says a client does: an absolute WAYLAND_DISPLAY names the
// socket outright, and a bare name is one path component under
// XDG_RUNTIME_DIR. A name with a separator in it is not a name — it is a path
// escaping the runtime directory — and is refused rather than joined.
func waylandSocket() string {
	display := os.Getenv("WAYLAND_DISPLAY")
	if display == "" {
		return ""
	}
	path := display
	if !filepath.IsAbs(path) {
		runtime := os.Getenv("XDG_RUNTIME_DIR")
		if !filepath.IsAbs(runtime) || strings.ContainsRune(display, filepath.Separator) {
			return ""
		}
		path = filepath.Join(runtime, display)
	}
	if !isSocket(path) {
		return ""
	}
	return path
}

// x11Sockets resolves the X display's socket and the cookie file that
// authorises a connection to it. Only a local display has a socket to mount:
// DISPLAY is ":N" or ":N.screen" locally, and anything else names a server
// reached over TCP, which the sandbox already has (the network is never cut).
//
// The cookie is XAUTHORITY when the display manager exported one, and
// ~/.Xauthority otherwise — inside the home the sandbox is about to empty,
// which is exactly why it has to be named here. A cookie that is a symlink is
// dropped by existing() along with every other link, and an X11 session whose
// cookie is symlinked connects to nothing.
func x11Sockets(dir, home string) []string {
	display := os.Getenv("DISPLAY")
	if !strings.HasPrefix(display, ":") {
		return nil
	}
	number, _, _ := strings.Cut(strings.TrimPrefix(display, ":"), ".")
	if number == "" {
		return nil
	}
	socket := filepath.Join(dir, "X"+number)
	if !isSocket(socket) {
		return nil
	}
	cookie := os.Getenv("XAUTHORITY")
	if !filepath.IsAbs(cookie) {
		cookie = filepath.Join(home, ".Xauthority")
	}
	return []string{socket, cookie}
}
