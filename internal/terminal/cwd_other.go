//go:build !linux

package terminal

// cwdTracked: see cwd_linux.go — no /proc here, so the card keeps showing the
// directory the session started in.
const cwdTracked = false

func processCwd(int) string { return "" }
