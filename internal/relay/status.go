package relay

import (
	"sort"
	"time"
)

// stateQueued is the state an open errand reads in while its task still waits
// for the target to reach a prompt (docs/hooks/mod-status.md).
const stateQueued = "queued"

// Status is what a session's mod reads for its status line: the errands it
// owes an answer to, the ones it handed out that are still open, and the
// outcomes waiting in its inbox (docs/hooks/mod-status.md). Every list is
// oldest first and never nil.
type Status struct {
	Owed  []OwedErrand  `json:"owed"`
	Open  []OpenErrand  `json:"open"`
	Ready []ReadyErrand `json:"ready"`
}

// OwedErrand is a request handed to the session that it has not answered.
// From is the asking session's label, empty for the command line.
type OwedErrand struct {
	Ticket string `json:"ticket"`
	From   string `json:"from"`
	Asked  string `json:"asked"`
}

// OpenErrand is a request the session handed out that has not ended. State is
// stateQueued until the task is delivered, then what the target last reported.
type OpenErrand struct {
	Ticket string `json:"ticket"`
	Target string `json:"target"`
	State  string `json:"state"`
}

// ReadyErrand is an outcome waiting in the session's inbox.
type ReadyErrand struct {
	Ticket string `json:"ticket"`
	Target string `json:"target"`
	Status string `json:"status"`
}

// Status reads id's errands without collecting anything: an outcome listed as
// ready stays in the inbox for the agent's own collect. Private tickets are
// left out of Open and Ready, which no collect of the session reaches.
func (s *Service) Status(id string) Status {
	s.mu.Lock()
	expired, senders := s.sweep()
	status := Status{Owed: s.owedLocked(id), Open: s.openErrandsLocked(id), Ready: s.readyLocked(id)}
	s.mu.Unlock()
	s.clearAll(expired)
	s.announceInboxAll(senders)
	return status
}

// owedLocked lists the errands id would answer at its prompt: submitted ones,
// the same rule errandOfLocked applies, in hand-off order. Called under s.mu.
func (s *Service) owedLocked(id string) []OwedErrand {
	var ids []string
	for ticketID, t := range s.tickets {
		if t.targetID == id && t.submitted {
			ids = append(ids, ticketID)
		}
	}
	sort.Slice(ids, func(i, j int) bool { return s.tickets[ids[i]].deliverySeq < s.tickets[ids[j]].deliverySeq })
	owed := make([]OwedErrand, 0, len(ids))
	for _, ticketID := range ids {
		t := s.tickets[ticketID]
		owed = append(owed, OwedErrand{Ticket: ticketID, From: t.sender, Asked: t.asked})
	}
	return owed
}

// openErrandsLocked lists id's own open errands, oldest first. Called under
// s.mu.
func (s *Service) openErrandsLocked(id string) []OpenErrand {
	open := make([]OpenErrand, 0)
	created := map[string]time.Time{}
	for ticketID, t := range s.tickets {
		if t.fromID != id || t.private {
			continue
		}
		state := s.reported[t.targetID]
		if t.delivered.IsZero() {
			state = stateQueued
		}
		open = append(open, OpenErrand{Ticket: ticketID, Target: t.target, State: state})
		created[ticketID] = t.created
	}
	sort.Slice(open, func(i, j int) bool {
		return oldestFirst(created[open[i].Ticket], created[open[j].Ticket], open[i].Ticket, open[j].Ticket)
	})
	return open
}

// readyLocked lists the outcomes waiting in id's inbox, oldest first. Called
// under s.mu.
func (s *Service) readyLocked(id string) []ReadyErrand {
	var found []*inboxEntry
	for _, e := range s.ready {
		if e.fromID == id {
			found = append(found, e)
		}
	}
	sort.Slice(found, func(i, j int) bool {
		return oldestFirst(found[i].ready, found[j].ready, found[i].ticket, found[j].ticket)
	})
	ready := make([]ReadyErrand, 0, len(found))
	for _, e := range found {
		ready = append(ready, ReadyErrand{Ticket: e.ticket, Target: e.target, Status: e.status})
	}
	return ready
}

// oldestFirst orders by time and breaks a tie by ticket, so two errands opened
// inside one tick of a coarse clock still list the same way on every read.
func oldestFirst(a, b time.Time, ticketA, ticketB string) bool {
	if !a.Equal(b) {
		return a.Before(b)
	}
	return ticketA < ticketB
}
