package terminal

import (
	"bytes"
	"encoding/json"
	"io"
	"os"

	"github.com/omartelo/lich/internal/pricing"
)

// A rollout line can be much larger than a normal event. Reading 64 KiB at a
// time keeps allocations bounded without turning a long transcript into many
// tiny reads; lines spanning blocks are carried into the next iteration.
const codexScanBlockBytes int64 = 64 * 1024

// codexTokenUsage is one info block's total_token_usage: the running total
// for the whole conversation, unlike a Claude transcript's per-turn counters.
// cached_input_tokens is a subset of Input and reasoning_output_tokens a
// subset of Output, so only the four fields a rate needs are kept — the other
// two would double-count if added in.
type codexTokenUsage struct {
	Input      int `json:"input_tokens"`
	Cached     int `json:"cached_input_tokens"`
	CacheWrite int `json:"cache_write_input_tokens"`
	Output     int `json:"output_tokens"`
}

type codexRolloutEntry struct {
	Type    string `json:"type"`
	Payload struct {
		Type   string `json:"type"`
		Model  string `json:"model"`
		Effort string `json:"effort"`
		Info   *struct {
			Last *struct {
				Total int `json:"total_tokens"`
			} `json:"last_token_usage"`
			Total  *codexTokenUsage `json:"total_token_usage"`
			Window int              `json:"model_context_window"`
		} `json:"info"`
	} `json:"payload"`
}

// codexContextUsage reads the latest active context size, model and effort from
// a Codex rollout. The rollout carries the effective window selected for that
// session, including a model_context_window override. The reverse scan stops at
// the current turn's turn_context, so an old conversation costs no more to read
// than a new one.
func codexContextUsage(providerSessionID string) (contextUsage, bool) {
	path, ok := codexTranscriptPath(providerSessionID)
	if !ok {
		return contextUsage{}, false
	}
	return scanCodexContextUsage(path)
}

func scanCodexContextUsage(path string) (contextUsage, bool) {
	var usage contextUsage
	haveTokens := false
	if !codexReverseScan(path, func(line []byte) bool {
		return consumeCodexUsageLine(line, &usage, &haveTokens)
	}) {
		return contextUsage{}, false
	}
	return usage, true
}

// codexReverseScan walks path from the end in codexScanBlockBytes chunks,
// calling consume on each complete line, latest first, until consume reports
// it found what it needs. false covers both an unreadable file and reaching
// the start with consume never satisfied — the same "nothing to report" a
// caller treats either way.
func codexReverseScan(path string, consume func(line []byte) (done bool)) bool {
	f, err := os.Open(path)
	if err != nil {
		return false
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return false
	}
	var suffix []byte
	for end := info.Size(); end > 0; {
		start := max(int64(0), end-codexScanBlockBytes)
		chunk := make([]byte, end-start)
		if _, err := f.ReadAt(chunk, start); err != nil && err != io.EOF {
			return false
		}
		lines := bytes.Split(append(chunk, suffix...), []byte("\n"))
		first := 0
		if start > 0 {
			first = 1
		}
		for i := len(lines) - 1; i >= first; i-- {
			if consume(lines[i]) {
				return true
			}
		}
		suffix = append(suffix[:0], lines[0]...)
		end = start
	}
	return false
}

func consumeCodexUsageLine(line []byte, usage *contextUsage, haveTokens *bool) bool {
	var entry codexRolloutEntry
	if json.Unmarshal(line, &entry) != nil {
		return false
	}
	if !*haveTokens {
		if entry.Type != "event_msg" || entry.Payload.Type != "token_count" ||
			entry.Payload.Info == nil || entry.Payload.Info.Last == nil ||
			entry.Payload.Info.Last.Total < 0 || entry.Payload.Info.Window <= 0 {
			return false
		}
		usage.tokens = entry.Payload.Info.Last.Total
		usage.window = entry.Payload.Info.Window
		usage.percent = min(usage.tokens*100/usage.window, 100)
		*haveTokens = true
		return false
	}
	if entry.Type != "turn_context" || entry.Payload.Model == "" {
		return false
	}
	usage.model = entry.Payload.Model
	usage.effort = entry.Payload.Effort
	return true
}

// codexTranscriptCost prices a Codex rollout's running total in USD. Unlike a
// Claude transcript's per-turn deltas, total_token_usage is already the
// conversation's cumulative cost, so the last line is the whole answer — no
// ledger walk, no offset to resume from. ok is false when the rollout carries
// no cost line yet, or when rates cannot price the model that ran it.
func codexTranscriptCost(path string, rates rateSource) (float64, bool) {
	var tokens pricing.Tokens
	var model string
	haveTokens := false
	if !codexReverseScan(path, func(line []byte) bool {
		return consumeCodexCostLine(line, &tokens, &model, &haveTokens)
	}) {
		return 0, false
	}
	rate, ok := rates.Rate(model)
	if !ok {
		return 0, false
	}
	return rate.Cost(tokens)
}

// consumeCodexCostLine mirrors consumeCodexUsageLine's two-phase reverse
// walk, gathering the running token total and the model id that priced it
// instead of the window and effort the context readout needs.
func consumeCodexCostLine(line []byte, tokens *pricing.Tokens, model *string, haveTokens *bool) bool {
	var entry codexRolloutEntry
	if json.Unmarshal(line, &entry) != nil {
		return false
	}
	if !*haveTokens {
		if entry.Type != "event_msg" || entry.Payload.Type != "token_count" ||
			entry.Payload.Info == nil || entry.Payload.Info.Total == nil {
			return false
		}
		t := entry.Payload.Info.Total
		*tokens = pricing.Tokens{
			Input:      t.Input - t.Cached,
			CacheRead:  t.Cached,
			CacheWrite: t.CacheWrite,
			Output:     t.Output,
		}
		*haveTokens = true
		return false
	}
	if entry.Type != "turn_context" || entry.Payload.Model == "" {
		return false
	}
	*model = entry.Payload.Model
	return true
}
