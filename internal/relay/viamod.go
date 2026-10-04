package relay

import (
	"context"
	"errors"
	"fmt"
	"time"
)

// ErrNoMod is what Terminal.SubmitPrompt answers for a session no mod polls
// from, which is every session but a Claude Code one running lich-plugin's mod.
var ErrNoMod = errors.New("no lich-plugin mod polls from this session")

// How far a prompt handed to a mod got (docs/hooks/mod-control.md).
const (
	PromptWithdrawn = "withdrawn" // no poll collected it, and it never runs
	PromptDelivered = "delivered" // a poll carried it, and no ack came back yet
	PromptAcked     = "acked"     // the mod acked it: OK and Reason say how it went
	PromptEnded     = "ended"     // the session exited first
)

// PromptReceipt is what became of one prompt handed to a mod. An ack OK means
// its turn started.
type PromptReceipt struct {
	State  string
	OK     bool
	Reason string
}

// defaultModAckWait bounds the wait on a mod's ack for a delivery that carries
// no ticket. An idle session acks within a second or two, once its turn
// started; past this the session is not idle, and the prompt runs once it is.
const defaultModAckWait = 10 * time.Second

// handToMod hands a message to the session's mod, and reports whether it is
// the mod's to deliver now: false with no error means type it instead, which
// is what a session with no mod and a prompt no poll collected both want.
func (s *Service) handToMod(sessionID, message string) (bool, error) {
	await, err := s.term.SubmitPrompt(sessionID, message)
	if errors.Is(err, ErrNoMod) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), s.modAckWait)
	defer cancel()
	receipt := await(ctx)
	switch receipt.State {
	case PromptWithdrawn:
		return false, nil
	case PromptEnded:
		return true, errors.New("the session ended before its mod took it")
	case PromptAcked:
		if !receipt.OK {
			return true, fmt.Errorf("its mod refused it: %s", receipt.Reason)
		}
	}
	return true, nil
}

// watchModReceipt follows a task handed to the target's mod until its turn
// starts, the errand ends, or the receipt window runs out.
//
// Delivery through the mod is at most once, so a task is typed only when no
// poll collected it. One a poll collected and nobody acked may still run, and
// is never typed again: on an idle target that reports state it is closed
// unread, with a late answer still taken; on a busy one it waits behind the
// running turn, and turn accounting takes it from there.
func (s *Service) watchModReceipt(
	id string, t *ticket, kind, message string, busy bool, await func(context.Context) PromptReceipt,
) {
	ctx, cancel := s.errandWindow(t)
	receipt := await(ctx)
	cancel()
	switch receipt.State {
	case PromptAcked:
		if !receipt.OK {
			// Never typed: the composer runs the same UserPromptSubmit hooks.
			s.failDelivery(id, t, fmt.Errorf("a hook at %q dropped it: %s", t.target, receipt.Reason))
		}
	case PromptWithdrawn:
		s.typeWithdrawn(id, t, kind, message)
	default:
		if !busy && s.reportsState(kind) {
			s.closeModUnread(id, t)
		}
	}
}

// errandWindow is a receipt window that also closes when the errand ends, so a
// wait on its ack does not outlive it.
func (s *Service) errandWindow(t *ticket) (context.Context, context.CancelFunc) {
	ctx, cancel := context.WithTimeout(context.Background(), s.receiptWindow)
	go func() {
		select {
		case <-t.done:
		case <-t.stalled:
		case <-t.unread:
		case <-t.undelivered:
		case <-ctx.Done():
		}
		cancel()
	}()
	return ctx, cancel
}

// typeWithdrawn types a task its mod never collected, if its errand is still
// open. It is stamped again because the target may have started a turn since.
func (s *Service) typeWithdrawn(id string, t *ticket, kind, message string) {
	s.mu.Lock()
	current, live := s.tickets[id]
	s.mu.Unlock()
	if !live || current != t {
		return
	}
	busy := s.stampDelivery(t)
	if err := s.typeTask(id, t, kind, message, busy); err != nil {
		s.failDelivery(id, t, err)
	}
}

// closeModUnread closes an errand whose task its mod collected and whose target
// never started on it.
func (s *Service) closeModUnread(id string, t *ticket) {
	s.mu.Lock()
	current, live := s.tickets[id]
	if !live || current != t || t.sawBusy {
		s.mu.Unlock()
		return
	}
	delete(s.tickets, id)
	close(t.unread)
	s.lapseLocked(id, t, StatusUnread)
	unattended := t.attended == 0
	if unattended {
		s.stashLocked(id, t, StatusUnread, "")
	}
	s.mu.Unlock()

	s.clear(t)
	if unattended {
		s.announceInbox(t.fromID)
	}
}
