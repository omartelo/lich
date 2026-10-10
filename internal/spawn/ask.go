package spawn

import (
	"context"
	"fmt"
	"time"

	"github.com/omartelo/lich/internal/terminal"
)

// AskWaitLimit is how long Ask waits for an answer: the 90 seconds an MCP call
// may block (cli.mcpMaxWait). A brief answer, which the mod asks for, takes
// seconds; one of thousands of words was measured at 109 and does not fit.
const AskWaitLimit = 90 * time.Second

// modNothingToFork is the reason a mod acks an ask with when the session has
// no conversation yet (docs/hooks/mod-control.md).
const modNothingToFork = "nothing-to-fork"

// Answered is a session's answer to a side question.
type Answered struct {
	ID      string `json:"id"`
	Project string `json:"project"`
	Label   string `json:"label"`
	Answer  string `json:"answer"`
}

// AskOptions is one Ask call, an object for the reason CloseOptions is one.
type AskOptions struct {
	From     string `json:"from"`
	Target   string `json:"target"`
	Project  string `json:"project"`
	Question string `json:"question"`
}

// Ask puts a side question to a running Claude Code session and waits for its
// answer. The session's mod answers it with a tool-less fork of the session's
// own conversation: the session's turn goes on undisturbed and the question
// never enters its transcript. Every way it ends without an answer is an
// error naming the session, a wait that runs out included: an answer that lands
// later is dropped.
func (s *Service) Ask(ctx context.Context, opts AskOptions) (Answered, error) {
	fromID, target, projectName, question := opts.From, opts.Target, opts.Project, opts.Question
	found, err := s.claudeTarget(target, projectName, "ask", "asked")
	if err != nil {
		return Answered{}, err
	}
	label := found.session.Label
	if found.session.ID == fromID {
		return Answered{}, fmt.Errorf(
			"%q is this session, and a session cannot ask itself: the question would ride the "+
				"request it is waiting on, which the answer never sees", label)
	}

	wait, cancel := context.WithTimeout(ctx, AskWaitLimit)
	defer cancel()
	out, err := s.term.RunModCommand(wait, found.session.ID,
		terminal.ModCommand{Kind: terminal.ModAsk, Question: question})
	if err != nil {
		return Answered{}, fmt.Errorf("%q: %w", label, err)
	}
	if err := unanswered(label, out); err != nil {
		return Answered{}, err
	}
	return Answered{
		ID: found.session.ID, Project: found.project.Name, Label: label, Answer: out.Answer,
	}, nil
}

func unanswered(label string, out terminal.ModOutcome) error {
	limit := int(AskWaitLimit / time.Second)
	switch {
	case out.State == terminal.ModWithdrawn:
		return fmt.Errorf(
			"%q did not take the question within %d seconds, so lich withdrew it: its mod stopped "+
				"polling (Claude Code frozen, or the mod reloading). It is safe to ask again", label, limit)
	case out.State == terminal.ModDelivered:
		return fmt.Errorf(
			"%q took the question and did not answer within %d seconds: the answer is long, or its "+
				"mod reloaded while answering. An answer that comes later is dropped", label, limit)
	case out.State == terminal.ModEnded:
		return fmt.Errorf("%q ended before answering", label)
	case out.OK:
		return nil
	case out.Error == modNothingToFork:
		return fmt.Errorf("%q has no conversation to answer from yet: it is new, or was just cleared", label)
	default:
		return fmt.Errorf("%q could not answer: %s", label, out.Error)
	}
}
