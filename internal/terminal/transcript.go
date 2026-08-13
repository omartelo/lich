package terminal

import (
	"os"
	"path/filepath"
)

// transcriptPath locates one conversation's transcript on disk. Every provider
// files them the same way — a home overridable by one environment variable, and
// a path under it that only a glob can complete, since the parts lich does not
// know (the project slug, the date a rollout started) are the provider's own
// encoding of things it never told us. The UUID in the pattern keeps at most one
// file matching. False when the home cannot be resolved or nothing matches yet.
func transcriptPath(homeVar, homeDir string, pattern ...string) (string, bool) {
	base := os.Getenv(homeVar)
	if base == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", false
		}
		base = filepath.Join(home, homeDir)
	}
	matches, err := filepath.Glob(filepath.Join(append([]string{base}, pattern...)...))
	if err != nil || len(matches) == 0 {
		return "", false
	}
	return matches[0], true
}

// claudeTranscriptPath resolves a Claude conversation by its UUID under
// $CLAUDE_CONFIG_DIR, else ~/.claude.
func claudeTranscriptPath(providerSessionID string) (string, bool) {
	return transcriptPath("CLAUDE_CONFIG_DIR", ".claude",
		"projects", "*", providerSessionID+".jsonl")
}

// codexTranscriptPath resolves a Codex rollout by its UUID under $CODEX_HOME,
// else ~/.codex. Codex files a rollout by the date it started —
// sessions/<yyyy>/<mm>/<dd>/rollout-<timestamp>-<uuid>.jsonl.
func codexTranscriptPath(providerSessionID string) (string, bool) {
	return transcriptPath("CODEX_HOME", ".codex",
		"sessions", "*", "*", "*", "rollout-*-"+providerSessionID+".jsonl")
}
