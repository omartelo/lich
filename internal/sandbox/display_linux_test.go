//go:build linux

package sandbox

import (
	"net"
	"os"
	"path/filepath"
	"slices"
	"testing"
)

// listenSocket puts a real unix socket on disk and returns its path: what
// displaySockets is asked to recognise, and the one thing a temp file cannot
// stand in for.
func listenSocket(t *testing.T, path string) string {
	t.Helper()
	listener, err := net.Listen("unix", path)
	if err != nil {
		t.Fatalf("listen on %s: %v", path, err)
	}
	t.Cleanup(func() { listener.Close() })
	return path
}

// clearDisplayEnv blanks the display variables, so a test answers for what it
// sets rather than for the desktop session running it.
func clearDisplayEnv(t *testing.T) {
	t.Helper()
	for _, name := range []string{"WAYLAND_DISPLAY", "XDG_RUNTIME_DIR", "DISPLAY", "XAUTHORITY"} {
		t.Setenv(name, "")
	}
}

func TestWaylandSocketUnderTheRuntimeDir(t *testing.T) {
	clearDisplayEnv(t)
	runtime := t.TempDir()
	want := listenSocket(t, filepath.Join(runtime, "wayland-1"))
	t.Setenv("XDG_RUNTIME_DIR", runtime)
	t.Setenv("WAYLAND_DISPLAY", "wayland-1")

	if got := waylandSocket(); got != want {
		t.Errorf("waylandSocket() = %q, want %q", got, want)
	}
}

// An absolute WAYLAND_DISPLAY names the socket outright, and the runtime
// directory is not consulted at all — the Wayland spec allows it and a
// compositor started by hand uses it.
func TestWaylandSocketAbsoluteDisplayNeedsNoRuntimeDir(t *testing.T) {
	clearDisplayEnv(t)
	want := listenSocket(t, filepath.Join(t.TempDir(), "wl"))
	t.Setenv("WAYLAND_DISPLAY", want)

	if got := waylandSocket(); got != want {
		t.Errorf("waylandSocket() = %q, want %q", got, want)
	}
}

// A bare display name is one path component. Anything with a separator in it is
// a path out of the runtime directory, and mounting whatever it reaches is how
// a display variable turns into a hole.
func TestWaylandSocketRefusesADisplayNameWithAPath(t *testing.T) {
	clearDisplayEnv(t)
	runtime := t.TempDir()
	escape := filepath.Join(runtime, "elsewhere")
	if err := os.Mkdir(escape, 0o700); err != nil {
		t.Fatal(err)
	}
	listenSocket(t, filepath.Join(escape, "wayland-1"))
	t.Setenv("XDG_RUNTIME_DIR", runtime)
	t.Setenv("WAYLAND_DISPLAY", filepath.Join("elsewhere", "wayland-1"))

	if got := waylandSocket(); got != "" {
		t.Errorf("waylandSocket() = %q, want nothing: the display name walks out of the runtime directory", got)
	}
}

func TestWaylandSocketRefusesWhatIsNotASocket(t *testing.T) {
	clearDisplayEnv(t)
	runtime := t.TempDir()
	if err := os.WriteFile(filepath.Join(runtime, "wayland-1"), nil, 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("XDG_RUNTIME_DIR", runtime)
	t.Setenv("WAYLAND_DISPLAY", "wayland-1")

	if got := waylandSocket(); got != "" {
		t.Errorf("waylandSocket() = %q, want nothing: the path is a regular file", got)
	}
}

// X11 is the fallback, and it carries the cookie that authorises the
// connection: without it the socket is mounted and every client is refused.
func TestX11SocketsCarryTheCookie(t *testing.T) {
	clearDisplayEnv(t)
	dir := t.TempDir()
	socket := listenSocket(t, filepath.Join(dir, "X7"))
	t.Setenv("DISPLAY", ":7.0")
	cookie := filepath.Join(t.TempDir(), "Xauthority")
	t.Setenv("XAUTHORITY", cookie)

	want := []string{socket, cookie}
	if got := x11Sockets(dir, "/home/u"); !slices.Equal(got, want) {
		t.Errorf("x11Sockets() = %v, want %v", got, want)
	}
}

// Without XAUTHORITY the cookie is the one in the home — which is the home the
// sandbox empties, so it has to be mounted back by name.
func TestX11SocketsFallBackToTheCookieInTheHome(t *testing.T) {
	clearDisplayEnv(t)
	dir := t.TempDir()
	socket := listenSocket(t, filepath.Join(dir, "X0"))
	t.Setenv("DISPLAY", ":0")

	want := []string{socket, filepath.Join("/home/u", ".Xauthority")}
	if got := x11Sockets(dir, "/home/u"); !slices.Equal(got, want) {
		t.Errorf("x11Sockets() = %v, want %v", got, want)
	}
}

// A DISPLAY naming a host is a server reached over TCP: there is no socket to
// mount, and the network is not cut, so the session reaches it either way.
func TestX11SocketsSkipARemoteDisplay(t *testing.T) {
	clearDisplayEnv(t)
	dir := t.TempDir()
	listenSocket(t, filepath.Join(dir, "X0"))
	t.Setenv("DISPLAY", "somehost:0")

	if got := x11Sockets(dir, "/home/u"); got != nil {
		t.Errorf("x11Sockets() = %v, want nothing for a remote display", got)
	}
}

// Wayland wins on a host running both. The X socket is the wider hole — X
// clients are not isolated from one another — and Xwayland means the host has
// one whether it needs it or not.
func TestDisplaySocketsPreferWayland(t *testing.T) {
	clearDisplayEnv(t)
	runtime := t.TempDir()
	want := listenSocket(t, filepath.Join(runtime, "wayland-1"))
	t.Setenv("XDG_RUNTIME_DIR", runtime)
	t.Setenv("WAYLAND_DISPLAY", "wayland-1")
	t.Setenv("DISPLAY", ":0")

	if got := displaySockets("/home/u"); !slices.Equal(got, []string{want}) {
		t.Errorf("displaySockets() = %v, want just the Wayland socket %q", got, want)
	}
}

// The clipboard is why any of this is mounted, so Describe has to carry it into
// the spec the backends read — not merely resolve it.
func TestDescribeMountsTheDisplaySocket(t *testing.T) {
	clearHarnessEnv(t)
	clearDisplayEnv(t)
	runtime := t.TempDir()
	socket := listenSocket(t, filepath.Join(runtime, "wayland-1"))
	t.Setenv("XDG_RUNTIME_DIR", runtime)
	t.Setenv("WAYLAND_DISPLAY", "wayland-1")

	spec := Describe("claude", t.TempDir(), t.TempDir(), "", nil)
	if !slices.Contains(spec.Read, socket) {
		t.Errorf("the display socket %q is not in the spec's readable paths %v", socket, spec.Read)
	}
}
