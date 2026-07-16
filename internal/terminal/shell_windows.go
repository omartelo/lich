//go:build windows

package terminal

import "os"

// defaultShell is spawned for "shell" sessions when the environment names none.
const defaultShell = "cmd.exe"

// userShell reads the user's shell from the environment, "" when unset.
// Windows has no $SHELL; COMSPEC is the closest equivalent.
func userShell() string { return os.Getenv("COMSPEC") }
