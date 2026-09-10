package quota

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"golang.org/x/text/unicode/norm"
)

// Claude Code 2.1.265, bundled chunk-ymgd59rk.js (Fy/sye), inspected from the
// official binary distributed via https://www.npmjs.com/package/@anthropic-ai/claude-code/v/2.1.265:
// secure-storage overrides config when defined, even empty; nonempty values
// are NFC-normalized before SHA-256, and the service uses the first 8 hex digits.
func claudeStorageDir(a Account) string {
	if dir, defined := a.lookupEnv(claudeSecureDirVar); defined {
		return norm.NFC.String(dir)
	}
	return norm.NFC.String(a.lookup(claudeDirVar))
}

func claudeKeychainService(a Account) string {
	const service = "Claude Code-credentials"
	if dir := claudeStorageDir(a); dir != "" {
		digest := sha256.Sum256([]byte(dir))
		return fmt.Sprintf("%s-%x", service, digest[:4])
	}
	return service
}

// The same CLI's plaintext storage adapter uses Fy()/.credentials.json on
// Windows (and Linux), not Credential Manager or APPDATA. An empty override
// selects homedir/.claude. NFC is applied to the directory, not the filename.
func readClaudeFile(a Account) (claudeCredentials, string) {
	var creds claudeCredentials
	dir := claudeStorageDir(a)
	if dir == "" {
		path, ok := harnessFile(a, "", ".claude")
		if !ok {
			return creds, StatusUnknown
		}
		dir = norm.NFC.String(path)
	}
	// A relative directory resolves against the provider's cwd, which this
	// account seam does not carry. Never resolve it against lich's directory.
	if !filepath.IsAbs(dir) {
		return creds, StatusUnknown
	}
	path := filepath.Join(dir, ".credentials.json")
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return creds, StatusSignedOut
	}
	// A file lich is not allowed to open is the one case it cannot answer for:
	// the login may well be good behind those permissions.
	if err != nil {
		return creds, StatusUnknown
	}
	// A file that parses to no token is a login the user has to redo, exactly
	// like no file at all: a half-written or truncated credentials file is the
	// common shape of that, and "Run `claude login`" is the instruction it
	// needs. Same answer codexPlan gives for the same file (codex.go).
	if json.Unmarshal(data, &creds) != nil || creds.OAuth.AccessToken == "" {
		return claudeCredentials{}, StatusSignedOut
	}
	return creds, StatusOK
}
