package relay

import (
	"fmt"
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
// only copy of what the user wrote is not — and one that lands late says so at
// the prompt it lands on (lateNotice).
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
			text := lateNotice(sess.ScheduledAt, now) + sess.ScheduledPrompt
			if err := s.deliver(sess.ID, text); err != nil {
				slog.Warn("relay: scheduled prompt not delivered", "session", sess.Label, "err", err)
			}
		}
	}
}

// scheduleClock is how the notice spells the moment a prompt was parked for:
// local time, to the minute, with the date — an overdue prompt is often days
// old, and a clock alone would say nothing about which day it was meant for.
const scheduleClock = "2006-01-02 15:04"

// lateNotice is the line put in front of a prompt that missed its time, and ""
// for one that did not. Everything reading the card reads it: the agent, which
// would otherwise act on a reminder from three days ago as if it were now, and
// the person, for whom a prompt typing itself at a session is otherwise
// indistinguishable from one typed on time.
//
// The threshold is scheduleTick, because that is the resolution the whole
// feature has: a prompt found on the very next pass is on time by every measure
// this package can take, and announcing that second would put a notice in front
// of every prompt lich ever delivers.
func lateNotice(at, now int64) string {
	late := time.Duration(now-at) * time.Second
	if late <= scheduleTick {
		return ""
	}
	return fmt.Sprintf("[lich] Scheduled for %s, delivered %s late.\n\n",
		time.Unix(at, 0).Format(scheduleClock), lateBy(late))
}

// lateBy words how far past its time a prompt is, in one unit — "45m", "3h",
// "2d". The same three rungs the card counts down on (frontend, timeUntil): the
// reader wants the scale, and a prompt that is two days late is not helped by
// the minutes on the end of it.
func lateBy(d time.Duration) string {
	switch {
	case d < time.Hour:
		return fmt.Sprintf("%dm", int(d.Round(time.Minute)/time.Minute))
	case d < 24*time.Hour:
		return fmt.Sprintf("%dh", int(d.Round(time.Hour)/time.Hour))
	default:
		return fmt.Sprintf("%dd", int(d.Round(24*time.Hour)/(24*time.Hour)))
	}
}
