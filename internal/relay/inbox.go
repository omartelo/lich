package relay

import (
	"fmt"
	"log/slog"
	"sort"
	"sync"
	"time"
)

// The inbox: where an errand's outcome waits when nobody was holding the line
// for it, how the sender is told it is there, and how it is collected.

// defaultNudgeDelay is how long a result sits before the nudge naming it is
// typed. Parallel workers finish in bursts — a fan-out answered within seconds
// of itself would otherwise cost one nudge, and one restarted turn, per worker.
const defaultNudgeDelay = 2 * time.Second

// InboxEventName carries how many uncollected results a session has waiting,
// so its card can say so — the nudge tells the agent, and this tells the
// person. Emitted whenever a sender's inbox changes size; zero clears the
// mark. Global for the same reason the relay event is.
const InboxEventName = "session-inbox"

// InboxEvent is the payload of InboxEventName.
type InboxEvent struct {
	ID    string `json:"id"`
	Count int    `json:"count"`
}

// Collected is what a drain returns: every outcome that was waiting for the
// caller, oldest first, and the labels of the sessions still owing one.
type Collected struct {
	Results []Result `json:"results"`
	Open    []string `json:"open"`
}

// inboxEntry is one finished errand whose outcome nobody was holding the line
// for. The result is kept here until the sender collects it instead of being
// typed at the sender's prompt in full: N results arriving as N prompt
// submissions each restart the sender's turn and leave the whole text in its
// context window — exactly the cost an orchestrating session cannot pay per
// worker. What is typed instead is one short nudge (nudgeNotice), debounced so
// a burst of workers finishing together costs one.
type inboxEntry struct {
	ticket string
	fromID string
	target string
	status string
	answer string
	ready  time.Time
	// nudged is whether a notice naming this entry was already typed. One nudge
	// per entry: re-nudging on every turn end would start a turn per nudge,
	// forever, for a sender that chose not to collect.
	nudged bool
}

// Collect drains every outcome waiting for fromID, oldest first. With nothing
// ready and errands still open it holds the line for the next one; with
// nothing ready and nothing open it returns empty at once. It is the batch
// half of the nudge: one call picks up what any number of workers produced.
func (s *Service) Collect(fromID string, waitSeconds int) (Collected, error) {
	if fromID == "" {
		return Collected{}, fmt.Errorf(
			"collecting needs a session of your own — outside one, wait on the ticket instead: lich wait <ticket>")
	}
	deadline := s.now().Add(waitFor(waitSeconds))
	for {
		s.mu.Lock()
		expired, senders := s.sweep()
		results := s.drainLocked(fromID)
		open := s.openLocked(fromID)
		if len(results) > 0 || len(open) == 0 {
			s.mu.Unlock()
			s.clearAll(expired)
			s.announceInboxAll(senders)
			if len(results) > 0 {
				s.announceInbox(fromID)
			}
			return Collected{Results: results, Open: open}, nil
		}
		wake := make(chan struct{}, 1)
		s.collectors[fromID] = append(s.collectors[fromID], wake)
		s.mu.Unlock()
		s.clearAll(expired)
		s.announceInboxAll(senders)

		remaining := deadline.Sub(s.now())
		if remaining <= 0 {
			s.unregister(fromID, wake)
			return Collected{Results: []Result{}, Open: open}, nil
		}
		timer := time.NewTimer(remaining)
		select {
		case <-wake:
			timer.Stop()
			s.unregister(fromID, wake)
		case <-timer.C:
			s.unregister(fromID, wake)
			// One last look: the result may have landed in the same instant.
			s.mu.Lock()
			results := s.drainLocked(fromID)
			open := s.openLocked(fromID)
			s.mu.Unlock()
			if len(results) > 0 {
				s.announceInbox(fromID)
			}
			return Collected{Results: results, Open: open}, nil
		}
	}
}

// drainLocked empties fromID's inbox, oldest first. Called under s.mu. It
// never returns nil: a JSON null where a script expects a list is a bug it
// should not have to defend against.
func (s *Service) drainLocked(fromID string) []Result {
	var found []*inboxEntry
	for id, e := range s.ready {
		if e.fromID != fromID {
			continue
		}
		delete(s.ready, id)
		found = append(found, e)
	}
	sort.Slice(found, func(i, j int) bool { return found[i].ready.Before(found[j].ready) })
	results := make([]Result, 0, len(found))
	for _, e := range found {
		results = append(results, Result{Ticket: e.ticket, Target: e.target, Status: e.status, Answer: e.answer})
	}
	return results
}

// openLocked is the labels still owing fromID an answer, deduplicated and
// sorted. Called under s.mu.
func (s *Service) openLocked(fromID string) []string {
	seen := map[string]bool{}
	for _, t := range s.tickets {
		if t.fromID == fromID {
			seen[t.target] = true
		}
	}
	labels := make([]string, 0, len(seen))
	for label := range seen {
		labels = append(labels, label)
	}
	sort.Strings(labels)
	return labels
}

