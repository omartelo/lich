//go:build !darwin

package quota

// readCursorKeychain is empty off macOS: cursor-agent keeps its session in
// auth.json there.
func readCursorKeychain() string { return "" }
