package relay

import (
	"log/slog"
	"time"

	"github.com/omartelo/lich/internal/store"
)

// A turn a plan's usage limit ended is picked up again once the limit resets:
// lich parks a scheduled prompt for that moment (later.go), the same row a
// person parks one on, so it shows on the card, survives a restart and is typed
// the way every scheduled prompt is. The terminal service finds the limit and
// its reset (internal/terminal, limit.go); this file decides whether to park.

// resumePrompt is what lich types at a session whose usage limit has reset. It
// is also how a parked prompt is told apart as lich's: the row has no column
// saying who wrote it.
// ponytail: matched by text, a person who parks these exact words has them
// dropped on the session's next turn; a column on the row if that ever matters.
const resumePrompt = "[lich] Your usage limit reset. Continue the task you were working on."

// resumeGrace is how long after the reset the prompt is due. Claude Code's own
// autoContinueAtUsageLimit, where the server enables it, resumes at the reset
// itself; the grace lets it go first, and its turn starting drops this prompt
// (dropResume) instead of a second continuation landing behind it.
const resumeGrace = 2 * time.Minute

// resumeHorizon is how far out a reset may be and still be waited for. The same
// horizon Claude Code's own auto-continue gives up at (2.1.293: "the usage limit
// now resets more than 24 hours out"): picking a task back up days later, on its
// own, surprises more than it helps, so the card only says when it resets.
const resumeHorizon = 24 * time.Hour

// ParkResume parks the continuation of session id's turn, due resumeGrace after
// resetsAt (unix seconds). Nothing is parked for a reset already past or beyond
// resumeHorizon, nor over a prompt already parked there: there is one per
// session, and a person's own outranks lich's.
func (s *Service) ParkResume(id string, resetsAt int64) {
	now := s.now()
	reset := time.Unix(resetsAt, 0)
	if !reset.After(now) || reset.Sub(now) > resumeHorizon {
		return
	}
	sess, ok := s.sessionRow(id)
	if !ok || sess.ScheduledPrompt != "" {
		return
	}
	due := reset.Add(resumeGrace).Unix()
	if err := s.sessions.SetSessionSchedule(id, due, resumePrompt); err != nil {
		slog.Warn("relay: park resume", "session", sess.Label, "err", err)
		return
	}
	s.markResume(id)
	if s.events != nil {
		s.events.Emit(ScheduleEventName, ScheduleEvent{ID: id, At: due, Prompt: resumePrompt})
	}
}

// dropResume clears session id's parked continuation, which a turn starting
// there has made moot: the person went on by hand, the provider resumed on its
// own, or something else was delivered. A prompt the person parked in its
// place since is theirs and stays.
func (s *Service) dropResume(id string) {
	sess, ok := s.sessionRow(id)
	if !ok || sess.ScheduledPrompt != resumePrompt {
		return
	}
	if err := s.sessions.SetSessionSchedule(id, 0, ""); err != nil {
		slog.Warn("relay: drop resume", "session", sess.Label, "err", err)
		return
	}
	if s.events != nil {
		s.events.Emit(ScheduleEventName, ScheduleEvent{ID: id})
	}
}

// markResume records that session id holds a continuation lich parked, so the
// turn that makes it moot can drop it without reading the workspace on every
// busy report (see Observe).
func (s *Service) markResume(id string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.resumes[id] = true
}

// takeResumeLocked answers whether a report of state ends session id's parked
// continuation, and forgets it if so. Called under s.mu.
func (s *Service) takeResumeLocked(id, state string) bool {
	if state != stateBusy || !s.resumes[id] {
		return false
	}
	delete(s.resumes, id)
	return true
}

// sessionRow finds session id in the workspace.
func (s *Service) sessionRow(id string) (store.Session, bool) {
	projects, err := s.sessions.LoadState()
	if err != nil {
		slog.Warn("relay: read session", "session", id, "err", err)
		return store.Session{}, false
	}
	for _, p := range projects {
		for _, sess := range p.Sessions {
			if sess.ID == id {
				return sess, true
			}
		}
	}
	return store.Session{}, false
}
