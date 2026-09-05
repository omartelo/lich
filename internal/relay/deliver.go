package relay

import (
	"fmt"
	"log/slog"
	"time"
)

// deliver puts message at a session's prompt and sends it — two writes, a beat
// apart, because the Enter has to arrive after the prompt has taken the paste
// (see defaultSubmitDelay). It waits for a prompt that is free first: this
// Enter sends everything on the line, including whatever the person at that
// session had started typing.
func (s *Service) deliver(sessionID, message string) error {
	if err := s.awaitFree(sessionID); err != nil {
		return err
	}
	if err := s.term.Write(sessionID, paste(message)); err != nil {
		return err
	}
	s.awaitSettled(sessionID)
	return s.term.Write(sessionID, submit)
}

// awaitSettled holds the Enter back until the target has finished taking the
// paste in — until its PTY has been quiet for submitDelay, not until
// submitDelay has passed since lich wrote.
//
// The two are the same thing only where the paste arrives as one event. They
// are not on Windows: ConPTY hands a child key events rather than the bytes
// written to it, and a bracketed paste is not a key, so the markers are dropped
// and the message reaches the TUI as a stream of typed characters. Every
// provider TUI has a heuristic for that — Codex suppresses Enter for 120ms
// after the last character of a burst, so it lands as a newline inside the
// paste instead of sending it — and the stream is still arriving when a write
// on this side has long returned. The message sat unsent at the target's
// prompt, which is where users found it.
//
// The quiet is the drain: a TUI redraws as it takes the characters in, so its
// silence is the first moment nothing more is coming. Waiting for it costs
// nothing where the paste was one event — the terminal was already quiet — and
// costs exactly the drain where it was not.
//
// Bounded because a TUI that repaints on a timer of its own would never go
// quiet, and an Enter that is late is better than one that never comes.
func (s *Service) awaitSettled(sessionID string) {
	deadline := s.now().Add(s.settleLimit)
	for {
		time.Sleep(s.submitDelay)
		if s.term.QuietFor(sessionID) >= s.submitDelay || !s.now().Before(deadline) {
			return
		}
	}
}

// awaitFree blocks while a session's prompt belongs to somebody else — the
// checkout's setup script, or the user mid-sentence at it, both of which
// terminal.Ready answers. Every write this package makes goes through deliver,
// so this one gate covers the task, the nudge and the retry alike; a check at
// each call site would be three places to forget it in and would still leave
// the window between the check and the write open.
//
// It is bounded by the same budget a queued delivery gets and, in practice,
// cannot spend it: unsent input goes stale on its own well inside that
// (terminal.draftIdle), so somebody who walked away mid-word costs a delivery
// its delay, never its outcome.
func (s *Service) awaitFree(sessionID string) error {
	deadline := s.now().Add(s.deliveryLimit)
	for !s.term.Ready(sessionID) {
		if !s.term.Live(sessionID) {
			return fmt.Errorf("session stopped before its prompt was free")
		}
		if s.now().After(deadline) {
			return fmt.Errorf("prompt was still not free after %s", s.deliveryLimit)
		}
		time.Sleep(readyPoll)
	}
	return nil
}

// awaitReady blocks until a target's agent is the program reading its PTY, and
// errors when that will never happen: the session stopped, or it is still not
// at a prompt by deadline.
//
// A session opened on a fresh worktree runs the project's setup script in that
// PTY first, and the script can take minutes. It is live the whole time, and a
// message typed into it reaches `pnpm install`, which reads and discards it —
// so the request never existed, and the sender waits out a ticket nobody was
// asked to answer. Seen on the first real run of a worktree session.
//
// What comes back is a cause for a log line and for the status the sender is
// given, not prose for a person: whoever reads about this reads it in the
// window or at an agent's prompt, where internal/cli words it.
func (s *Service) awaitReady(dest candidate, deadline time.Time) error {
	for {
		time.Sleep(readyPoll)
		if s.term.Ready(dest.ID) {
			return nil
		}
		if !s.term.Live(dest.ID) {
			return fmt.Errorf("%q stopped before its agent started", dest.Peer.Label)
		}
		if s.now().After(deadline) {
			return fmt.Errorf(
				"%q was still not at a prompt after %s: whatever holds that terminal — "+
					"the project's worktree setup script, a provider that never came up — "+
					"outlasted the wait",
				dest.Peer.Label, s.deliveryLimit,
			)
		}
	}
}

// reportsState is whether a provider tells lich what it is doing, which is what
// makes its silence mean something (docs/hooks/session-state.md).
func (s *Service) reportsState(kind string) bool {
	return s.plugins != nil && s.plugins.Installed(kind)
}

// offersTools is whether the target can answer with a tool rather than with the
// shell command. Naming a tool a session does not have is worse than naming the
// command, which works everywhere — so an unknown answer is no.
func (s *Service) offersTools(kind string) bool {
	return s.plugins != nil && s.plugins.HasTools(kind)
}

// watchReceipt closes a ticket whose task nobody ever picked up.
//
// Delivery has always been provable — the bytes reached the PTY — and receipt
// never was. Between the two sits everything that can be on a terminal instead
// of a prompt: Claude Code asking whether a new directory is trusted, a
// provider still taking over the tty, a dialog left open by whoever was here
// last. The task is typed into that, and it is gone. Nothing fails: the sender
// waits out a ticket nobody was asked to answer, and finds out an hour later
// when it expires, which is how this feature's own first users found it.
//
// An agent that reads a prompt reports UserPromptSubmit within a second or two,
// so the absence of that report inside the window is the answer. It is only
// asked of providers that report at all — the rest have always been silent, and
// silence has to mean something to be read as anything.
//
// Before the errand is written off, the task is typed in once more. The
// commonest thing under an unread write is a provider that was still taking
// over the tty when Ready's quiet heuristic cleared — opencode's startup pauses
// long enough mid-splash to pass for a prompt, then flushes what was typed at
// it — and by the time the window has run out that same provider has been
// sitting at its real prompt for most of it. The second write is the manual
// resend automated, it carries the same ticket, and it costs nothing new: a
// terminal that swallows it was going to be reported unread anyway, and one
// mid-dialog got the first write already.
func (s *Service) watchReceipt(id string, t *ticket, kind, message string) {
	timer := time.NewTimer(s.receiptWindow)
	defer timer.Stop()
	select {
	case <-t.done:
		return
	case <-t.stalled:
		return
	case <-timer.C:
	}

	s.mu.Lock()
	current, live := s.tickets[id]
	if !live || current != t || t.sawBusy {
		s.mu.Unlock()
		return
	}
	if !t.redelivered && s.term.Ready(t.targetID) {
		t.redelivered = true
		s.mu.Unlock()
		slog.Warn("relay: task was typed and nothing read it, typing it again", "target", t.target)
		if err := s.handOff(id, t, kind, message); err == nil {
			return
		}
		// The second write was refused — the session died under it. Nothing read
		// the task, which is what unread says.
		s.mu.Lock()
		if current, live = s.tickets[id]; !live || current != t || t.sawBusy {
			s.mu.Unlock()
			return
		}
	}
	delete(s.tickets, id)
	close(t.unread)
	unattended := t.attended == 0
	s.mu.Unlock()

	s.clear(t)
	if unattended {
		s.stash(id, t, StatusUnread, "")
	}
}
