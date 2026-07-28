package terminal

import "log/slog"

// usageEventName carries a session's context-window usage ({id, percent, tokens}),
// emitted after a turn ends. Global like the other session events: the card that
// shows it is only mounted while its project is active, so a per-session name
// could not reach it.
const usageEventName = "session-usage"

// usageEvent is the payload of usageEventName. Percent is the share of the
// context window the turn left occupied (0–100); Tokens is the raw input-side
// count behind it, Window the model's context window, Model the model id, and
// Effort the reasoning effort level ("" when the line records none).
type usageEvent struct {
	ID      string `json:"id"`
	Percent int    `json:"percent"`
	Tokens  int    `json:"tokens"`
	Window  int    `json:"window"`
	Model   string `json:"model"`
	Effort  string `json:"effort"`
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
	return usageEvent{
		ID:      id,
		Percent: u.percent,
		Tokens:  u.tokens,
		Window:  u.window,
		Model:   u.model,
		Effort:  u.effort,
	}, true
}
