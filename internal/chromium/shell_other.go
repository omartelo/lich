//go:build !linux && !windows && !(darwin && arm64)

package chromium

// An Intel Mac ships no window: it is built on the Apple Silicon runner alone
// (docs/ceilings.md). It answers as if none were bundled, and lich opens as a
// tab instead (TabFallback).
func bundledShell() string { return "" }
