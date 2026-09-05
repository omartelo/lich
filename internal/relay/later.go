package relay

import (
	"log/slog"
	"time"
)

// A scheduled prompt is the user's own words parked on a session to be typed at
// it later — a reminder that arrives as work, not a job runner. One per
// session, held on the session row itself (internal/store, sessions.scheduled_at):
// scheduling again replaces what was there, which is what keeps the card able
// to say the whole of it in a line.
//
// It lives in this package because delivery is the whole of the feature and the
// delivery is already here: the paste, the wait for the target to take it in,
// and the Enter behind it (see deliver) — the one path that gets a TUI to
// accept a message on every provider and on Windows too. What it deliberately
// does not reuse is the message the relay composes around a task: there is no
// sender, no ticket and nobody to report back to. The user is the one waiting.

// scheduleTick is how often due prompts are looked for. The window's shortest
// shortcut is a quarter of an hour, so this is already far finer than anything
// that can be asked for, and it costs one read of the open sessions.
const scheduleTick = 30 * time.Second

// ScheduleEventName is emitted when a session's scheduled prompt is delivered,
// so the card drops its mark at the moment the prompt is typed rather than at
// the next reload. Payload: ScheduleEvent. The window is the only other writer
// of that row, and it already knows what it wrote — this event says what the
// clock did.
const ScheduleEventName = "session-schedule"

// ScheduleEvent is the payload of ScheduleEventName: the session whose mark
// changed, and when its prompt is now due. At is 0 for every event this package
// emits — the prompt has just been typed and nothing is waiting — and is in the
// payload anyway because the mark is a time, and an event that only ever means
// "clear" would have to be replaced the first time anything else moves one.
type ScheduleEvent struct {
	ID string `json:"id"`
	At int64  `json:"at"`
}

// RunSchedules types due prompts at their sessions until the process ends.
// Started once at launch: it holds nothing, so a workspace that never schedules
// anything pays one read every scheduleTick and nothing else.
func (s *Service) RunSchedules() {
	ticker := time.NewTicker(scheduleTick)
	defer ticker.Stop()
	for range ticker.C {
		s.deliverDue()
	}
}

// deliverDue types every prompt whose time has come, one session at a time.
//
// A session that is not at a prompt is left for the next pass rather than
// failed: the row is the only record the prompt exists, and dropping it would
// take the card's mark away with nothing typed anywhere. That covers the
// session still running its setup script, the one whose user is mid-sentence,
// and the card whose terminal was never opened — none of which is an error, and
// all of which end by themselves.
//
// A prompt that came due while lich was closed is delivered late, on the first
// pass after launch. Late is what this feature promises; silently dropping the
// only copy of what the user wrote is not.
func (s *Service) deliverDue() {
	projects, err := s.sessions.LoadState()
	if err != nil {
		slog.Warn("relay: read scheduled prompts", "err", err)
		return
	}
	now := s.now().Unix()
	for _, p := range projects {
		for _, sess := range p.Sessions {
			if sess.ScheduledPrompt == "" || sess.ScheduledAt == 0 || sess.ScheduledAt > now {
				continue
			}
			if !s.term.Ready(sess.ID) {
				continue
			}
			// Cleared before the write, not after: deliver blocks while the paste
			// settles, and a pass slow enough to overlap the next one would type the
			// same prompt twice. A write that then fails costs a prompt the user can
			// still see on their own screen and retype; two unasked-for turns cannot
			// be taken back.
			if err := s.sessions.SetSessionSchedule(sess.ID, 0, ""); err != nil {
				slog.Warn("relay: clear scheduled prompt", "session", sess.Label, "err", err)
				continue
			}
			if s.events != nil {
				s.events.Emit(ScheduleEventName, ScheduleEvent{ID: sess.ID})
			}
			if err := s.deliver(sess.ID, sess.ScheduledPrompt); err != nil {
				slog.Warn("relay: scheduled prompt not delivered", "session", sess.Label, "err", err)
			}
		}
	}
}