// unregister drops one Collect call's wake channel.
func (s *Service) unregister(fromID string, wake chan struct{}) {
	s.mu.Lock()
	defer s.mu.Unlock()
	list := s.collectors[fromID]
	for i, ch := range list {
		if ch == wake {
			s.collectors[fromID] = append(list[:i], list[i+1:]...)
			break
		}
	}
	if len(s.collectors[fromID]) == 0 {
		delete(s.collectors, fromID)
	}
}

// stash puts one finished errand's outcome in the inbox and decides how the
// sender hears about it: a Collect already holding the line is woken, a busy
// sender is left alone until its turn ends (Observe flushes then), and an idle
// one gets a nudge after the debounce. A sender that is not a session — the
// `lich` command from a script — is never nudged; its entry waits to be asked
// for with `lich wait <ticket>`.
func (s *Service) stash(id string, t *ticket, status, answer string) {
	s.mu.Lock()
	s.ready[id] = &inboxEntry{
		ticket: id, fromID: t.fromID, target: t.target,
		status: status, answer: answer, ready: s.now(),
	}
	woke := false
	for _, wake := range s.collectors[t.fromID] {
		select {
		case wake <- struct{}{}:
		default:
		}
		woke = true
	}
	nudgeable := !woke && t.fromID != "" && s.state[t.fromID] != stateBusy
	if nudgeable && s.nudgeTimer[t.fromID] == nil {
		fromID := t.fromID
		s.nudgeTimer[fromID] = time.AfterFunc(s.nudgeDelay, func() { s.flushNudge(fromID) })
	}
	s.mu.Unlock()
	s.announceInbox(t.fromID)
}

// countLocked is how many results wait for fromID. Called under s.mu.
func (s *Service) countLocked(fromID string) int {
	count := 0
	for _, e := range s.ready {
		if e.fromID == fromID {
			count++
		}
	}
	return count
}

// announceInbox tells the window how many results a sender has waiting. The
// count is read here rather than handed in, under a lock held across the emit:
// two errands finishing together would otherwise count 1 and 2 under s.mu and
// emit them in either order, leaving the card saying one result with two of
// them waiting until the next change. The lock is its own, because Emit blocks
// on a stalled /events client and s.mu may not be held across that.
func (s *Service) announceInbox(fromID string) {
	if s.events == nil || fromID == "" {
		return
	}
	s.announceMu.Lock()
	defer s.announceMu.Unlock()
	s.mu.Lock()
	count := s.countLocked(fromID)
	s.mu.Unlock()
	s.events.Emit(InboxEventName, InboxEvent{ID: fromID, Count: count})
}

func (s *Service) announceInboxAll(senders []string) {
	for _, fromID := range senders {
		s.announceInbox(fromID)
	}
}

// flushNudge types one nudge naming everything waiting for fromID, if any of
// it has not been nudged before. Ran by the debounce timer and by Observe when
// the sender's turn ends — a delivery mid-turn would queue as its own turn,
// which is the cost this whole path exists to avoid. The two triggers can land
// on the same fromID close together, so the whole attempt is serialized per
// sender: the later call waits for the earlier one's outcome — including its
// unmark on failure — instead of finding the entry still marked and giving up.
func (s *Service) flushNudge(fromID string) {
	s.mu.Lock()
	nm, ok := s.nudging[fromID]
	if !ok {
		nm = &sync.Mutex{}
		s.nudging[fromID] = nm
	}
	s.mu.Unlock()
	nm.Lock()
	defer nm.Unlock()

	s.mu.Lock()
	if timer := s.nudgeTimer[fromID]; timer != nil {
		timer.Stop()
		delete(s.nudgeTimer, fromID)
	}
	if s.state[fromID] == stateBusy {
		// The prompt is not free; the end of this turn flushes instead.
		s.mu.Unlock()
		return
	}
	var marked []string
	count := 0
	targets := map[string]bool{}
	for id, e := range s.ready {
		if e.fromID != fromID {
			continue
		}
		if !e.nudged {
			e.nudged = true
			marked = append(marked, id)
		}
		count++
		targets[e.target] = true
	}
	s.mu.Unlock()
	if len(marked) == 0 {
		return
	}
	labels := make([]string, 0, len(targets))
	for label := range targets {
		labels = append(labels, label)
	}
	sort.Strings(labels)

	sender, _ := s.sessionOf(fromID)
	if err := s.deliver(fromID, nudgeNotice(count, labels, s.offersTools(sender.Kind))); err != nil {
		// The notice is the only thing that tells an agent results are waiting —
		// the results themselves are never typed — so one that never reached the
		// prompt has to be sendable again: the entries take their mark back and the
		// sender's next turn end tries once more. Marking before the write and
		// giving it back after is what keeps a burst to one nudge; marking only on
		// success would let two flushes racing each find the same entry fresh.
		slog.Warn("relay: news not delivered", "session", fromID, "err", err)
		s.mu.Lock()
		for _, id := range marked {
			if e, ok := s.ready[id]; ok {
				e.nudged = false
			}
		}
		s.mu.Unlock()
	}
}
