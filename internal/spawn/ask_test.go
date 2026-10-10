package spawn

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/omartelo/lich/internal/terminal"
)

func TestAskHandsTheModTheQuestionAndReturnsTheAnswer(t *testing.T) {
	svc, term := newControlService(t)
	term.outcome = terminal.ModOutcome{ID: "m1", State: terminal.ModAcked, OK: true, Answer: "Fixing the login test."}
	got, err := svc.Ask(context.Background(), AskOptions{From: "s1", Target: "auth-fix", Question: "what are you on?"})
	if err != nil {
		t.Fatalf("Ask: %v", err)
	}
	want := terminal.ModCommand{Kind: terminal.ModAsk, Question: "what are you on?"}
	if len(term.ran) != 1 || term.ran[0] != want || term.ranOn[0] != "s2" {
		t.Fatalf("ran %+v on %v, want %+v on s2", term.ran, term.ranOn, want)
	}
	if got != (Answered{ID: "s2", Project: "lich", Label: "auth-fix", Answer: "Fixing the login test."}) {
		t.Fatalf("Ask = %+v", got)
	}
	if left := time.Until(term.deadline); left <= ControlWaitLimit || left > AskWaitLimit {
		t.Fatalf("waited with %v left, want up to %v", left, AskWaitLimit)
	}
}

func TestAskRefusesBeforeReachingTheSession(t *testing.T) {
	tests := []struct{ name, target, want string }{
		{"another CLI", "codex-run", `"codex-run" runs Codex, and only a Claude Code session can be asked`},
		{"the caller itself", "Session 3", "a session cannot ask itself"},
		{"no target", " ", "name the session to ask"},
		{"a target nobody has", "nobody", "no session named"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			svc, term := newControlService(t)
			_, err := svc.Ask(context.Background(), AskOptions{From: "s1", Target: tc.target, Question: "why?"})
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("err = %v, want one saying %q", err, tc.want)
			}
			if len(term.ran) != 0 {
				t.Fatalf("the refusal still reached the session: %+v", term.ran)
			}
		})
	}
}

func TestAskFailsEveryWayWithoutAnAnswer(t *testing.T) {
	tests := []struct {
		name string
		out  terminal.ModOutcome
		want string
	}{
		{"never taken", terminal.ModOutcome{State: terminal.ModWithdrawn},
			`"auth-fix" did not take the question within 90 seconds`},
		{"taken, not answered in time", terminal.ModOutcome{State: terminal.ModDelivered},
			"An answer that comes later is dropped"},
		{"the session ended", terminal.ModOutcome{State: terminal.ModEnded}, `"auth-fix" ended before answering`},
		{"no conversation yet", terminal.ModOutcome{State: terminal.ModAcked, Error: "nothing-to-fork"},
			"no conversation to answer from yet"},
		{"the API failed", terminal.ModOutcome{State: terminal.ModAcked, Error: "api-error 529 overloaded"},
			`"auth-fix" could not answer: api-error 529 overloaded`},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			svc, term := newControlService(t)
			term.outcome = tc.out
			_, err := svc.Ask(context.Background(), AskOptions{From: "s1", Target: "auth-fix", Question: "why?"})
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("err = %v, want one saying %q", err, tc.want)
			}
		})
	}
}

func TestAskPassesTheTerminalsRefusalOn(t *testing.T) {
	svc, term := newControlService(t)
	term.runErr = errors.New("an ask needs a question, and none was given")
	_, err := svc.Ask(context.Background(), AskOptions{From: "s1", Target: "auth-fix", Question: " "})
	if !errors.Is(err, term.runErr) || !strings.HasPrefix(err.Error(), `"auth-fix": `) {
		t.Fatalf("err = %v, want the terminal's, naming the session", err)
	}
}
