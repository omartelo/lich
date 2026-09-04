package terminal

import (
	"log/slog"
	"path/filepath"

	"github.com/omartelo/lich/internal/providers"
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
// All five are zero for a provider that records what a turn spent but not the
// window it spent it against — oh-my-pi, opencode and Crush, the cost-only rung
// of docs/ceilings.md. A zero Window is the frontend's signal to draw the cost
// alone rather than a ring around a window nobody reported.
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
	src, ok := usageSourceFor(providerSessionID, s.spawnOf(id).cwd)
	if !ok {
		return usageEvent{}, false
	}
	u, ok, supported := contextUsageFor(src)
	// A provider whose window lich can read but has not read yet — a transcript
	// still being written, a conversation before its first assistant line — keeps
	// the readout's last value rather than repainting it at zero. A provider with
	// no window to read never had one to lose, so its cost goes out alone.
	if !ok && supported {
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
	cost, miss, priced := s.sessionCost(id, src)
	switch {
	case priced:
		event.CostUSD = &cost
	case miss.spoken():
		event.CostMiss = string(miss)
	case !supported:
		// Nothing to report at all: no window, and no cost either — the readout is
		// off, or the conversation has not been priced yet. Emitting the zeroes
		// would blank a figure the last turn earned.
		return usageEvent{}, false
	}
	return event, true
}

// usageSource is where one conversation's records live: which provider wrote
// them and the one file that holds them — a transcript for Claude Code, Codex
// and oh-my-pi, a database for opencode and Crush.
type usageSource struct {
	kind string
	path string
	// id is the provider's own conversation id, which is what keys the ledger
	// rows and, for the two database providers, the row to read.
	id string
}

// usageSourceFor finds the store holding a conversation, from the id alone.
// Every provider files under a globally unique conversation id, so the store
// that has it is the store that ran it — which is why this asks the disk rather
// than the session's kind: a provider CLI started by hand inside a shell session
// reports an id lich never spawned a kind for, and its footer reads the same as
// any other.
//
// cwd is the session's spawn directory, needed by Crush alone: it keeps one
// database per checkout rather than one per machine, so without a directory
// there is no database to ask.
func usageSourceFor(providerSessionID, cwd string) (usageSource, bool) {
	if path, ok := claudeTranscriptPath(providerSessionID); ok {
		return usageSource{kind: providers.Claude, path: path, id: providerSessionID}, true
	}
	if path, ok := codexTranscriptPath(providerSessionID); ok {
		return usageSource{kind: providers.Codex, path: path, id: providerSessionID}, true
	}
	if path, ok := ompTranscriptPath(providerSessionID); ok {
		return usageSource{kind: providers.OMP, path: path, id: providerSessionID}, true
	}
	if path, ok := opencodeSessionDB(); ok &&
		sessionRowExists(path, opencodeSessionTable, providerSessionID) {
		return usageSource{kind: providers.OpenCode, path: path, id: providerSessionID}, true
	}
	if path, ok := crushSessionDB(cwd); ok &&
		sessionRowExists(path, crushSessionTable, providerSessionID) {
		return usageSource{kind: providers.Crush, path: path, id: providerSessionID}, true
	}
	if path, ok := kiroSessionPath(providerSessionID); ok {
		return usageSource{kind: providers.Kiro, path: path, id: providerSessionID}, true
	}
	return usageSource{}, false
}

// contextUsageFor reads how much of the model's context window a conversation
// occupies. supported is whether this provider records a window at all: only
// Claude Code, Codex and Kiro CLI do, and for the rest a miss is the standing
// state rather than a read that has not landed yet (docs/ceilings.md).
// Antigravity and Cursor CLI never reach here — they file no transcript
// usageSourceFor finds.
//
// Kiro is the odd one of the three: it records the percentage and the window but
// no token count, so its arm derives the tokens rather than reading them
// (usage_kiro.go).
func contextUsageFor(src usageSource) (usage contextUsage, ok, supported bool) {
	switch src.kind {
	case providers.Claude:
		u, ok := claudeContextUsage(src.path)
		return u, ok, true
	case providers.Codex:
		u, ok := scanCodexContextUsage(src.path)
		return u, ok, true
	case providers.Kiro:
		u, ok := kiroContextUsage(src.path)
		return u, ok, true
	}
	return contextUsage{}, false, false
}

// sessionCost is what session id has cost across every conversation it has run,
// or ok=false for "no number to show", with the costMiss saying which kind of
// absence it is. That covers the flag being off (nothing is even read then), a
// transcript that cannot be reached, and a model no price table knows — see
// scanTranscriptCost for why an unpriced line stops the count instead of being
// skipped.
//
// Routed by provider the way contextUsageFor is, and each rung reaches its total
// differently:
//
//   - A Claude transcript is more than one file — what its sub-agents spent is
//     billed to the same account and written to transcripts of their own, so each
//     is counted into the same session, and one of them falling short withholds
//     the whole number, for the same reason a single unpriced line does. Only
//     this one resumes from an offset; the rest are read whole every turn.
//   - A Codex rollout reports its own running token total, which lich prices
//     from the shipped table.
//   - oh-my-pi, opencode and Crush price their own turns as they write them, and
//     the figure they recorded is the one lich shows: they bill models no table
//     here knows, and re-pricing what a provider already billed would put a
//     second, disagreeing number on screen. Whose arithmetic each is, and what
//     it leaves out, is in the reader beside each call.
//
// The accounting is persisted per transcript, so it survives both a `/clear`
// (a new conversation under the same session, counted into its own row) and a
// restart of lich itself.
//
// A nil price table gates every rung, the three that never consult one
// included: it is the readout's kill switch (see Service.prices), not a rate
// lookup, and half a readout is not a state worth having.
func (s *Service) sessionCost(id string, src usageSource) (float64, costMiss, bool) {
	if s.prices == nil || !s.store.CostReadout() {
		return 0, costMissNone, false
	}
	switch src.kind {
	case providers.Claude:
		if miss, ok := s.countTranscript(id, src.id, src.path); !ok {
			return 0, miss, false
		}
		for _, sub := range claudeSubagentPaths(src.id) {
			miss, ok := s.countTranscript(id, src.id+"/"+filepath.Base(sub), sub)
			if !ok {
				return 0, miss, false
			}
		}
	case providers.Codex:
		cost, miss, ok := codexTranscriptCost(src.path, s.prices)
		if !ok {
			return 0, miss, false
		}
		if miss, ok := s.saveWholeCost(id, src.id, cost); !ok {
			return 0, miss, false
		}
	case providers.OMP:
		cost, miss, ok := ompTranscriptCost(src.path)
		if !ok {
			return 0, miss, false
		}
		if miss, ok := s.saveWholeCost(id, src.id, cost); !ok {
			return 0, miss, false
		}
	case providers.OpenCode, providers.Crush:
		cost, ok := sessionDBCost(src.path, costQueryFor(src.kind), src.id)
		if !ok {
			return 0, costMissUnread, false
		}
		if miss, ok := s.saveWholeCost(id, src.id, cost); !ok {
			return 0, miss, false
		}
	default:
		return 0, costMissNone, false
	}
	total, err := s.store.SessionCost(id)
	if err != nil {
		slog.Warn("terminal: read session cost", "session", id, "err", err)
		return 0, costMissUnread, false
	}
	return total, costMissNone, true
}

// saveWholeCost files what one conversation has cost so far, for every provider
// that reports a running total rather than per-turn deltas. Unlike
// countTranscript there is no offset to resume from: each call prices the
// conversation whole and overwrites the stored row, so a re-read costs a scan
// and never a double count.
func (s *Service) saveWholeCost(id, transcriptID string, cost float64) (costMiss, bool) {
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
