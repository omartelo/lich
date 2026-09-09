package quota

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func unsetTestEnv(t *testing.T, name string) {
	t.Helper()
	t.Setenv(name, "")
	if err := os.Unsetenv(name); err != nil {
		t.Fatal(err)
	}
}

func TestClaudeStorageSelection(t *testing.T) {
	for _, tc := range []struct {
		name         string
		env          map[string]string
		dir, service string
	}{
		{"default", map[string]string{}, "", "Claude Code-credentials"},
		{"config", map[string]string{claudeDirVar: "/work"}, "/work", "Claude Code-credentials-0c9a453f"},
		{"override", map[string]string{claudeDirVar: "/other", claudeSecureDirVar: "/work"}, "/work", "Claude Code-credentials-0c9a453f"},
		{"empty overrides config", map[string]string{claudeDirVar: "/work", claudeSecureDirVar: ""}, "", "Claude Code-credentials"},
		{"NFC", map[string]string{claudeSecureDirVar: "/cafe\u0301"}, "/café", "Claude Code-credentials-a434c8fb"},
		{"NFC config", map[string]string{claudeDirVar: "/cafe\u0301"}, "/café", "Claude Code-credentials-a434c8fb"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			a := Account{Env: tc.env, Read: true}
			if claudeStorageDir(a) != tc.dir || claudeKeychainService(a) != tc.service {
				t.Errorf("storage = %q / %q, want %q / %q", claudeStorageDir(a), claudeKeychainService(a), tc.dir, tc.service)
			}
		})
	}
}

func TestAccountEnvironmentDoesNotBorrowLichValues(t *testing.T) {
	t.Setenv(claudeSecureDirVar, "/lich-storage")
	t.Setenv(claudeDirVar, "/lich-config")
	t.Setenv(claudeTokenVar, "lich-token")
	a := Account{Read: true, Env: map[string]string{claudeDirVar: "/session"}}
	if claudeStorageDir(a) != "/session" || a.lookup(claudeTokenVar) != "" {
		t.Fatal("session inherited lich's login")
	}
	if dir := claudeStorageDir(lichEnv()); dir != "/lich-storage" {
		t.Error("machine-wide lookup lost its environment")
	}
	unsetTestEnv(t, claudeSecureDirVar)
	if dir := claudeStorageDir(lichEnv()); dir != "/lich-config" {
		t.Error("machine-wide lookup did not fall back to config")
	}
}

func TestSecureStoragePresenceSeparatesCachedAccounts(t *testing.T) {
	a := Account{Read: true, Env: map[string]string{claudeDirVar: "/work"}}
	absent := cacheKey(a)
	a.Env[claudeSecureDirVar] = ""
	if cacheKey(a) == absent {
		t.Fatal("empty override reused a different Keychain account's quota")
	}
	a.Env[claudeSecureDirVar] = "/elsewhere"
	if cacheKey(a) == absent {
		t.Fatal("separate secure storage reused another account's quota")
	}
}

func TestClaudeFileUsesSecureStorageOnWindowsAndLinux(t *testing.T) {
	config := credsDir(t, claudeCredsJSON)
	storage := credsDir(t, `{"claudeAiOauth":{"accessToken":"storage-token"}}`)
	a := Account{Read: true, Env: map[string]string{claudeDirVar: config, claudeSecureDirVar: storage}}
	creds, status := readClaudeFile(a)
	if status != StatusOK || creds.OAuth.AccessToken != "storage-token" {
		t.Fatal("file reader did not select secure storage")
	}
	a.Env[claudeSecureDirVar] = ""
	a.Env[accountHomeVar] = t.TempDir()
	if err := os.Rename(storage, filepath.Join(a.Env[accountHomeVar], ".claude")); err != nil {
		t.Fatal(err)
	}
	if creds, status = readClaudeFile(a); status != StatusOK || creds.OAuth.AccessToken != "storage-token" {
		t.Fatal("empty override did not select home/.claude")
	}
	a.Env[claudeSecureDirVar] = "relative"
	if _, status = readClaudeFile(a); status != StatusUnknown {
		t.Fatal("relative storage must not resolve against lich's cwd")
	}
}

func TestUnreadableDefaultSessionIsAlsoUnknown(t *testing.T) {
	writeCreds(t, claudeCredsJSON, codexCredsJSON)
	s := newService("", "", time.Now())
	s.SetSessions(func(string) Account { return Account{} })
	for _, p := range s.Plans("unreadable") {
		if p.Status != StatusUnknown || len(p.Windows) != 0 {
			t.Fatal("unreadable default session borrowed the machine login")
		}
	}
}
