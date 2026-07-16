//go:build !windows

package terminal

import "os"

// defaultShell is spawned for "shell" sessions when the environment names none.
const defaultShell = "/bin/sh"

// userShell reads the user's shell from the environment, "" when unset.
func userShell() string { return os.Getenv("SHELL") }
