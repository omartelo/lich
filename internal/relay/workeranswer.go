package relay

import "log/slog"

// A subagent worker whose mod answers for it (docs/hooks/mod-answer.md) and a
// worker its caller closed: the two ways a subagent errand ends that a native
// subagent has, its last message coming back as its result and TaskStop.

// taskMessage composes a ticket's message for the target it is handed to now.
// A subagent errand at a worker whose mod answers it is handed the task alone,
// and is marked so a turn ending without the answer does not end it.
func (s *Service) taskMessage(id string, t *ticket, kind string) string {
	if t.subagent && s.term.ModAnswers(t.targetID) {
		s.mu.Lock()
		t.modAnswers = true
		s.mu.Unlock()
		return composeForWorker(t.sender, t.prompt)
	}
	return compose(t.sender, id, t.prompt, s.offersTools(kind))
}

// WorkerAnswered answers the one subagent errand open at workerID with text,
// the final message its mod reported, by the same path as Reply. A session with
// no subagent errand open, or with two, is ignored: the mod cannot tell a worker
// from a session whose subagents the user kept native, and nothing in the text
// says which of two errands it answers. An errand answered already keeps its
// first answer.
func (s *Service) WorkerAnswered(workerID, text string) {
	s.mu.Lock()
	id, ok := s.subagentErrandLocked(workerID)
	s.mu.Unlock()
	if !ok {
		return
	}
	if err := s.Reply(workerID, id, text); err != nil {
		slog.Debug("relay: a worker's answer was not taken", "session", workerID, "err", err)
		return
	}
	s.finishIfTurnEnded(workerID)
}

// subagentErrandLocked is the single subagent errand at workerID that an answer
// can still close: open and handed over, or lapsed. Called under s.mu.
func (s *Service) subagentErrandLocked(workerID string) (string, bool) {
	var found []string
	for id, t := range s.tickets {
		if t.targetID == workerID && t.subagent && t.submitted {
			found = append(found, id)
		}
	}
	for id, t := range s.lapsed {
		if t.targetID == workerID && t.subagent {
			found = append(found, id)
		}
	}
	if len(found) != 1 {
		return "", false
	}
	return found[0], true
}

// finishIfTurnEnded finishes a worker whose answer landed after the turn it
// answered in had already ended: the mod reports from Stop, and the done report
// of the same Stop races it. One landing mid-turn is finished by that done
// (finishedWorkerLocked).
func (s *Service) finishIfTurnEnded(workerID string) {
	s.mu.Lock()
	finish := s.reportedWorkers[workerID] && s.state[workerID] != stateBusy &&
		!s.errandOpenAtLocked(workerID) && s.workerFinished != nil
	if finish {
		delete(s.reportedWorkers, workerID)
	}
	s.mu.Unlock()
	if finish {
		go s.finishWorker(workerID)
	}
}

// errandOpenAtLocked is whether any errand is still open at sessionID. Called
// under s.mu.
func (s *Service) errandOpenAtLocked(sessionID string) bool {
	for _, t := range s.tickets {
		if t.targetID == sessionID {
			return true
		}
	}
	return false
}

// SessionClosed ends the subagent errands at a session that was just closed,
// without telling their callers: closing a worker is its caller stopping it
// (TaskStop runs `lich close`), or the user doing the same from its card, and
// neither has anything left to hear. A caller still holding the line hears
// stopped; the ticket stays waitable as stopped, and nothing reaches the inbox.
// The terminal calls this before the process dies (terminal.SetSessionClosed),
// so the SessionEnd its CLI reports on the way out finds nothing to call
// unanswered. Errands of any other kind end as they always did.
func (s *Service) SessionClosed(sessionID string) {
	var stopped []*ticket
	s.mu.Lock()
	delete(s.reportedWorkers, sessionID)
	for id, t := range s.tickets {
		if t.targetID != sessionID || !t.subagent {
			continue
		}
		delete(s.tickets, id)
		s.lapseLocked(id, t, StatusStopped)
		t.stopped = true
		close(t.stalled)
		stopped = append(stopped, t)
	}
	s.mu.Unlock()
	s.clearAll(stopped)
}
