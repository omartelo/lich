package relay

import (
	"log/slog"
	"sort"
)

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
	ended, notice, nudged := s.endedErrands(sessionID, state)
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
	if notice != "" {
		s.askForATicket(sessionID, notice, nudged)
	}
	// This session as a sender: its turn ending frees its prompt, which is what
	// a nudge held back during the turn was waiting for.
	if state == stateDone {
		s.flushNudge(sessionID)
	}
}

// askForATicket types the notice at the worker's own prompt and gives the marks
// back if it never got there. The notice is the whole of what happens to an
// ambiguous turn — no sender is told anything — so one that failed to arrive has
// to be sendable again, the same bookkeeping flushNudge does for the inbox.
func (s *Service) askForATicket(sessionID, notice string, nudged []string) {
	err := s.deliver(sessionID, notice)
	if err == nil {
		return
	}
	slog.Warn("relay: ticket request not delivered", "session", sessionID, "err", err)
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, id := range nudged {
		if t, ok := s.tickets[id]; ok {
			t.nudged = false
		}
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

// endedErrands takes every ticket the report ends off the table and closes its
// stall channel, for Observe to announce outside the lock. Called with s.mu
// held.
//
// It returns two more things for the turn nothing can attribute: the notice to
// type at the worker's prompt, and the tickets it was marked against, so a
// notice that never arrived can be sent again.
func (s *Service) endedErrands(sessionID, state string) ([]endedErrand, string, []string) {
	var ended []endedErrand
	notice, nudged := "", []string(nil)
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
		candidates := s.turnCandidates(sessionID)
		if len(candidates) == 1 {
			id := candidates[0]
			t := s.tickets[id]
			delete(s.tickets, id)
			close(t.stalled)
			ended = append(ended, endedErrand{id, t})
			break
		}
		if len(candidates) > 1 {
			notice, nudged = s.askTicketNoticeLocked(candidates)
		}
	}
	return ended, notice, nudged
}

// askTicketNoticeLocked words the notice for a turn that ended with more than
// one errand it could have been, and marks the errands it names. Empty when
// every one of them has been asked about already: the notice is typed at the
// worker's prompt and starts a turn of its own, so nudging on every turn end
// would nudge that session forever. A task that arrives later is unmarked and
// asks again, which is the rule the inbox nudge follows for the same reason.
// Called under s.mu.
func (s *Service) askTicketNoticeLocked(candidates []string) (string, []string) {
	var fresh []string
	for _, id := range candidates {
		if t := s.tickets[id]; !t.nudged {
			t.nudged = true
			fresh = append(fresh, id)
		}
	}
	if len(fresh) == 0 {
		return "", nil
	}
	return pickTicketNudge(len(candidates), openErrands(s.tickets, candidates)), fresh
}

// turnCandidates is every errand of a target's the turn that just ended could
// have been, oldest delivery first. A turn answers for at most one errand:
// every provider queues typed input, so two messages delivered to one session
// run as two turns, and a single done closing both would report "answered
// elsewhere" about a request the target had not read yet.
//
// One candidate is that turn's errand and nothing else's. Two are a turn
// nothing here can attribute — the target's screen is the only place that says
// which task it worked on, and reading it is the one thing this feature does
// not do — so the caller closes neither and asks the worker to name the ticket
// instead. Picking the oldest, which this used to do, closed a stranger's
// errand on a guess and told its sender the work was over.
//
// A message delivered mid-turn is not a candidate for that turn's end: the turn
// was already running, so its ending says nothing about the task pasted into
// it. That is what skipTurns counts, and it is why the case a second task
// arrives inside the first's turn is attributable rather than ambiguous.
//
// Called under s.mu. It consumes the skip counts and clears the busy marks, so
// one turn ending is read exactly once and the next starts clean.
func (s *Service) turnCandidates(sessionID string) []string {
	var candidates []string
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
		candidates = append(candidates, id)
	}
	for _, t := range s.tickets {
		if t.targetID == sessionID {
			t.sawBusy = false
		}
	}
	sort.Slice(candidates, func(i, j int) bool {
		return s.tickets[candidates[i]].deliverySeq < s.tickets[candidates[j]].deliverySeq
	})
	return candidates
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
