package relay

import (
	"strings"
	"testing"
)

func TestPromptPastesTheTextAndSendsIt(t *testing.T) {
	term := newFakeTerminal("s1")
	svc := newRelay(fakeSessions{}, term, nil)

	if err := svc.Prompt("s1", "fix the auth redirect\nwith a test"); err != nil {
		t.Fatalf("Prompt: %v", err)
	}

	writes := term.writesTo("s1")
	if len(writes) != 2 || writes[0] != paste("fix the auth redirect\nwith a test") || writes[1] != submit {
		t.Fatalf("writes to s1 = %q, want the paste and then the Enter", writes)
	}
}

// The race's task is the user's own words, so nothing is wrapped around it: no
// sender line, no ticket to reply to.
func TestPromptTypesNoEnvelope(t *testing.T) {
	term := newFakeTerminal("s1")
	svc := newRelay(fakeSessions{}, term, nil)

	if err := svc.Prompt("s1", "ship it"); err != nil {
		t.Fatalf("Prompt: %v", err)
	}

	if got := term.written("s1"); strings.Contains(got, "[lich]") || strings.Contains(got, "ticket") {
		t.Fatalf("typed at s1 = %q, want the prompt alone", got)
	}
}

// ESC would end the bracketed paste early and run the rest as keystrokes.
func TestPromptStripsControlCharacters(t *testing.T) {
	term := newFakeTerminal("s1")
	svc := newRelay(fakeSessions{}, term, nil)

	if err := svc.Prompt("s1", "a\x1b[201~b"); err != nil {
		t.Fatalf("Prompt: %v", err)
	}

	if got := term.writesTo("s1")[0]; got != paste("a[201~b") {
		t.Fatalf("pasted %q, want the escape stripped", got)
	}
}

func TestPromptRefusesAnEmptyPrompt(t *testing.T) {
	term := newFakeTerminal("s1")
	svc := newRelay(fakeSessions{}, term, nil)

	if err := svc.Prompt("s1", " \n\x1b "); err == nil {
		t.Fatal("Prompt of blanks = nil error, want a refusal")
	}
	if got := term.writesTo("s1"); len(got) != 0 {
		t.Fatalf("writes to s1 = %q, want none", got)
	}
}

func TestPromptRefusesAPromptOverTheLimit(t *testing.T) {
	term := newFakeTerminal("s1")
	svc := newRelay(fakeSessions{}, term, nil)

	if err := svc.Prompt("s1", strings.Repeat("x", promptLimit+1)); err == nil {
		t.Fatal("Prompt over the limit = nil error, want a refusal")
	}
	if got := term.writesTo("s1"); len(got) != 0 {
		t.Fatalf("writes to s1 = %q, want none", got)
	}
}

func TestPromptFailsForASessionThatIsGone(t *testing.T) {
	svc := newRelay(fakeSessions{}, newFakeTerminal(), nil)

	if err := svc.Prompt("gone", "ship it"); err == nil {
		t.Fatal("Prompt to a session with no PTY = nil error, want a failure")
	}
}
