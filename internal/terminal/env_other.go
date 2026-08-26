//go:build !linux

package terminal

// envReadable: no other process's environment on this platform, so a session
// running a binary lich did not choose cannot be told apart from one running
// the default login — the quota reading is withheld rather than taken against
// the wrong account (internal/quota, StatusUnknown).
//
// macOS could answer this through sysctl's KERN_PROCARGS2, the way
// cwd_darwin.go answers the working directory; nobody has had the hardware to
// write and prove it. Windows exposes nothing short of reading another
// process's memory.
const envReadable = false

func readEnv(int) map[string]string { return nil }
