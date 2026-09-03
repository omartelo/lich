package terminal

import (
	"log/slog"
	"path/filepath"
)

// usageEventName carries a session's context-window usage ({id, percent, tokens}),
// emitted after a turn ends. Global like the other session events: the card that
// shows it is only mounted while its project is active, so a per-session name
// could not reach it.
const usageEventName = "session-usage"

// usageEvent is the payload of usageEventName. Percent is the share of the
// context window the turn left occupied (0–100); Tokens is the provider's active
// context count behind it, Window the model's effective context window, Model
// the model id, and Effort the reasoning effort level ("" when the line records
// none).
//
// CostUSD is what the session has cost so far, and is omitted — not zeroed —
// whenever there is no number worth showing: the setting is off, or a model in
// the transcript has no known price. The readout is for API billing, so on a
// subscription the field never appears at all and the footer shows nothing.
//
// CostMiss says why, and only when the answer is "lich cannot price this
// conversation" — a costMiss.spoken() one. It stays empty for the readout being
// off, which is not a failure but the design above, and for a miss the next turn
// heals. Without it a session that cannot be priced and a session that is not
// being priced look identical, and an empty footer reads as zero spend.
type usageEvent struct {
	ID       string   `json:"id"`
	Percent  int      `json:"percent"`
	Tokens   int      `json:"tokens"`
	Window   int      `json:"window"`
	Model    string   `json:"model"`
	Effort   string   `json:"effort"`
	CostUSD  *float64 `json:"costUsd,omitempty"`
	CostMiss string   `json:"costMiss,omitempty"`
}

// emitUsage reads the context-window usage of the provider conversation running
// in session id and pushes it to the frontend. Called off the status hook on
// every non-idle state (see New), so it tracks the context as it grows through a
// turn, not only at the end. Silent on any miss — no provider id yet, no
// transcript, an unreadable or half-written file — so the readout keeps its last
// value instead of flickering.
func (s *Service) emitUsage(id string) {
	if event, ok := s.sessionUsage(id); ok {
		s.hub.Emit(usageEventName, event)
	}
}

// sessionUsage resolves the event emitUsage would push for session id, or
// ok=false for every miss the doc above lists. Split from the emit so the
// resolution is testable without a connected window — pollCwd's pattern.
func (s *Service) sessionUsage(id string) (usageEvent, bool) {
	providerSessionID, err := s.store.ProviderSession(id)
	if err != nil {
		slog.Warn("terminal: read provider session", "session", id, "err", err)
		return usageEvent{}, false
	}
	if providerSessionID == "" {
		return usageEvent{}, false
	}
	u, ok := contextUsageFor(providerSessionID)
	if !ok {
		return usageEvent{}, false
	}
	event := usageEvent{
		ID:      id,
		Percent: u.percent,
		Tokens:  u.tokens,
		Window:  u.window,
		Model:   u.model,
		Effort:  u.effort,
	}
	if cost, miss, ok := s.sessionCost(id, providerSessionID); ok {
		event.CostUSD = &cost
	} else if miss.spoken() {
		event.CostMiss = string(miss)
	}
	return event, true
}

func contextUsageFor(providerSessionID string) (contextUsage, bool) {
	if usage, ok := claudeContextUsage(providerSessionID); ok {
		return usage, true
	}
	return codexContextUsage(providerSessionID)
}

// sessionCost is what session id has cost across every conversation it has run,
// or ok=false for "no number to show", with the costMiss saying which kind of
// absence it is. That covers the flag being off (nothing is even read then), a
// transcript that cannot be reached, and a model no price table knows — see
// scanTranscriptCost for why an unpriced line stops the count instead of being
// skipped.
//
// Routed by provider the way contextUsageFor is: a Claude transcript is more
// than one file — what its sub-agents spent is billed to the same account and
// written to transcripts of their own, so each is counted into the same
// session, and one of them falling short withholds the whole number, for the
// same reason a single unpriced line does. A Codex rollout has no sub-agent
// files and reports its own running total, so countCodexTranscript prices it
// whole rather than walking a ledger.
//
// The accounting is persisted per transcript, so it survives both a `/clear`
// (a new conversation under the same session, counted into its own row) and a
// restart of lich itself.
func (s *Service) sessionCost(id, providerSessionID string) (float64, costMiss, bool) {
	if s.prices == nil || !s.store.CostReadout() {
		return 0, costMissNone, false
	}
	if path, ok := claudeTranscriptPath(providerSessionID); ok {
		if miss, ok := s.countTranscript(id, providerSessionID, path); !ok {
			return 0, miss, false
		}
		for _, sub := range claudeSubagentPaths(providerSessionID) {
			miss, ok := s.countTranscript(id, providerSessionID+"/"+filepath.Base(sub), sub)
			if !ok {
				return 0, miss, false
			}
		}
	} else if path, ok := codexTranscriptPath(providerSessionID); ok {
		if miss, ok := s.countCodexTranscript(id, providerSessionID, path); !ok {
			return 0, miss, false
		}
	} else {
		return 0, costMissNone, false
	}
	total, err := s.store.SessionCost(id)
	if err != nil {
		slog.Warn("terminal: read session cost", "session", id, "err", err)
		return 0, costMissUnread, false
	}
	return total, costMissNone, true
}

// countCodexTranscript folds one Codex rollout's cost into the session's
// ledger. Unlike countTranscript's per-turn deltas, total_token_usage is
// already the conversation's running total, so there is no offset to resume
// from: each call prices the rollout whole and overwrites the stored row.
func (s *Service) countCodexTranscript(id, transcriptID, path string) (costMiss, bool) {
	cost, miss, ok := codexTranscriptCost(path, s.prices)
	if !ok {
		return miss, false
	}
	if err := s.store.SaveCostLedger(id, transcriptID, 0, "", cost); err != nil {
		slog.Warn("terminal: save cost ledger", "session", id, "err", err)
		return costMissUnread, false
	}
	return costMissNone, true
}

// countTranscript folds one transcript into the session's ledger, resuming from
// where the last turn stopped and saving how far this one got. false is "this
// file has no total to contribute yet" — it could not be read, its accounting
// could not be stored, or the scan stopped at a line it cannot price — and the
// costMiss beside it is which of those, for the footer to say the last one.
//
// transcriptID is what the ledger row is keyed by, which is the provider's
// conversation id for the conversation itself and that id plus the file name for
// each sub-agent beside it: one row per file, so a re-read resumes per file.
func (s *Service) countTranscript(id, transcriptID, path string) (costMiss, bool) {
	offset, lastMessage, cost, err := s.store.CostLedger(id, transcriptID)
	if err != nil {
		slog.Warn("terminal: read cost ledger", "session", id, "err", err)
		return costMissUnread, false
	}
	from := costLedger{offset: offset, lastMessage: lastMessage, cost: cost}
	ledger, miss, ok := scanTranscriptCost(path, from, s.prices)
	if !ok {
		return miss, false
	}
	if ledger != from {
		if err := s.store.SaveCostLedger(
			id, transcriptID, ledger.offset, ledger.lastMessage, ledger.cost,
		); err != nil {
			slog.Warn("terminal: save cost ledger", "session", id, "err", err)
			return costMissUnread, false
		}
	}
	return miss, miss == costMissNone
}
