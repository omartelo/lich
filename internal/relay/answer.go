package relay

import (
	"fmt"
	"sort"
	"strings"
	"time"
)

// Wait blocks on an already-delivered ticket for another round, so a caller
// whose first wait ran out can come back without sending the message twice.
// An outcome already sitting in the inbox is handed over on the spot.
func (s *Service) Wait(ticketID string, waitSeconds int) (Result, error) {
	s.mu.Lock()
	expired, senders := s.sweep()
	if e, ok := s.ready[ticketID]; ok {
		delete(s.ready, ticketID)
		s.mu.Unlock()
		s.clearAll(expired)
		s.announceInboxAll(senders)
		s.announceInbox(e.fromID)
		return Result{Ticket: e.ticket, Target: e.target, Status: e.status, Answer: e.answer}, nil
	}
	t, ok := s.tickets[ticketID]
	s.mu.Unlock()
	s.clearAll(expired)
	s.announceInboxAll(senders)
	if !ok {
		return Result{}, fmt.Errorf("unknown ticket %q — it was answered long ago, or expired", ticketID)
	}
	return s.await(ticketID, t, waitFor(waitSeconds)), nil
}

// Reply hands an answer back to whoever is waiting on ticketID. It is what the
// message composed by Send asks the receiving agent to run.
//
// An empty ticketID answers the one errand open against replierID, the way an
// empty ticket collects everything waiting for a sender (see Collect). The
// ticket is written down in one place only — the message typed at the target's
// prompt — so an agent whose context no longer reaches that message would
// otherwise be holding an answer with no route home, and the sender blocked on
// a ticket nobody can name. With two open it is refused: see errandOfLocked.
func (s *Service) Reply(replierID, ticketID, answer string) error {
	answer = sanitize(answer)
	if len(answer) > answerLimit {
		// The cut is in bytes and can land inside a rune; the tail is typed into
		// a PTY, and half a rune there is garbage keystrokes, not text.
		answer = strings.ToValidUTF8(answer[:answerLimit], "")
	}

	s.mu.Lock()
	if ticketID == "" {
		id, err := s.errandOfLocked(replierID)
		if err != nil {
			s.mu.Unlock()
			return err
		}
		ticketID = id
	}
	t, ok := s.tickets[ticketID]
	if !ok {
		s.mu.Unlock()
		return fmt.Errorf("unknown ticket %q — it was answered already, or expired", ticketID)
	}
	select {
	case <-t.done:
		s.mu.Unlock()
		return fmt.Errorf("ticket %q was already answered", ticketID)
	default:
	}
	t.answer = answer
	t.answered = true
	close(t.done)
	// The errand is over the moment the answer lands, whether or not anyone is
	// still waiting on it: a sender whose wait ran out has moved on, and the
	// mark on both cards has to go with the ticket rather than with the waiter.
	delete(s.tickets, ticketID)
	unattended := t.attended == 0
	s.mu.Unlock()

	s.clear(t)
	if unattended {
		s.stash(ticketID, t, StatusAnswered, answer)
	}
	return nil
}

// errandOfLocked is the errand an answer that named no ticket belongs to: the
// single message delivered to this session and still open. Called under s.mu.
//
// Nothing in an answer itself says which request it belongs to, so with two
// errands open there is no way to tell them apart — and the guess this used to
// make (the oldest delivery) sent a session's answer to its second task home as
// the answer to its first, which both senders then read as a confident report of
// work they never asked for. So it is refused instead, naming every open ticket
// with the line it asked for, and the agent retries with the one it means. One
// open errand needs no guess, and none is still the error it always was.
//
// A queued task nobody has read yet is never listed: it is not at that prompt,
// so no answer at that prompt can be about it.
func (s *Service) errandOfLocked(replierID string) (string, error) {
	if replierID == "" {
		return "", fmt.Errorf(
			"answering without a ticket needs a session of your own — name it instead: lich reply <ticket> \"<answer>\"")
	}
	open := make([]string, 0, len(s.tickets))
	for id, t := range s.tickets {
		if t.targetID != replierID || t.delivered.IsZero() {
			continue
		}
		open = append(open, id)
	}
	switch len(open) {
	case 0:
		return "", fmt.Errorf("no open request to answer — nobody is waiting on this session")
	case 1:
		return open[0], nil
	}
	// Hand-off order, so the list reads the way the tasks arrived rather than the
	// way Go happened to walk the map.
	sort.Slice(open, func(i, j int) bool {
		return s.tickets[open[i]].deliverySeq < s.tickets[open[j]].deliverySeq
	})
	return "", fmt.Errorf(
		"%d requests are open against this session, and an answer that names no ticket "+
			"would close the wrong one. Name the ticket the answer belongs to:\n%s",
		len(open), openErrands(s.tickets, open),
	)
}

