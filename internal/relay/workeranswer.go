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
		return composeForWorker(s.lang(), t.sender, t.prompt)
	}
	return compose(s.lang(), t.sender, id, t.prompt, s.offersTools(kind))
}

// WorkerAnswered answers the one subagent errand open at workerID with text,
// the final message its mod reported, by the same path as Reply. A session with
// no subagent errand open, or with two, is ignored: the mod cannot tell a worker
// from a session whose subagents the user kept native, and nothing in the text
// says which of two errands it answers. An errand answered already keeps its
// first answer.
//
// A worker still awaiting an outcome of its own (awaitsOutcomeLocked) is
// ignored too. The mod reports a turn that left nothing in Claude Code's
// background list, and an errand the worker sent to another session is not in
// that list, so the text is "I handed it off" rather than the result. The
// outcome reaching the worker starts the turn whose report is the answer.
func (s *Service) WorkerAnswered(workerID, text string) {
	s.mu.Lock()
	id, ok := s.subagentErrandLocked(workerID)
	awaiting := s.awaitsOutcomeLocked(workerID)
	s.mu.Unlock()
	if !ok {
		return
	}
	if awaiting {
		slog.Debug("relay: a worker reported while an errand of its own is unread", "session", workerID)
		return
	}
	if err := s.Reply(workerID, id, text); err != nil {
		slog.Debug("relay: a worker's answer was not taken", "session", workerID, "err", err)
		return
	}
	s.finishIfTurnEnded(workerID)
}

// The reasons a worker's mod gives for a turn that ended with nothing left
// running and no answer (docs/hooks/mod-answer.md, Unanswered turns).
const (
	UnansweredBlank   = "blank"
	UnansweredError   = "error"
	UnansweredRefusal = "refusal"
)

// unansweredWhy is what the caller reads about each reason, in the place a
// report would be.
var unansweredWhy = map[string]string{
	UnansweredBlank:   "The worker's last turn ended without a message.",
	UnansweredError:   "The worker's last turn ended in an API error.",
	UnansweredRefusal: "The worker's last turn ended in a refusal.",
}

// IsUnansweredReason is whether reason is one the contract names.
func IsUnansweredReason(reason string) bool {
	_, ok := unansweredWhy[reason]
	return ok
}

// WorkerUnanswered ends the one subagent errand open at workerID as
// unanswered, the way a turn ending without a reply ends an ordinary errand:
// its mod reported a turn that left nothing running and had no answer, so
// nothing else ever would end it while the worker runs. It is ignored where
// WorkerAnswered is, for an errand that already ended, and for an API error
// while a usage limit's continuation is parked at the worker, since that
// continuation is the turn that answers.
//
// The errand stays answerable by its ticket (lapseLocked) and the worker is not
// finished: its user can still steer it to an answer from its card.
func (s *Service) WorkerUnanswered(workerID, reason string) {
	s.mu.Lock()
	id, ok := s.subagentErrandLocked(workerID)
	t := s.tickets[id]
	resumes := reason == UnansweredError && s.resumes[workerID]
	if !ok || t == nil || resumes || s.awaitsOutcomeLocked(workerID) {
		s.mu.Unlock()
		return
	}
	delete(s.tickets, id)
	t.why = unansweredWhy[reason]
	close(t.stalled)
	s.lapseLocked(id, t, StatusUnanswered)
	unattended := t.attended == 0
	if unattended {
		s.stashLocked(id, t, StatusUnanswered, t.why)
	}
	s.mu.Unlock()

	s.clear(t)
	if s.events != nil {
		s.events.Emit(StalledEventName, StalledEvent{ID: t.fromID, TargetID: t.targetID, Target: t.target})
	}
	if unattended {
		s.announceInbox(t.fromID)
	}
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

// owesSubagentAnswerLocked is whether a subagent errand still waits on
// sessionID's answer: someone opened it as their subagent and has not heard
// back. Called under s.mu.
func (s *Service) owesSubagentAnswerLocked(sessionID string) bool {
	for _, t := range s.tickets {
		if t.targetID == sessionID && t.subagent {
			return true
		}
	}
	for _, t := range s.lapsed {
		if t.targetID == sessionID && t.subagent {
			return true
		}
	}
	return false
}

// awaitsOutcomeLocked is whether an outcome is still on its way to sessionID's
// prompt: an errand it sent is open, or a result for it has not been read by a
// turn of its own yet (inboxEntry.seen). A private errand never reaches that
// prompt, so it is not counted: nothing would ever resume the session for it.
// Called under s.mu.
func (s *Service) awaitsOutcomeLocked(sessionID string) bool {
	for _, t := range s.tickets {
		if t.fromID == sessionID && !t.private {
			return true
		}
	}
	for _, e := range s.ready {
		if e.fromID == sessionID && !e.seen {
			return true
		}
	}
	for _, e := range s.held {
		if e.fromID == sessionID && !e.private && !e.seen {
			return true
		}
	}
	return false
}

// seeNewsLocked marks every result sessionID was already told about as read: a
// turn starting after the notice is the one the notice started, or one that
// came after it. Called under s.mu on a busy report.
func (s *Service) seeNewsLocked(sessionID string) {
	for _, e := range s.ready {
		if e.fromID == sessionID && e.nudged {
			e.seen = true
		}
	}
	for _, e := range s.held {
		if e.fromID == sessionID && e.nudged {
			e.seen = true
		}
	}
}

// finishIfTurnEnded finishes a worker whose answer landed after the turn it
// answered in had already ended: the mod reports from Stop, and the done report
// of the same Stop races it. One landing mid-turn is finished by that done
// (finishedWorkerLocked).
func (s *Service) finishIfTurnEnded(workerID string) {
	s.mu.Lock()
	finish := s.reportedWorkers[workerID] && s.state[workerID] != stateBusy &&
		!s.errandOpenAtLocked(workerID) && !s.awaitsOutcomeLocked(workerID) && s.workerFinished != nil
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

// SessionClosed ends the subagent errands at a session that was just closed by
// closerID, empty when the window closed it. A caller that closed its own
// worker (TaskStop runs `lich close` in the caller's process) stopped the work
// and is told nothing; a caller still holding the line hears stopped, and the
// ticket stays waitable as stopped. Anyone else's close ends work the caller
// never stopped, so the stop is filed in the caller's inbox with a nudge, as is
// one for a caller that is itself a worker somebody waits on
// (owesSubagentAnswerLocked), whose answer waits for this outcome
// (WorkerAnswered). The terminal calls this before the process dies
// (terminal.SetSessionClosed), so the SessionEnd its CLI reports on the way out
// finds nothing to call unanswered. Errands of any other kind end as they
// always did.
func (s *Service) SessionClosed(sessionID, closerID string) {
	var stopped []*ticket
	var told []string
	s.mu.Lock()
	delete(s.reportedWorkers, sessionID)
	for id, t := range s.tickets {
		if t.targetID != sessionID || !t.subagent {
			continue
		}
		delete(s.tickets, id)
		if t.attended == 0 && (t.fromID != closerID || s.owesSubagentAnswerLocked(t.fromID)) {
			s.stashLocked(id, t, StatusStopped, "")
			told = append(told, t.fromID)
		} else {
			s.lapseLocked(id, t, StatusStopped)
		}
		t.stopped = true
		close(t.stalled)
		stopped = append(stopped, t)
	}
	s.mu.Unlock()
	s.clearAll(stopped)
	s.announceInboxAll(told)
}
