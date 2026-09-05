package relay

import ()

// Observe takes one session-state report from the hooks the provider already
// runs (docs/hooks/session-state.md). It exists for one case: a target that
// works through a relayed request and then answers somewhere lich cannot read —
// its provider's own peer channel, or simply out loud to the person watching.
// Nothing would ever close that ticket, and the sender would sit out the full
// wait learning nothing.
//
// A turn ending is only this request's turn when the target has been working
// since the request arrived. Every provider queues typed input, so a message
// handed to a busy session is answered a turn later; the ending of the turn
// already in progress is skipped rather than mistaken for an answer that never
// came. Getting that backwards would report "answered elsewhere" about a
// request the target had not read yet, which is worse than saying nothing.
func (s *Service) Observe(sessionID, state string) {
	s.mu.Lock()
	s.recordState(sessionID, state)
	ended := s.endedErrands(sessionID, state)
	// A waiter still holding the line carries the news out through its own
	// select; an errand nobody is attending is stashed for the sender instead,
	// the way an answer would be. Without it the promise a pending result makes
	// — "news will arrive at your prompt" — is silently never kept.
	var quiet []endedErrand
	for _, e := range ended {
		if e.t.attended == 0 {
			quiet = append(quiet, e)
		}
	}
	s.mu.Unlock()

	for _, e := range ended {
		s.clear(e.t)
		if s.events != nil {
			s.events.Emit(StalledEventName, StalledEvent{
				ID: e.t.fromID, TargetID: e.t.targetID, Target: e.t.target,
			})
		}
	}
	for _, e := range quiet {
		s.stash(e.id, e.t, StatusUnanswered, "")
	}
	// This session as a sender: its turn ending frees its prompt, which is what
	// a nudge held back during the turn was waiting for.
	if state == stateDone {
		s.flushNudge(sessionID)
	}
}

// endedErrand is a ticket a state report closed, with the id it was filed under.
type endedErrand struct {
	id string
	t  *ticket
}

// recordState files one report into the turn map and the roster. Called with
// s.mu held.
func (s *Service) recordState(sessionID, state string) {
	// waiting keeps the previous state on record. Mid-turn it means a permission
	// prompt — the turn is still open, and a delivery now queues behind it, so it
	// has to keep reading as busy: read as idle it would arm the receipt check
	// against a target that cannot pick anything up until a human answers, and
	// the errand would be reported unread with its message still queued. After
	// done it is the provider's "your turn" nudge, and done is what must survive.
	// idle is SessionEnd: an ended session reports nothing more, and keeping a
	// row for it would grow the map by one dead entry per session for the life
	// of the process — absent reads as "not working", which is what idle means.
	switch state {
	case stateWaiting:
	case stateIdle:
		delete(s.state, sessionID)
	default:
		s.state[sessionID] = state
	}
	// The roster publishes the report itself rather than the turn state above:
	// waiting is exactly what a caller has to see, and idle drops the row for
	// the same reason it does there — an ended session reports nothing more.
	switch {
	case state == stateIdle:
		delete(s.reported, sessionID)
	case state == stateWaiting && s.state[sessionID] != stateBusy:
		// Except when that waiting is not a block at all. One Notification
		// means both "I need a permission decision" and "I have been sitting at
		// my prompt" (docs/hooks/session-state.md, and internal/terminal's
		// turnLog, which keeps the card honest about the same pair). Only the
		// first is a session a caller must not send work into; the second is
		// the most available a session ever is, and publishing it as waiting
		// tells every peer to hold off. The turn state above is what tells them
		// apart, and it is left standing here.
	default:
		s.reported[sessionID] = state
	}

}

// endedErrands takes every ticket the report ends off the table and closes
// its stall channel, for Observe to announce outside the lock. Called with
// s.mu held.
func (s *Service) endedErrands(sessionID, state string) []endedErrand {
	var ended []endedErrand
	switch state {
	case stateBusy:
		for _, t := range s.tickets {
			// A ticket whose message is still held back by the target's setup
			// has nothing in that PTY yet; whatever runs there is not about it.
			if t.targetID == sessionID && !t.delivered.IsZero() {
				t.sawBusy = true
			}
		}
	case stateIdle:
		// SessionEnd needs no turn to have run: the CLI has left the PTY and
		// nothing there can answer anymore. A ticket still queued is left for
		// awaitReady, which sees the session die and reports it undelivered —
		// a different thing to be told, and its own message to be told it in.
		for id, t := range s.tickets {
			if t.targetID == sessionID && !t.delivered.IsZero() {
				delete(s.tickets, id)
				close(t.stalled)
				ended = append(ended, endedErrand{id, t})
			}
		}
	case stateDone:
		if id, t := s.turnErrand(sessionID); t != nil {
			delete(s.tickets, id)
			close(t.stalled)
			ended = append(ended, endedErrand{id, t})
		}
	}
	return ended
}

// turnErrand decides which of a target's errands the turn that just ended
// belongs to, and returns it — nil when the turn was nobody's. A turn answers
// for at most one errand: every provider queues typed input, so two messages
// delivered to one session run as two turns, and a single done closing both
// would report "answered elsewhere" about a request the target had not read
// yet. The oldest delivery owns the turn; the rest wait for their own, their
// busy marks reset so the next turn starts clean. Called under s.mu.
func (s *Service) turnErrand(sessionID string) (string, *ticket) {
	var oldestID string
	var oldest *ticket
	for id, t := range s.tickets {
		if t.targetID != sessionID || t.delivered.IsZero() {
			continue
		}
		// The turn that was already running when the message was delivered. Its
		// own busy reports say nothing about this request either.
		if t.skipTurns > 0 {
			t.skipTurns--
			t.sawBusy = false
			continue
		}
		if !t.sawBusy {
			continue
		}
		if oldest == nil || t.deliverySeq < oldest.deliverySeq {
			oldestID, oldest = id, t
		}
	}
	for _, t := range s.tickets {
		if t.targetID == sessionID && t != oldest {
			t.sawBusy = false
		}
	}
	return oldestID, oldest
}

// announce raises or clears one session's mark. Called outside s.mu on every
// path: Emit blocks on a stalled /events client, and holding the relay's lock
// across it would stall every other errand behind one unread window.
func (s *Service) announce(sessionID, peer, direction, ticketID string) {
	if s.events == nil || sessionID == "" {
		return
	}
	s.events.Emit(RelayEventName, RelayEvent{
		ID: sessionID, Peer: peer, Direction: direction, Ticket: ticketID,
	})
}

// clear takes down both ends' marks for a ticket that is over.
func (s *Service) clear(t *ticket) {
	s.announce(t.targetID, "", "", "")
	s.announce(t.fromID, "", "", "")
}

func (s *Service) clearAll(tickets []*ticket) {
	for _, t := range tickets {
		s.clear(t)
	}
}
