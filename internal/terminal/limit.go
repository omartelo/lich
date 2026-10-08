package terminal

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"slices"
	"time"

	"github.com/omartelo/lich/internal/providers"
)

// limitEventName carries a turn a plan's usage limit ended ({id, window,
// resetsAt}). Nothing clears it: the next state report the session makes does,
// on the window's side (session-limit-store.ts), because any turn starting is
// the proof the limit no longer holds.
const limitEventName = "session-limit"

// limitWatchTick is how often an open Codex turn is checked for having ended on
// a usage limit. Codex raises no hook when that happens (docs/hooks/session-state.md),
// so this read is the only way the card stops spinning; the scheduled prompt
// polls at the same pace, and nothing here needs to be finer than it.
const limitWatchTick = 30 * time.Second

// The windows a usage limit names, in the window's vocabulary rather than any
// provider's. Anything else a provider reports (a spend cap, a model-scoped
// window) reads as plainly "usage limit" on the card.
const (
	limitWindowSession = "session"
	limitWindowWeekly  = "weekly"
)

// usageLimit is a turn a provider ended because the plan's usage limit was hit:
// which window ran out, and when it resets in unix seconds (0 when the provider
// did not say).
type usageLimit struct {
	window   string
	resetsAt int64
}

// limitEvent is the payload of limitEventName.
type limitEvent struct {
	ID       string `json:"id"`
	Window   string `json:"window"`
	ResetsAt int64  `json:"resetsAt"`
}

// usageLimitFor reads whether a conversation's last turn ended on a usage limit.
// Only Claude Code and Codex record one together with its reset; every other
// provider retries on its own or records nothing a reader can trust
// (docs/ceilings.md), and reads as no limit.
func usageLimitFor(src usageSource) (usageLimit, bool) {
	switch src.kind {
	case providers.Claude:
		tail, ok := readTail(src.path, usageTailBytes)
		if !ok {
			return usageLimit{}, false
		}
		return parseClaudeUsageLimit(tail)
	case providers.Codex:
		return scanCodexUsageLimit(src.path)
	}
	return usageLimit{}, false
}

// claudeLimitWindows maps Claude Code's rateLimitType onto the card's windows.
var claudeLimitWindows = map[string]string{
	"five_hour":        limitWindowSession,
	"seven_day":        limitWindowWeekly,
	"seven_day_opus":   limitWindowWeekly,
	"seven_day_sonnet": limitWindowWeekly,
}

// parseClaudeUsageLimit reads the newest main-thread assistant line of a Claude
// transcript tail. When a limit ends a turn, Claude Code writes a synthetic
// assistant line with error "rate_limit" and the rejection it got, reset
// included, under quotaLimits (measured on 2.1.293). Any other newest line
// means the conversation went on past whatever limit came before it.
func parseClaudeUsageLimit(tail []byte) (usageLimit, bool) {
	for _, line := range slices.Backward(bytes.Split(tail, []byte("\n"))) {
		var entry struct {
			Type        string `json:"type"`
			IsSidechain bool   `json:"isSidechain"`
			Error       string `json:"error"`
			QuotaLimits *struct {
				ResetsAt      int64  `json:"resetsAt"`
				RateLimitType string `json:"rateLimitType"`
			} `json:"quotaLimits"`
		}
		if json.Unmarshal(bytes.TrimSpace(line), &entry) != nil {
			continue
		}
		if entry.Type != "assistant" || entry.IsSidechain {
			continue
		}
		if entry.Error != "rate_limit" || entry.QuotaLimits == nil {
			return usageLimit{}, false
		}
		return usageLimit{
			window:   claudeLimitWindows[entry.QuotaLimits.RateLimitType],
			resetsAt: entry.QuotaLimits.ResetsAt,
		}, true
	}
	return usageLimit{}, false
}

// codexRateWindow is one window of a rollout's rate_limits block.
type codexRateWindow struct {
	UsedPercent   float64 `json:"used_percent"`
	WindowMinutes int     `json:"window_minutes"`
	ResetsAt      int64   `json:"resets_at"`
}

// The window lengths Codex reports for its two plan windows.
const (
	codexSessionMinutes = 5 * 60
	codexWeeklyMinutes  = 7 * 24 * 60
)

