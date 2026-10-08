//go:build darwin

package quota

import (
	"context"
	"os/exec"
	"strings"
)

// The Keychain item cursor-agent 2026.06+ stores its session under, as the
// Orca project read it.
const (
	cursorKeychainService = "cursor-access-token"
	cursorKeychainAccount = "cursor-user"
)

// readCursorKeychain is empty for a missing item and for a refused or locked
// Keychain alike: either way auth.json is the next place cursor-agent looks.
func readCursorKeychain() string {
	return cursorKeychainItem(func(args ...string) ([]byte, error) {
		ctx, cancel := context.WithTimeout(context.Background(), keychainTimeout)
		defer cancel()
		return exec.CommandContext(ctx, "/usr/bin/security", args...).Output()
	})
}

func cursorKeychainItem(run func(...string) ([]byte, error)) string {
	out, err := run("find-generic-password", "-a", cursorKeychainAccount, "-w", "-s", cursorKeychainService)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}
