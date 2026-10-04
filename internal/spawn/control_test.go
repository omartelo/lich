package spawn

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/omartelo/lich/internal/store"
	"github.com/omartelo/lich/internal/terminal"
)

// newControlService is a workspace where the caller s1 and the target auth-fix
// run Claude Code, and codex-run runs another CLI.
func newControlService(t *testing.T) (*Service, *fakeTerminal) {
	t.Helper()
	svc, sessions, _, term, _ := newService(t)
	sessions.projects = []store.Project{{ID: "p1", Name: "lich", Path: "/src/lich", Sessions: []store.Session{
		{ID: "s1", Label: "Session 3", Kind: "claude"},
		{ID: "s2", Label: "auth-fix", Kind: "claude"},
		{ID: "s3", Label: "codex-run", Kind: "codex"},
	}}}
	term.outcome = terminal.ModOutcome{ID: "m1", State: terminal.ModAcked, OK: true}
	return svc, term
}

func TestControlHandsTheModOneCommandPerAction(t *testing.T) {
	tests := []struct {
		action, value, args string
		want                terminal.ModCommand
	}{
		{"prompt", "run the tests", "", terminal.ModCommand{Kind: terminal.ModPrompt, Text: "run the tests"}},
		{"abort", "", "", terminal.ModCommand{Kind: terminal.ModAbort}},
		{"model", "opus", "", terminal.ModCommand{Kind: terminal.ModModel, Model: "opus"}},
		{"model", "", "", terminal.ModCommand{Kind: terminal.ModModel}},
		{"effort", "high", "", terminal.ModCommand{Kind: terminal.ModEffort, Effort: "high"}},
		{"effort", "", "", terminal.ModCommand{Kind: terminal.ModEffort}},
		{"command", "compact", "keep the plan",
			terminal.ModCommand{Kind: terminal.ModRunCommand, Name: "compact", Args: "keep the plan"}},
	}
	for _, tc := range tests {
		t.Run(tc.action+" "+tc.value, func(t *testing.T) {
			svc, term := newControlService(t)
			got, err := svc.Control(context.Background(), "s1", "auth-fix", "", tc.action, tc.value, tc.args)
			if err != nil {
				t.Fatalf("Control: %v", err)
			}
			if len(term.ran) != 1 || term.ran[0] != tc.want || term.ranOn[0] != "s2" {
				t.Fatalf("ran %+v on %v, want %+v on s2", term.ran, term.ranOn, tc.want)
			}
			want := Controlled{ID: "s2", Project: "lich", Label: "auth-fix", Action: tc.action,
				Value: tc.value, CommandID: "m1", State: ControlDone}
			if got != want {
				t.Fatalf("Control = %+v, want %+v", got, want)
			}
		})
	}
}

func TestControlRefusesBeforeReachingTheSession(t *testing.T) {
	tests := []struct {
		name, from, target, action, value, args, want string
	}{
		{"another CLI", "s1", "codex-run", "abort", "", "", "only a Claude Code session"},
		{"the caller itself", "s1", "Session 3", "abort", "", "", "cannot control itself"},
		{"a value on an abort", "s1", "auth-fix", "abort", "now", "", "takes nothing after it"},
		{"args on a prompt", "s1", "auth-fix", "prompt", "go", "x", "only a command takes arguments"},
		{"an empty prompt", "s1", "auth-fix", "prompt", " ", "", "needs its text"},
		{"an unknown action", "s1", "auth-fix", "compact", "", "", "the actions are prompt, abort"},
		{"no target", "s1", " ", "abort", "", "", "name the session to control"},
		{"a target nobody has", "s1", "nobody", "abort", "", "", "no session named"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			svc, term := newControlService(t)
			_, err := svc.Control(context.Background(), tc.from, tc.target, "", tc.action, tc.value, tc.args)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("err = %v, want one saying %q", err, tc.want)
			}
			if len(term.ran) != 0 {
				t.Fatalf("the refusal still reached the session: %+v", term.ran)
			}
		})
	}
}

func TestControlReportsHowFarTheCommandGot(t *testing.T) {
	for _, state := range []string{ControlDelivered, ControlQueued, ControlEnded} {
		t.Run(state, func(t *testing.T) {
			svc, term := newControlService(t)
			term.outcome = terminal.ModOutcome{ID: "m4", State: state}
			got, err := svc.Control(context.Background(), "s1", "auth-fix", "", "abort", "", "")
			if err != nil {
				t.Fatalf("Control: %v", err)
			}
			if got.State != state || got.CommandID != "m4" {
				t.Fatalf("Control = %+v, want state %s with the command's id", got, state)
			}
		})
	}
}

func TestControlNamesTheSessionThatRefused(t *testing.T) {
	svc, term := newControlService(t)
	term.outcome = terminal.ModOutcome{ID: "m1", State: terminal.ModAcked, Error: "no turn is running"}
	_, err := svc.Control(context.Background(), "s1", "auth-fix", "", "abort", "", "")
	if err == nil || err.Error() != `"auth-fix" refused the abort: no turn is running` {
		t.Fatalf("err = %v", err)
	}
}

func TestControlPassesTheTerminalsRefusalOn(t *testing.T) {
	svc, term := newControlService(t)
	term.runErr = errors.New("the session cannot take commands")
	if _, err := svc.Control(context.Background(), "s1", "auth-fix", "", "abort", "", ""); !errors.Is(err, term.runErr) {
		t.Fatalf("err = %v, want the terminal's", err)
	}
}

// The wait is bounded, and a slash command, which runs only once the session
// is idle, gets the longer bound.
func TestControlBoundsTheWait(t *testing.T) {
	waited := func(action, value string) time.Duration {
		svc, term := newControlService(t)
		if _, err := svc.Control(context.Background(), "s1", "auth-fix", "", action, value, ""); err != nil {
			t.Fatalf("Control %s: %v", action, err)
		}
		if term.deadline.IsZero() {
			t.Fatalf("%s waited with no deadline", action)
		}
		// Measured from after the call, so it is never longer than the bound.
		return time.Until(term.deadline)
	}
	prompt, command := waited("prompt", "go"), waited("command", "clear")
	if command > ControlWaitLimit || prompt > ControlWaitLimit {
		t.Fatalf("waits %v and %v, want both within %v", prompt, command, ControlWaitLimit)
	}
	if command <= prompt {
		t.Fatalf("a slash command waits %v, no longer than a prompt's %v", command, prompt)
	}
}
