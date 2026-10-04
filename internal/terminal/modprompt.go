package terminal

import (
	"context"
	"errors"

	"github.com/omartelo/lich/internal/relay"
)

// SubmitPrompt queues text as a prompt for session id's Claude Code mod and
// returns the wait on its ack, which says how far the prompt got once its
// context ends (relay.PromptReceipt). It answers relay.ErrNoMod for a session
// no mod can collect it from: the transport is down, the session is not
// running, or no mod polls from it. The relay is its one caller, and it does
// none of the relay's ticket accounting.
//
// The wait is a closure because the ack can land before anyone waits on it,
// and only the waiter queued with the prompt keeps it (modQueue.await).
func (s *Service) SubmitPrompt(id, text string) (func(context.Context) relay.PromptReceipt, error) {
	if s.ws == nil {
		return nil, relay.ErrNoMod
	}
	w, err := s.queueModCommand(id, ModCommand{Kind: ModPrompt, Text: text})
	if errors.Is(err, errModDetached) || errors.Is(err, errModNotRunning) {
		return nil, relay.ErrNoMod
	}
	if err != nil {
		return nil, err
	}
	return func(ctx context.Context) relay.PromptReceipt {
		out := s.ws.mods.await(ctx, w)
		return relay.PromptReceipt{State: out.State, OK: out.OK, Reason: out.Error}
	}, nil
}
