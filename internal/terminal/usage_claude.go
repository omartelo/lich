package terminal

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
)

const (
	smallWindow   = 200_000
	defaultWindow = 1_000_000
)

// windowForModel resolves the context window the percent is taken against. The
// transcript records the model but not its window, so it is named: Haiku is the
// only Claude model still on 200k, everything else is 1M — naming the exception
// rather than the rule is what lets a model released after this build read right.
// A count past 200k disproves the guess (a window cannot hold more than its size),
// so it widens. Matched by substring: a dated id ("…-4-5-20251001") still hits.
func windowForModel(model string, tokens int) int {
	if strings.Contains(model, "haiku") && tokens <= smallWindow {
		return smallWindow
	}
	return defaultWindow
}

// usageTailBytes bounds how much of a transcript's end is scanned for the last
// assistant message. One JSONL line (a single message, tool results and all) can
// run to tens of KB, so this holds several — the read stays O(tail), not
// O(file), yet still reaches the last assistant line past a couple of large user
// turns. ponytail: a turn larger than this whole window makes the read miss and
// the readout keep its prior number — widen it then, nothing breaks.
const usageTailBytes = 512 * 1024

// contextUsage is one provider conversation's context-window occupancy.
type contextUsage struct {
	tokens  int
	percent int
	window  int
	model   string
	effort  string
}

// claudeContextUsage reads the context-window usage of a Claude conversation
// from its transcript. The transcript is ~/.claude/projects/<slug>/<id>.jsonl;
// the id is a globally-unique UUID, so a glob across every project slug finds it
// without reconstructing Claude's path-encoding of the slug. ok is false on any
// miss (not found, unreadable, no assistant usage in the tail) — each is a "keep
// the last value" for the caller, none worth logging once per turn.
func claudeContextUsage(providerSessionID string) (contextUsage, bool) {
	path, ok := claudeTranscriptPath(providerSessionID)
	if !ok {
		return contextUsage{}, false
	}
	tail, ok := readTail(path, usageTailBytes)
	if !ok {
		return contextUsage{}, false
	}
	return parseContextUsage(tail)
}

// claudeTranscriptPath locates a conversation's transcript by its UUID under the
// Claude config dir ($CLAUDE_CONFIG_DIR, else ~/.claude). The UUID is unique, so
// at most one file matches; false when none does yet.
func claudeTranscriptPath(providerSessionID string) (string, bool) {
	base := os.Getenv("CLAUDE_CONFIG_DIR")
	if base == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", false
		}
		base = filepath.Join(home, ".claude")
	}
	matches, err := filepath.Glob(filepath.Join(base, "projects", "*", providerSessionID+".jsonl"))
	if err != nil || len(matches) == 0 {
		return "", false
	}
	return matches[0], true
}

// readTail returns up to the last max bytes of a file. false when it can't be
// opened or stat'd — the transcript may not exist yet, or be mid-write.
func readTail(path string, max int64) ([]byte, bool) {
	f, err := os.Open(path)
	if err != nil {
		return nil, false
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return nil, false
	}
	start := int64(0)
	if info.Size() > max {
		start = info.Size() - max
	}
	buf := make([]byte, info.Size()-start)
	if _, err := f.ReadAt(buf, start); err != nil && err != io.EOF {
		return nil, false
	}
	return buf, true
}

// parseContextUsage pulls context-window occupancy from a transcript tail: the
// newest main-thread assistant line's token usage. The context side is input +
// cache-read + cache-creation (output is the reply, not context loaded); percent
// is that against the line's model window (see windowForModel), capped at 100. false when the tail holds
// no such line — a fresh conversation, or a tail of only user turns. Sidechain
// lines (a Task sub-agent's own conversation, written into the same transcript)
// are skipped: their context is the sub-agent's, not the window the user sees. A
// leading partial line (the tail was cut mid-line) fails to parse and is skipped
// like any malformed line.
func parseContextUsage(tail []byte) (contextUsage, bool) {
	lines := bytes.Split(tail, []byte("\n"))
	for i := len(lines) - 1; i >= 0; i-- {
		line := bytes.TrimSpace(lines[i])
		if len(line) == 0 {
			continue
		}
		var entry struct {
			Type        string `json:"type"`
			IsSidechain bool   `json:"isSidechain"`
			Effort      string `json:"effort"`
			Message     struct {
				Model string `json:"model"`
				Usage *struct {
					Input       int `json:"input_tokens"`
					CacheRead   int `json:"cache_read_input_tokens"`
					CacheCreate int `json:"cache_creation_input_tokens"`
				} `json:"usage"`
			} `json:"message"`
		}
		if err := json.Unmarshal(line, &entry); err != nil {
			continue
		}
		if entry.Type != "assistant" || entry.IsSidechain || entry.Message.Usage == nil {
			continue
		}
		u := entry.Message.Usage
		tokens := u.Input + u.CacheRead + u.CacheCreate
		window := windowForModel(entry.Message.Model, tokens)
		percent := min(tokens*100/window, 100)
		return contextUsage{
			tokens:  tokens,
			percent: percent,
			window:  window,
			model:   entry.Message.Model,
			effort:  entry.Effort,
		}, true
	}
	return contextUsage{}, false
}