// scanCodexUsageLimit reads whether a Codex rollout's last turn ended on a usage
// limit. Codex writes no error for it: the turn completes with no agent message,
// right after a token_count whose rate_limits show a window spent to 100% —
// measured on 0.144.5 against a server answering usage_limit_reached. A turn
// that completed with something said, or one still running, is no limit.
func scanCodexUsageLimit(path string) (usageLimit, bool) {
	var limit usageLimit
	completed, found := false, false
	codexReverseScan(path, func(line []byte) bool {
		var entry struct {
			Type    string `json:"type"`
			Payload struct {
				Type             string  `json:"type"`
				LastAgentMessage *string `json:"last_agent_message"`
				RateLimits       *struct {
					Primary   *codexRateWindow `json:"primary"`
					Secondary *codexRateWindow `json:"secondary"`
				} `json:"rate_limits"`
			} `json:"payload"`
		}
		if json.Unmarshal(line, &entry) != nil || entry.Type != "event_msg" {
			return false
		}
		switch entry.Payload.Type {
		case "task_complete":
			if completed {
				return true
			}
			completed = entry.Payload.LastAgentMessage == nil
			return !completed
		case "token_count":
			if !completed {
				return true
			}
			if entry.Payload.RateLimits == nil {
				return false
			}
			limit, found = spentCodexWindow(entry.Payload.RateLimits.Primary, entry.Payload.RateLimits.Secondary)
			return true
		case "task_started", "user_message":
			return true
		}
		return false
	})
	return limit, found
}

// spentCodexWindow is the window a token_count shows spent, the later reset
// when both are: the turn can only continue once the last of them lifts.
func spentCodexWindow(windows ...*codexRateWindow) (usageLimit, bool) {
	var limit usageLimit
	found := false
	for _, w := range windows {
		if w == nil || w.UsedPercent < 100 || w.ResetsAt <= limit.resetsAt {
			continue
		}
		limit, found = usageLimit{window: codexLimitWindow(w.WindowMinutes), resetsAt: w.ResetsAt}, true
	}
	return limit, found
}

func codexLimitWindow(minutes int) string {
	switch minutes {
	case codexSessionMinutes:
		return limitWindowSession
	case codexWeeklyMinutes:
		return limitWindowWeekly
	}
	return ""
}

// noteLimit reads whether session id's last turn ended on a usage limit and,
// when it did, tells the window and hands the reset to whoever parks the
// continuation (SetUsageLimit). Silent on every miss: no limit is the usual
// answer, and a transcript lich cannot read is one it has nothing to say about.
func (s *Service) noteLimit(id string) bool {
	src, ok := s.transcriptSource(id)
	if !ok {
		return false
	}
	limit, ok := usageLimitFor(src)
	if !ok {
		return false
	}
	s.hub.Emit(limitEventName, limitEvent{ID: id, Window: limit.window, ResetsAt: limit.resetsAt})
	if park := s.limitWatcher(); park != nil && limit.resetsAt > 0 {
		park(id, limit.resetsAt)
	}
	return true
}

// SetUsageLimit wires fn to every turn a usage limit ended that names its reset
// (relay.ParkResume). Only one watcher, like SetSessionState.
func (s *Service) SetUsageLimit(fn func(id string, resetsAt int64)) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.onLimit = fn
}

func (s *Service) limitWatcher() func(id string, resetsAt int64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.onLimit
}

// RunLimitWatch checks every open Codex turn for having ended on a usage limit,
// until the process ends. Started once at launch: with no Codex turn open, a
// pass costs one look at the turn log.
func (s *Service) RunLimitWatch() {
	ticker := time.NewTicker(limitWatchTick)
	defer ticker.Stop()
	for range ticker.C {
		s.watchCodexLimits()
	}
}

// watchCodexLimits ends each open Codex turn whose rollout shows it stopped on a
// usage limit. It ends it the way an interrupted one is ended — a limit is not a
// finished turn either, and the provider will never report the end itself.
func (s *Service) watchCodexLimits() {
	for _, id := range s.turns.openIDs() {
		src, ok := s.transcriptSource(id)
		if !ok || src.kind != providers.Codex {
			continue
		}
		if !s.noteLimit(id) {
			continue
		}
		slog.Info("terminal: codex turn ended on a usage limit", "session", id)
		s.noteInterrupt(id)
	}
}
