//go:build darwin

package quota

import (
	"errors"
	"os/exec"
	"reflect"
	"testing"
)

func TestClaudeKeychainUsesTheSessionItemBeforeAnyFile(t *testing.T) {
	a := Account{Read: true, Env: map[string]string{
		claudeDirVar: credsDir(t, claudeCredsJSON), claudeSecureDirVar: "/work", "USER": "session-user",
	}}
	creds, status := readClaudeKeychain(a, func(args ...string) ([]byte, error) {
		want := []string{"find-generic-password", "-a", "session-user", "-w", "-s", "Claude Code-credentials-0c9a453f"}
		if !reflect.DeepEqual(args, want) {
			t.Fatalf("security arguments = %q, want %q", args, want)
		}
		return []byte(`{"claudeAiOauth":{"accessToken":"keychain-token"}}`), nil
	})
	if status != StatusOK || creds.OAuth.AccessToken != "keychain-token" {
		t.Fatal("Keychain did not supply the login")
	}
}

func TestClaudeKeychainFailureNeverBorrowsFileLogin(t *testing.T) {
	a := Account{Read: true, Env: map[string]string{claudeDirVar: credsDir(t, claudeCredsJSON), "USER": "unsafe name"}}
	for _, tc := range []struct {
		data string
		err  error
	}{
		{"", errors.New("locked, refused or timed out")},
		{"not JSON", nil},
		{`{"claudeAiOauth":{}}`, nil},
	} {
		creds, status := readClaudeKeychain(a, func(args ...string) ([]byte, error) {
			if args[2] != "claude-code-user" {
				t.Fatal("unsafe username did not use Claude's fallback")
			}
			return []byte(tc.data), tc.err
		})
		if status != StatusUnknown || creds.OAuth.AccessToken != "" {
			t.Fatal("failed Keychain read must stay unknown")
		}
	}
}

func TestClaudeMissingKeychainItemReadsOnlyItsSelectedFile(t *testing.T) {
	a := Account{Read: true, Env: map[string]string{claudeDirVar: credsDir(t, claudeCredsJSON), "USER": "test"}}
	missing := exec.Command("/bin/sh", "-c", "exit 44").Run()
	if missing == nil {
		t.Fatal("fixture must exit with errSecItemNotFound's status")
	}
	creds, status := readClaudeKeychain(a, func(...string) ([]byte, error) { return nil, missing })
	if status != StatusOK || creds.OAuth.AccessToken != "tok-claude" {
		t.Fatal("missing Keychain item did not read its file fallback")
	}
}
