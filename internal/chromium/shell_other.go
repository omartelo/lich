//go:build !linux

package chromium

// Windows and macOS still open the system browser: the window has only been
// built and measured on Linux (docs/ceilings.md). Both answer as if no window
// were bundled, and the ladder below carries on unchanged.
const shellExpected = false

func bundledShell() string { return "" }
