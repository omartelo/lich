//go:build !darwin

package chromium

// TabFallback is macOS's alone (fallback_darwin.go): here a missing or dying
// window is the error the user sees.
const TabFallback = false
