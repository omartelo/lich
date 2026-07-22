//go:build !linux && !darwin && !windows

package terminal

// cwdTracked: no live cwd read on this platform, so the card keeps showing the
// directory the session started in.
const cwdTracked = false

func processCwd(int) string { return "" }
