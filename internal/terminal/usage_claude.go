package terminal

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"slices"
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
// turns.
//
// Ceiling: a turn larger than this whole window makes the read miss and the
// readout keep its prior number — widen it then, nothing breaks.
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
// from its transcript, which usageSourceFor already located. ok is false on any
// miss (unreadable, no assistant usage in the tail) — each is a "keep the last
// value" for the caller, none worth logging once per turn.
func claudeContextUsage(path string) (contextUsage, bool) {
	tail, ok := readTail(path, usageTailBytes)
	if !ok {
		return contextUsage{}, false
	}
	return parseContextUsage(tail)
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
// is that against the line's model window (see windowForModel), capped at 100.
// false when the tail holds no such line — a fresh conversation, or a tail of
// only user turns. Sidechain
// lines (a Task sub-agent's own conversation, written into the same transcript)
// are skipped: their context is the sub-agent's, not the window the user sees. A
// leading partial line (the tail was cut mid-line) fails to parse and is skipped
// like any malformed line.
//
// A compaction writes no assistant line, only a compact_boundary carrying the
// post-compaction count. Meeting one first means it is newer than the last
// assistant line, whose count is now stale, so the tokens come from the boundary
// — while the scan keeps going for the model, which the boundary does not
// record and only an assistant line can name. Both triggers, "manual" and
// "auto", write the same shape.
func parseContextUsage(tail []byte) (contextUsage, bool) {
	lines := bytes.Split(tail, []byte("\n"))
	compacted := -1
	for _, line := range slices.Backward(lines) {
		line := bytes.TrimSpace(line)
		if len(line) == 0 {
			continue
		}
		var entry struct {
			Type            string `json:"type"`
			Subtype         string `json:"subtype"`
			IsSidechain     bool   `json:"isSidechain"`
			Effort          string `json:"effort"`
			CompactMetadata *struct {
				PostTokens int `json:"postTokens"`
			} `json:"compactMetadata"`
			Message struct {
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
		if compacted < 0 && entry.Type == "system" && entry.Subtype == "compact_boundary" &&
			!entry.IsSidechain && entry.CompactMetadata != nil {
			compacted = entry.CompactMetadata.PostTokens
			continue
		}
		if entry.Type != "assistant" || entry.IsSidechain || entry.Message.Usage == nil {
			continue
		}
		u := entry.Message.Usage
		tokens := u.Input + u.CacheRead + u.CacheCreate
		if compacted >= 0 {
			tokens = compacted
		}
		return usageFor(tokens, entry.Message.Model, entry.Effort), true
	}
	if compacted >= 0 {
		// A compaction with no assistant line left in the tail to name the
		// model: the default window is the same guess windowForModel makes for
		// a model it has never heard of.
		return usageFor(compacted, "", ""), true
	}
	return contextUsage{}, false
}

// usageFor resolves the window and percent a token count reads as on a model.
func usageFor(tokens int, model, effort string) contextUsage {
	window := windowForModel(model, tokens)
	return contextUsage{
		tokens:  tokens,
		percent: min(tokens*100/window, 100),
		window:  window,
		model:   model,
		effort:  effort,
	}
}
