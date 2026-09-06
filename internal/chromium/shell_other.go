//go:build !linux && !windows && !(darwin && arm64)

package chromium

// An Intel Mac still opens the system browser: the window is built on the
// Apple Silicon runner alone (docs/ceilings.md). It answers as if no window
// were bundled, and the ladder below carries on unchanged.
const shellExpected = false

func bundledShell() string { return "" }
