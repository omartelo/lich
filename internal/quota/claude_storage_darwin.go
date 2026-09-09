//go:build darwin

package quota

import (
	"context"
	"encoding/json"
	"os/exec"
	"os/user"
	"regexp"
	"time"
)

const (
	keychainTimeout      = 2 * time.Second
	keychainItemNotFound = 44
)

var claudeKeychainUser = regexp.MustCompile(`^[a-zA-Z0-9._-]+$`)

func readClaudeCredentials(a Account) (claudeCredentials, string) {
	return readClaudeKeychain(a, func(args ...string) ([]byte, error) {
		ctx, cancel := context.WithTimeout(context.Background(), keychainTimeout)
		defer cancel()
		return exec.CommandContext(ctx, "/usr/bin/security", args...).Output()
	})
}

func readClaudeKeychain(a Account, run func(...string) ([]byte, error)) (claudeCredentials, string) {
	var creds claudeCredentials
	// UZ in the cited CLI selects USER, then os.userInfo().username, sanitizing
	// unsupported names to claude-code-user. Match the account as well as service.
	name := a.lookup("USER")
	if name == "" {
		current, err := user.Current()
		if err != nil {
			return creds, StatusUnknown
		}
		name = current.Username
	}
	if !claudeKeychainUser.MatchString(name) {
		name = "claude-code-user"
	}
	data, err := run("find-generic-password", "-a", name, "-w", "-s", claudeKeychainService(a))
	if err != nil {
		// security truncates errSecItemNotFound (-25300) to exit status 44.
		// Only an absent item allows Claude's plaintext fallback; a locked or
		// refused Keychain must never send a different file login's quota.
		if exit, ok := err.(*exec.ExitError); ok && exit.ExitCode() == keychainItemNotFound {
			return readClaudeFile(a)
		}
		return creds, StatusUnknown
	}
	if json.Unmarshal(data, &creds) != nil || creds.OAuth.AccessToken == "" {
		return creds, StatusUnknown
	}
	return creds, StatusOK
}
