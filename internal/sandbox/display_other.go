//go:build !linux

package sandbox

// displaySockets answers for the platforms with no X11 or Wayland socket to
// hand over. macOS reaches its pasteboard through a mach service rather than a
// socket, so a confined session there has no clipboard at all — see
// docs/ceilings.md, along with the rest of that profile having never run.
func displaySockets(string) []string {
	return nil
}
