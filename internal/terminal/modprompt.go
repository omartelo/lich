package terminal

import (
	"context"
	"errors"

	"github.com/omartelo/lich/internal/relay"
)

// SubmitPrompt queues text as a prompt for session id's Claude Code mod, with
// the wait on its ack, which says how far the prompt got once its context ends
// (relay.PromptReceipt), and whether a poll collected it yet. It answers
// relay.ErrNoMod for a session no mod can collect it from: the transport is
// down, the session is not running, or no mod polls from it. The relay is its
// one caller, and it does none of the relay's ticket accounting.
//
// The wait is a closure because the ack can land before anyone waits on it,
// and only the waiter queued with the prompt keeps it (modQueue.await).
func (s *Service) SubmitPrompt(id, text string) (relay.HandedPrompt, error) {
	if s.ws == nil {
		return relay.HandedPrompt{}, relay.ErrNoMod
	}
	w, err := s.queueModCommand(id, ModCommand{Kind: ModPrompt, Text: text})
	if errors.Is(err, errModDetached) || errors.Is(err, errModNotRunning) {
		return relay.HandedPrompt{}, relay.ErrNoMod
	}
	if err != nil {
		return relay.HandedPrompt{}, err
	}
	mods := &s.ws.mods
	return relay.HandedPrompt{
		Await: func(ctx context.Context) relay.PromptReceipt {
			out := mods.await(ctx, w)
			return relay.PromptReceipt{State: out.State, OK: out.OK, Reason: out.Error}
		},
		Collected: func() bool { return mods.collected(w) },
	}, nil
}

// ModAttached is whether a Claude Code mod polled from running session id
// within the attach window, which is what SubmitPrompt queues for.
func (s *Service) ModAttached(id string) bool {
	return s.ws != nil && s.Live(id) && s.ws.mods.attached(id)
}

func (q *modQueue) collected(w *modWaiter) bool {
	q.mu.Lock()
	defer q.mu.Unlock()
	return w.delivered
}

func (q *modQueue) attached(id string) bool {
	q.mu.Lock()
	defer q.mu.Unlock()
	seen, ok := q.lastSeen[id]
	return ok && q.now().Sub(seen) <= q.attachWindow()
}
