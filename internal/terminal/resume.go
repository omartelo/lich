package terminal

import (
	"os"
	"path/filepath"

	"github.com/omartelo/lich/internal/providers"
)

// ResumeAvailable reports whether the conversation providerSessionID names can
// still be reopened, so the frontend only offers a resume it can honour.
//
// The session row keeps its provider session id for good, but a provider prunes
// its own transcripts, so the id outlives the conversation it points at. Resuming
// one then dies inside the PTY with the provider's error in place of a session —
// the shortcut Start's doc names. The transcript on disk is what answers the
// question; this asks it for existence alone.
//
// False for a provider with no resume wired (resumeArgs): there is nothing to
// offer where the CLI cannot reopen a conversation by id.
func (*Service) ResumeAvailable(kind, providerSessionID string) bool {
	if providerSessionID == "" {
		return false
	}
	switch kind {
	case providers.Claude:
		_, ok := claudeTranscriptPath(providerSessionID)
		return ok
	case providers.Codex:
		_, ok := codexTranscriptPath(providerSessionID)
		return ok
	}
	return false
}

// codexTranscriptPath locates a conversation's rollout by its UUID under the
// Codex home ($CODEX_HOME, else ~/.codex). Codex files a rollout by the date it
// started — sessions/<yyyy>/<mm>/<dd>/rollout-<timestamp>-<uuid>.jsonl — so the
// date components are globbed rather than guessed; the UUID keeps at most one
// file matching. False when none does yet.
func codexTranscriptPath(providerSessionID string) (string, bool) {
	base := os.Getenv("CODEX_HOME")
	if base == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", false
		}
		base = filepath.Join(home, ".codex")
	}
	pattern := filepath.Join(base, "sessions", "*", "*", "*", "rollout-*-"+providerSessionID+".jsonl")
	matches, err := filepath.Glob(pattern)
	if err != nil || len(matches) == 0 {
		return "", false
	}
	return matches[0], true
}
