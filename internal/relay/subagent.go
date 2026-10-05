package relay

import (
	"fmt"
	"strings"
)

// A subagent errand (SendSubagent) is an ordinary one with three differences,
// all here: its report reaches the sender whole rather than as a nudge to
// collect it, the sender hears when its worker blocks on a permission, and
// sweep leaves it alone while the worker runs (workerHolds).

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
