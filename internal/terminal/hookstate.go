package terminal

import "sync"

// turnLog is the memory behind the one distinction the session-state contract
// cannot make on its own (docs/hooks/session-state.md): Claude Code raises the
// same `Notification` — reported as `waiting` — for a permission prompt and for
// a session that has simply been sitting at its own prompt with nothing to do.
// The card draws `waiting` as "Waiting on you" and the window toasts it, so a
// session nobody was blocked on wore the badge that says a human is.
//
// A permission decision only ever happens inside a turn, so the report before it
// tells the two apart: after `busy` a turn is open and something in it is
// blocked on a human; after `done`, or after nothing at all, there is no turn to
// block and the notification is the provider's "your turn" nudge. It is the same
// reading internal/relay makes of the pair (see relay.Observe), from its own
// record: that one answers for which turn an errand belongs to, this one for
// what the window is told.
//
// The zero value is ready to use.
type turnLog struct {
	mu   sync.Mutex
	open map[string]bool
}

// report takes one session-state report and answers whether the window should
// hear it. Only an idle-prompt `waiting` is held back, and it is dropped rather
// than published under a state of its own: the card then keeps whatever it was
// already showing — the finished turn, an inbox count, nothing — which is what a
// session nobody is blocked on should show.
func (l *turnLog) report(id, state string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	switch state {
	case statusWaiting:
		// Recorded neither way: `waiting` interrupts a turn rather than replacing
		// it, so the report after it still has to be read against the turn it
		// interrupted. Two permission prompts in one turn are two blocks.
		return l.open[id]
	case statusBusy:
		if l.open == nil {
			l.open = make(map[string]bool)
		}
		l.open[id] = true
	default:
		// `done` ends the turn; `idle` (SessionEnd) ends the session, and a row
		// per dead session would grow for the life of the process. Absent reads
		// as "no turn open", which is what both of them mean here.
		delete(l.open, id)
	}
	return true
}

// forget drops a session's turn, so a PTY respawned under the same id is read
// against its own reports rather than against those of the provider that left.
func (l *turnLog) forget(id string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	delete(l.open, id)
}
