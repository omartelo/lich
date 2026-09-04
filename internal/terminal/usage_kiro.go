package terminal

import (
	"encoding/json"
	"math"
	"os"
)

// The Kiro CLI side of the footer readout. Kiro is the one provider that records
// its context usage as a percentage rather than a token count: every request
// files the share of the window it left occupied, and the window itself sits
// beside it under the model. Both are read off the session metadata file
// transcript.go resolved (kiroSessionPath) — the `.json`, not the `.jsonl` of
// turns beside it.
//
// It records no cost lich can show. Its spend is metered in *credits*
// (`metering_usage`, `"unit": "credit"`, measured on 2.21.0), whose price
// depends on the plan the account is on, and lich's readout is dollars. So Kiro
// takes no arm in sessionCost and falls to its default — the context ring alone,
// which is the rung docs/ceilings.md puts it on.

// kiroSessionState is the slice of the session file the readout needs. Only
// these fields are decoded; the turn-by-turn metadata beside them is the bulk of
// the file and none of it is read.
type kiroSessionState struct {
	SessionState struct {
		ModelState struct {
			ModelInfo *struct {
				ModelID string `json:"model_id"`
				Window  int    `json:"context_window_tokens"`
			} `json:"model_info"`
			// Percent of the window occupied, 0–100 — a session Kiro's own TUI
			// drew as "1%" carries 0.74200004 here (measured on 2.21.0). Null
			// until the conversation's first request answers.
			Percent *float64 `json:"context_usage_percentage"`
		} `json:"rts_model_state"`
	} `json:"session_state"`
}

// kiroContextUsage reads how much of the window a Kiro conversation occupies.
// ok is false while the file holds no answer yet — a session opened and not yet
// asked anything writes both fields as null — which the caller reads as "keep
// the last value" rather than repainting the ring at zero.
func kiroContextUsage(path string) (contextUsage, bool) {
	data, err := os.ReadFile(path)
	if err != nil {
		return contextUsage{}, false
	}
	var state kiroSessionState
	if err := json.Unmarshal(data, &state); err != nil {
		return contextUsage{}, false
	}
	model := state.SessionState.ModelState
	if model.ModelInfo == nil || model.Percent == nil || model.ModelInfo.Window <= 0 {
		return contextUsage{}, false
	}
	percent := *model.Percent
	if percent < 0 {
		return contextUsage{}, false
	}
	return contextUsage{
		// Derived, because Kiro files no token count to read: its own
		// `input_token_count` and the three beside it are zero on a turn that
		// really spent tokens (measured on 2.21.0), so the percentage is the
		// only thing it actually answers. Deriving keeps the tooltip's
		// "n / window tokens" agreeing with the ring above it instead of
		// reading "0 / 200,000" next to a ring at 1%.
		tokens:  int(math.Round(percent / 100 * float64(model.ModelInfo.Window))),
		percent: int(math.Round(percent)),
		window:  model.ModelInfo.Window,
		model:   model.ModelInfo.ModelID,
	}, true
}
