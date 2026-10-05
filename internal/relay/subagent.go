package relay

import (
	"fmt"
	"log/slog"
	"strings"
)

// A subagent errand (SendSubagent) is an ordinary one with four differences,
// all here: its report reaches the sender whole rather than as a nudge to
// collect it, the sender hears when its worker blocks on a permission, sweep
// leaves it alone while the worker runs (workerHolds), and the worker is
// finished once it reported and its turn ended (finishedWorkerLocked).

// tellNews delivers one flush's news to fromID. With no subagent report among
// it that is the nudge, as ever. With one, the report goes to the sender's mod
// whole, beside the nudge for whatever else waits: the model reads it the way it
// reads a native subagent's result, without a collect call. A sender whose mod
// does not take it is typed the short nudge instead and the reports go back to
// the inbox for the collect it names: typing a long report into a TUI is what
// the mod route exists to avoid.
func (s *Service) tellNews(fromID string, n news, hasTools bool) error {
	nudge := nudgeNotice(n.count, n.labels, hasTools)
	if len(n.reports) == 0 {
		return s.deliver(fromID, nudge)
	}
	handled, err := s.handToMod(fromID, s.reportNote(n, hasTools))
	if err != nil {
		return err
	}
	if handled {
		s.announceInbox(fromID)
		return nil
	}
	s.mu.Lock()
	s.restoreReportsLocked(n.reports)
	s.mu.Unlock()
	return s.typeIn(fromID, nudge)
}

// restoreReportsLocked puts reports takeNewsLocked moved to held back in the
// inbox, unless a Wait on their ticket took them meanwhile. Called under s.mu.
func (s *Service) restoreReportsLocked(reports []*inboxEntry) {
	for _, e := range reports {
		if s.held[e.ticket] == e {
			delete(s.held, e.ticket)
			s.ready[e.ticket] = e
		}
	}
}

// reportNote is the reports of one flush, whole, followed by the nudge for the
// results that are not reports, if any wait.
func (s *Service) reportNote(n news, hasTools bool) string {
	parts := make([]string, 0, len(n.reports)+1)
	for _, e := range n.reports {
		parts = append(parts, subagentReport(e, s.sessions.SessionBranch(e.targetID)))
	}
	if rest := n.count - len(n.reports); rest > 0 {
		parts = append(parts, nudgeNotice(rest, n.others, hasTools))
	}
	return strings.Join(parts, "\n\n")
}

// subagentReport is one worker's report as its sender reads it: who wrote it,
// on which branch, under which ticket, and then the report itself.
func subagentReport(e *inboxEntry, branch string) string {
	on := ""
	if branch != "" {
		on = fmt.Sprintf(" on branch %s", branch)
	}
	return fmt.Sprintf(
		"[lich] Session %q%s finished the task you handed it (ticket %s). Its report:\n\n%s",
		e.target, on, e.ticket, e.answer,
	)
}

// blockedNotice is what a sender is told when its subagent waits on a person.
func blockedNotice(target string) string {
	return fmt.Sprintf(
		"[lich] Session %q, the subagent you opened, is waiting on a permission prompt in its "+
			"card. Its task stays open: open that card to answer it.",
		target,
	)
}

// blockedSendersLocked marks every subagent errand delivered to sessionID as
// told about the block it has just reported, and returns the senders not told
// yet. A busy or done report ends the block (unblockLocked). Called under s.mu,
// before recordState, while s.state still says whether a turn is running: a
// waiting outside one is a prompt at rest, not a block.
func (s *Service) blockedSendersLocked(sessionID string) []*ticket {
	if s.state[sessionID] != stateBusy {
		return nil
	}
	var told []*ticket
	for _, t := range s.tickets {
		if t.targetID != sessionID || !t.subagent || !t.submitted || t.blockNoted {
			continue
		}
		t.blockNoted = true
		told = append(told, t)
	}
	return told
}

// unblockLocked ends the block sessionID was in for its subagent errands, so
// the next one is told again. Called under s.mu.
func (s *Service) unblockLocked(sessionID string) {
	for _, t := range s.tickets {
		if t.targetID == sessionID {
			t.blockNoted = false
		}
	}
}

// SetWorkerFinished wires what runs for a worker that finished: it answered its
// subagent errand and the turn it answered in ended, with no other errand open
// at it. The app closes the worker there when it shares its caller's checkout
// (spawn.CloseFinishedWorker). Called at startup, before any errand exists.
func (s *Service) SetWorkerFinished(fn func(workerID string) error) {
	s.workerFinished = fn
}

// noteWorkerReportedLocked marks the target of a subagent errand that was just
// answered, for the end of its turn to finish. Called under s.mu.
func (s *Service) noteWorkerReportedLocked(t *ticket) {
	if t.subagent {
		s.reportedWorkers[t.targetID] = true
	}
}

// finishedWorkerLocked reads one state report for a worker that reported this
// turn: a done finishes it unless another errand is still open at it, checked
// before endedErrands takes this turn's errands off the table, since one of
// those may still be answered late. An interrupt or the session ending clears
// the mark without finishing it: whoever stopped that turn is still at its
// card. Called under s.mu.
func (s *Service) finishedWorkerLocked(sessionID, state string) bool {
	if !s.reportedWorkers[sessionID] {
		return false
	}
	switch state {
	case stateDone:
	case stateInterrupted, stateIdle:
		delete(s.reportedWorkers, sessionID)
		return false
	default:
		return false
	}
	delete(s.reportedWorkers, sessionID)
	for _, t := range s.tickets {
		if t.targetID == sessionID {
			return false
		}
	}
	return s.workerFinished != nil
}

func (s *Service) finishWorker(workerID string) {
	if err := s.workerFinished(workerID); err != nil {
		slog.Warn("relay: close a finished worker", "session", workerID, "err", err)
	}
}
