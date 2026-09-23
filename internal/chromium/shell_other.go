//go:build !linux && !windows && !darwin

package chromium

// No package ships a window here (docs/ceilings.md): the install answers as if
// none were bundled, and lich reports the missing window as any bundled
// install would.
func bundledShell() string { return "" }
