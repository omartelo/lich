//go:build !linux && !windows

package chromium

// macOS still opens the system browser: the window has not been built there
// (docs/ceilings.md). It answers as if no window were bundled, and the ladder
// below carries on unchanged.
const shellExpected = false

func bundledShell() string { return "" }
