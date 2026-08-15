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
// context window the turn left occupied (0–100); Tokens is the raw input-side
// count behind it, Window the model's context window, Model the model id, and
// Effort the reasoning effort level ("" when the line records none).
//
// CostUSD is what the session has cost so far, and is omitted — not zeroed —
// whenever there is no number worth showing: the setting is off, or a model in
// the transcript has no known price. The readout is for API billing, so on a
// subscription the field never appears at all and the footer shows nothing.
type usageEvent struct {
	ID      string   `json:"id"`
	Percent int      `json:"percent"`
	Tokens  int      `json:"tokens"`
	Window  int      `json:"window"`
	Model   string   `json:"model"`
	Effort  string   `json:"effort"`
	CostUSD *float64 `json:"costUsd,omitempty"`
}

// emitUsage reads the context-window usage of the provider conversation running
// in session id and pushes it to the frontend. Called off the status hook on
// every non-idle state (see New), so it tracks the context as it grows through a
// turn, not only at the end. Silent on any miss — no provider id yet, no
// transcript, an unreadable or half-written file — so the readout keeps its last
// value instead of flickering.
//
// Only Claude reports a provider session id today (resume and the session-start
// hook are Claude-only), so the reader is Claude's. A second provider that grows
// one selects its own reader here by the session's kind — not an interface until
// there are two to hide behind it.
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
	u, ok := claudeContextUsage(providerSessionID)
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
	if cost, ok := s.sessionCost(id, providerSessionID); ok {
		event.CostUSD = &cost
	}
	return event, true
}

// sessionCost is what session id has cost across every conversation it has run,
// or ok=false for "no number to show". That covers the flag being off (nothing
// is even read then), a transcript that cannot be reached, and a model no price
// table knows — see scanTranscriptCost for why an unpriced line stops the count
// instead of being skipped.
//
// A conversation is more than one file: what its sub-agents spent is billed to
// the same account and written to transcripts of their own, so each is counted
// into the same session. One of them falling short withholds the whole number,
// for the same reason a single unpriced line does.
//
// The accounting is persisted per transcript, so it survives both a `/clear`
// (a new conversation under the same session, counted into its own row) and a
// restart of lich itself.
func (s *Service) sessionCost(id, providerSessionID string) (float64, bool) {
	if s.prices == nil || !s.store.CostReadout() {
		return 0, false
	}
	path, ok := claudeTranscriptPath(providerSessionID)
	if !ok {
		return 0, false
	}
	if !s.countTranscript(id, providerSessionID, path) {
		return 0, false
	}
	for _, sub := range claudeSubagentPaths(providerSessionID) {
		if !s.countTranscript(id, providerSessionID+"/"+filepath.Base(sub), sub) {
			return 0, false
		}
	}
	total, err := s.store.SessionCost(id)
	if err != nil {
		slog.Warn("terminal: read session cost", "session", id, "err", err)
		return 0, false
	}
	return total, true
}

// countTranscript folds one transcript into the session's ledger, resuming from
// where the last turn stopped and saving how far this one got. false is "this
// file has no total to contribute yet" — it could not be read, its accounting
// could not be stored, or the scan stopped at a line it cannot price.
//
// transcriptID is what the ledger row is keyed by, which is the provider's
// conversation id for the conversation itself and that id plus the file name for
// each sub-agent beside it: one row per file, so a re-read resumes per file.
func (s *Service) countTranscript(id, transcriptID, path string) bool {
	offset, lastMessage, cost, err := s.store.CostLedger(id, transcriptID)
	if err != nil {
		slog.Warn("terminal: read cost ledger", "session", id, "err", err)
		return false
	}
	from := costLedger{offset: offset, lastMessage: lastMessage, cost: cost}
	ledger, complete, ok := scanTranscriptCost(path, from, s.prices)
	if !ok {
		return false
	}
	if ledger != from {
		if err := s.store.SaveCostLedger(
			id, transcriptID, ledger.offset, ledger.lastMessage, ledger.cost,
		); err != nil {
			slog.Warn("terminal: save cost ledger", "session", id, "err", err)
			return false
		}
	}
	return complete
}
