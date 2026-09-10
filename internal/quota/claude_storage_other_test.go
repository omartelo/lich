//go:build !darwin

package quota

import (
	"net/http"
	"testing"
)

// Every other test injects claudeLogin, so this is the only place the reader
// New actually ships is exercised end to end.
func TestNewReadsTheShippedCredentialStore(t *testing.T) {
	writeCreds(t, claudeCredsJSON, "")
	url, _ := serve(t, http.StatusOK, claudeLimitsBody)
	s := New()
	s.claudeURL = url
	// Leave the profile route unreachable: naming the account is a second
	// request this test has no server for, and an unnamed one is not a failure.
	s.profileURL = ""

	got := s.claudePlan(lichEnv())
	if got.Status != StatusOK || got.Plan != "Max 5x" || len(got.Windows) == 0 {
		t.Fatalf("plan = %+v, want an ok Max 5x reading through readClaudeCredentials", got)
	}
}
