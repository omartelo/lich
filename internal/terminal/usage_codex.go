package terminal

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
)

// A rollout line can be much larger than a normal event. Reading 64 KiB at a
// time keeps allocations bounded without turning a long transcript into many
// tiny reads; lines spanning blocks are carried into the next iteration.
const codexScanBlockBytes int64 = 64 * 1024

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
			Window int `json:"model_context_window"`
		} `json:"info"`
	} `json:"payload"`
}

type codexModelsCache struct {
	Models []struct {
		Slug                          string `json:"slug"`
		ContextWindow                 int    `json:"context_window"`
		MaxContextWindow              int    `json:"max_context_window"`
		EffectiveContextWindowPercent int    `json:"effective_context_window_percent"`
	} `json:"models"`
}

// codexContextUsage reads the latest active context size, model and effort from
// a Codex rollout. Codex writes the current context tranche into the rollout,
// while its own readout uses the model cache's maximum effective window; prefer
// that maximum and keep the rollout value as the fallback. The reverse scan
// stops at the current turn's turn_context, so an old conversation costs no
// more to read than a new one.
func codexContextUsage(providerSessionID string) (contextUsage, bool) {
	path, ok := codexTranscriptPath(providerSessionID)
	if !ok {
		return contextUsage{}, false
	}
	usage, ok := scanCodexContextUsage(path)
	if !ok {
		return contextUsage{}, false
	}
	if window, ok := codexMaxContextWindow(usage.model); ok {
		usage.window = window
		usage.percent = min(usage.tokens*100/window, 100)
	}
	return usage, true
}

func codexMaxContextWindow(model string) (int, bool) {
	base, ok := harnessDir("CODEX_HOME", ".codex")
	if !ok {
		return 0, false
	}
	data, err := os.ReadFile(filepath.Join(base, "models_cache.json"))
	if err != nil {
		return 0, false
	}
	var cache codexModelsCache
	if json.Unmarshal(data, &cache) != nil {
		return 0, false
	}
	for _, candidate := range cache.Models {
		if candidate.Slug != model {
			continue
		}
		window := max(candidate.ContextWindow, candidate.MaxContextWindow)
		percent := candidate.EffectiveContextWindowPercent
		if window <= 0 || percent <= 0 || percent > 100 {
			return 0, false
		}
		return window * percent / 100, true
	}
	return 0, false
}

func scanCodexContextUsage(path string) (contextUsage, bool) {
	f, err := os.Open(path)
	if err != nil {
		return contextUsage{}, false
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return contextUsage{}, false
	}
	var usage contextUsage
	var suffix []byte
	haveTokens := false
	for end := info.Size(); end > 0; {
		start := max(int64(0), end-codexScanBlockBytes)
		chunk := make([]byte, end-start)
		if _, err := f.ReadAt(chunk, start); err != nil && err != io.EOF {
			return contextUsage{}, false
		}
		lines := bytes.Split(append(chunk, suffix...), []byte("\n"))
		first := 0
		if start > 0 {
			first = 1
		}
		for i := len(lines) - 1; i >= first; i-- {
			if consumeCodexUsageLine(lines[i], &usage, &haveTokens) {
				return usage, true
			}
		}
		suffix = append(suffix[:0], lines[0]...)
		end = start
	}
	return contextUsage{}, false
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
