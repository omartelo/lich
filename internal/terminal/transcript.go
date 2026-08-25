package terminal

import (
	"crypto/md5"
	"fmt"
	"os"
	"path/filepath"
)

// globTranscript completes a transcript path under base: the parts lich does not
// know — the project slug, the date a rollout started, the timestamp in a file
// name — are the provider's own encoding of things it never told us, so they are
// globbed rather than reconstructed. The conversation id in the pattern keeps at
// most one file matching. False when nothing matches yet.
func globTranscript(base string, pattern ...string) (string, bool) {
	matches, err := filepath.Glob(filepath.Join(append([]string{base}, pattern...)...))
	if err != nil || len(matches) == 0 {
		return "", false
	}
	return matches[0], true
}

// harnessDir is where a harness keeps its state: the directory its own
// environment variable names, else sub under the user's home. False when there
// is no home to hang it off.
func harnessDir(homeVar, sub string) (string, bool) {
	if base := os.Getenv(homeVar); base != "" {
		return base, true
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", false
	}
	return filepath.Join(home, sub), true
}

// claudeTranscriptPath locates a Claude conversation by its UUID under
// $CLAUDE_CONFIG_DIR, else ~/.claude.
func claudeTranscriptPath(providerSessionID string) (string, bool) {
	base, ok := harnessDir("CLAUDE_CONFIG_DIR", ".claude")
	if !ok {
		return "", false
	}
	return globTranscript(base, "projects", "*", providerSessionID+".jsonl")
}

// claudeSubagentPaths locates the transcripts of the sub-agents a Claude
// conversation spawned. A sub-agent's conversation is a file of its own beside
// the parent's — projects/<slug>/<id>/subagents/agent-<id>.jsonl — where it used
// to be sidechain lines written into the parent itself, and its tokens are
// billed to the same account. Empty for a conversation that spawned none, which
// is most of them.
func claudeSubagentPaths(providerSessionID string) []string {
	base, ok := harnessDir("CLAUDE_CONFIG_DIR", ".claude")
	if !ok {
		return nil
	}
	matches, err := filepath.Glob(
		filepath.Join(base, "projects", "*", providerSessionID, "subagents", "*.jsonl"),
	)
	if err != nil {
		return nil
	}
	return matches
}

// codexTranscriptPath locates a Codex rollout by its UUID under $CODEX_HOME,
// else ~/.codex. Codex files a rollout by the date it started —
// sessions/<yyyy>/<mm>/<dd>/rollout-<timestamp>-<uuid>.jsonl.
func codexTranscriptPath(providerSessionID string) (string, bool) {
	base, ok := harnessDir("CODEX_HOME", ".codex")
	if !ok {
		return "", false
	}
	return globTranscript(base, "sessions", "*", "*", "*", "rollout-*-"+providerSessionID+".jsonl")
}

// antigravityConversationPath is the database Antigravity keeps one
// conversation in, under its own directory beneath ~/.gemini — the CLI reads
// that root through no environment variable of its own (1.1.19 falls back to a
// hardcoded ".gemini" when it cannot resolve the home), so there is nothing to
// honour here but the home itself.
//
// It is not a transcript: Antigravity files the conversation as SQLite and
// writes its JSONL under `brain/<id>/`, a path the hook payload hands over
// rather than one lich reconstructs. This resolves the one file whose existence
// answers "can this id still be reopened".
func antigravityConversationPath(providerSessionID string) (string, bool) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", false
	}
	path := filepath.Join(home, ".gemini", "antigravity-cli", "conversations", providerSessionID+".db")
	if _, err := os.Stat(path); err != nil {
		return "", false
	}
	return path, true
}

// cursorChatStore is the database Cursor CLI keeps one chat in, under its own
// config directory. It is not a transcript: the CLI files a chat as SQLite at
// chats/<workspace>/<chatId>/store.db, which is the one file whose existence
// answers "can this id still be reopened".
//
// The workspace segment is the md5 of the resolved directory the chat ran in,
// which is why cwd is needed here the way it is for Crush: Cursor keeps its
// chats per checkout, so the same id proves nothing about another one. False
// without a cwd — a directory lich cannot name has no chat directory to ask.
func cursorChatStore(providerSessionID, cwd string) (string, bool) {
	base, ok := cursorConfigDir()
	if !ok || cwd == "" {
		return "", false
	}
	path := filepath.Join(base, "chats", cursorWorkspaceKey(cwd), providerSessionID, "store.db")
	if _, err := os.Stat(path); err != nil {
		return "", false
	}
	return path, true
}

// cursorWorkspaceKey is how Cursor names a checkout's chat directory: the hex
// md5 of the absolute, cleaned path. md5 is the CLI's choice and this only has
// to reproduce it — nothing here is a security decision.
func cursorWorkspaceKey(cwd string) string {
	abs, err := filepath.Abs(cwd)
	if err != nil {
		abs = filepath.Clean(cwd)
	}
	return fmt.Sprintf("%x", md5.Sum([]byte(abs)))
}

// cursorConfigDir resolves Cursor CLI's config directory, or false when there is
// no home to hang it off. It does not go through harnessDir because Cursor
// answers to two variables in a shape no other provider uses: $CURSOR_CONFIG_DIR
// wins outright, else $XDG_CONFIG_HOME with "cursor" under it — and the fallback
// when neither is set is ~/.cursor, not ~/.config/cursor. Reading it as
// xdg-basedir the way opencode and Crush are read would point at a directory
// Cursor never writes on a machine with no XDG_CONFIG_HOME, which is most of
// them. Measured on 2026.08.11; the same rule lives in internal/sandbox.
func cursorConfigDir() (string, bool) {
	if dir := os.Getenv("CURSOR_CONFIG_DIR"); dir != "" {
		return dir, true
	}
	if base := os.Getenv("XDG_CONFIG_HOME"); base != "" {
		return filepath.Join(base, "cursor"), true
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", false
	}
	return filepath.Join(home, ".cursor"), true
}

// ompTranscriptPath locates an oh-my-pi conversation by its id under omp's agent
// directory. omp files one JSONL per session, in a directory named after the cwd
// it ran in — sessions/<encoded-cwd>/<timestamp>_<id>.jsonl.
func ompTranscriptPath(providerSessionID string) (string, bool) {
	base, ok := ompAgentDir()
	if !ok {
		return "", false
	}
	return globTranscript(base, "sessions", "*", "*_"+providerSessionID+".jsonl")
}

// ompAgentDir resolves oh-my-pi's agent directory, or false when there is no
// home to hang it off. It does not go through harnessDir because omp answers to
// two variables and a named profile wins outright over the explicit override —
// the order `omp config path` was measured to apply. The same rule lives in
// internal/agentplugin/omp.go, which installs into it: each package resolves its
// own harness paths, as the Claude Code pair does.
func ompAgentDir() (string, bool) {
	if profile := os.Getenv("OMP_PROFILE"); profile != "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", false
		}
		return filepath.Join(home, ".omp", "profiles", profile, "agent"), true
	}
	return harnessDir("PI_CODING_AGENT_DIR", filepath.Join(".omp", "agent"))
}