// openErrands lists the errands a ticketless answer was refused with: the reply
// command for each, and the line it asked for beside it, so the agent picks the
// ticket by what the task was rather than by a number it cannot tell apart.
func openErrands(tickets map[string]*ticket, ids []string) string {
	return errandLines(tickets, ids, "  lich reply ", " \"<answer>\"")
}

// namedErrands lists the same errands as history rather than as somewhere to
// reply: the ticket and what it asked, no command. It is what a session is
// shown about errands that are already over — an invitation to answer one of
// those is an invitation to run something that fails.
func namedErrands(tickets map[string]*ticket, ids []string) string {
	return errandLines(tickets, ids, "  ", "")
}

// errandLines renders one line per errand: the ticket between lead and tail,
// and the opening of what it asked beside it.
func errandLines(tickets map[string]*ticket, ids []string, lead, tail string) string {
	var b strings.Builder
	for _, id := range ids {
		b.WriteString(lead + id + tail)
		if asked := tickets[id].asked; asked != "" {
			fmt.Fprintf(&b, "   — %s", asked)
		}
		b.WriteString("\n")
	}
	return strings.TrimRight(b.String(), "\n")
}

// askedLimit is how much of a prompt an errand is named by. Long enough to tell
// two tasks apart, short enough that a session holding several still reads as a
// list rather than as the prompts all over again.
const askedLimit = 72

// askedExcerpt is the opening of a prompt on one line. The cut is in bytes and
// can land inside a rune, which is a replacement character in the middle of the
// one line that identifies an errand.
func askedExcerpt(prompt string) string {
	line := strings.Join(strings.Fields(prompt), " ")
	if len(line) <= askedLimit {
		return line
	}
	return strings.ToValidUTF8(line[:askedLimit], "") + "…"
}

// await blocks on one ticket until it is answered or the wait runs out. It only
// reads: the ticket is dropped by whoever closes it (Reply, or sweep), because
// a wait that expired leaves an errand that is still open and still has to be
// waitable, and an answer has to reach a caller who stopped waiting for it too.
func (s *Service) await(id string, t *ticket, wait time.Duration) Result {
	timer := time.NewTimer(wait)
	defer timer.Stop()

	s.mu.Lock()
	t.attended++
	s.mu.Unlock()

	// An answer that is already in outranks everything: a target may reply and
	// end its turn in the same breath, and a select over two ready channels
	// picks between them at random.
	select {
	case <-t.done:
		s.leave(t)
		return Result{Ticket: id, Target: t.target, Status: StatusAnswered, Answer: t.answer}
	default:
	}

	select {
	case <-t.done:
		s.leave(t)
		return Result{Ticket: id, Target: t.target, Status: StatusAnswered, Answer: t.answer}
	case <-t.stalled:
		s.leave(t)
		return Result{Ticket: id, Target: t.target, Status: StatusUnanswered}
	case <-t.unread:
		s.leave(t)
		return Result{Ticket: id, Target: t.target, Status: StatusUnread}
	case <-t.undelivered:
		s.leave(t)
		return Result{Ticket: id, Target: t.target, Status: StatusUndelivered}
	case <-timer.C:
		return Result{Ticket: id, Target: t.target, Status: s.giveUp(id, t)}
	}
}

// leave drops this caller's claim on a ticket it is carrying the answer out of.
func (s *Service) leave(t *ticket) {
	s.mu.Lock()
	t.attended--
	s.mu.Unlock()
}

// giveUp drops the claim of a caller whose wait ran out and returns the status
// that caller should hear. It catches the races that would lose what became of
// the errand, all of the same shape: the ticket closing in the same instant,
// seeing this caller still attending, and leaving the news for it — after it
// had already stopped listening. Whoever leaves last owns what is left behind.
//
// A reply is stashed for the sender, because an answer outlives the wait it
// missed. The three ways an errand ends without one — the receipt window
// closing it unread, a turn ending with no answer, a message that never
// reached a prompt — are told to the caller instead: it is still here to hear
// them, and the alternative is reporting the errand as still in progress, on a
// ticket already out of the map.
func (s *Service) giveUp(id string, t *ticket) string {
	s.mu.Lock()
	t.attended--
	last := t.attended == 0
	orphaned := t.answered && last
	unread, stalled, undelivered := false, false, false
	if last && !t.answered {
		select {
		case <-t.unread:
			unread = true
		default:
		}
		select {
		case <-t.stalled:
			stalled = true
		default:
		}
		select {
		case <-t.undelivered:
			undelivered = true
		default:
		}
	}
	s.mu.Unlock()
	if orphaned {
		s.stash(id, t, StatusAnswered, t.answer)
	}
	if unread {
		return StatusUnread
	}
	if stalled {
		return StatusUnanswered
	}
	if undelivered {
		return StatusUndelivered
	}
	return StatusPending
}
