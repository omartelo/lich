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
	case providers.OMP:
		_, ok := ompTranscriptPath(providerSessionID)
		return ok
	}
	return false
}

// ompTranscriptPath locates a conversation's transcript by its id under oh-my-pi's
// agent directory. omp files one JSONL per session, in a directory named after
// the cwd it ran in — sessions/<encoded-cwd>/<timestamp>_<id>.jsonl — so the
// directory and the timestamp are globbed and the id is what identifies it.
//
// The directory is the one `omp config path` prints: a named profile moves the
// whole thing and wins outright, then the explicit override, then the default.
// The same rule lives in internal/agentplugin/omp.go, which installs into it —
// each package resolves its own harness paths, as the Claude Code pair does.
func ompTranscriptPath(providerSessionID string) (string, bool) {
	base, ok := ompAgentDir()
	if !ok {
		return "", false
	}
	pattern := filepath.Join(base, "sessions", "*", "*_"+providerSessionID+".jsonl")
	matches, err := filepath.Glob(pattern)
	if err != nil || len(matches) == 0 {
		return "", false
	}
	return matches[0], true
}

// ompAgentDir resolves oh-my-pi's agent directory, or false when there is no
// home to hang it off.
func ompAgentDir() (string, bool) {
	if profile := os.Getenv("OMP_PROFILE"); profile != "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", false
		}
		return filepath.Join(home, ".omp", "profiles", profile, "agent"), true
	}
	if dir := os.Getenv("PI_CODING_AGENT_DIR"); dir != "" {
		return dir, true
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", false
	}
	return filepath.Join(home, ".omp", "agent"), true
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
