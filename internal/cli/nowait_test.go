package cli

import (
	"bytes"
	"strings"
	"testing"
)

// Collecting without waiting is how a sender busy in a turn of its own looks in
// at a decision point: the nudge only lands between turns, and holding the line
// would stall the work.

func TestWaitNoWaitCollectsWhatIsReadyWithoutHoldingTheLine(t *testing.T) {
	f := newFakeLich(t, `{"results":[],"open":["docs"]}`)

	code, stdout, _ := run(t, f, "wait", "--no-wait")
	if code != ExitPending {
		t.Fatalf("exit = %d, want %d: nothing ready while an errand is open", code, ExitPending)
	}
	call := f.only(t)
	if call.method != "relay.CollectNow" {
		t.Fatalf("method = %q, want relay.CollectNow", call.method)
	}
	if len(call.args) != 1 || call.args[0] != "s1" {
		t.Errorf("args = %v, want the session alone", call.args)
	}
	if !strings.Contains(stdout, `Still working: "docs"`) {
		t.Errorf("stdout = %q, want who still owes", stdout)
	}
}

func TestWaitNoWaitTakesNoTicket(t *testing.T) {
	f := newFakeLich(t, `null`)

	code, _, stderr := run(t, f, "wait", "--no-wait", "a1b2c3d4")
	if code != 1 || !strings.Contains(stderr, "usage: lich wait") {
		t.Fatalf("exit = %d, stderr = %q, want the usage", code, stderr)
	}
	if len(f.calls) != 0 {
		t.Errorf("a malformed command reached the app: %+v", f.calls)
	}
}

func TestMCPWaitNoWaitCollectsWhatIsReady(t *testing.T) {
	f := newFakeLich(t, `{"results":[{"ticket":"t1","target":"auth","status":"answered","answer":"all green"}],"open":[]}`)

	replies := speak(t, f, `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":
		{"name":"wait_for_answer","arguments":{"no_wait":true}}}`)

	text, failed := textOf(t, replies[0])
	if failed {
		t.Fatalf("tool reported a failure: %s", text)
	}
	if call := f.only(t); call.method != "relay.CollectNow" {
		t.Fatalf("method = %q, want relay.CollectNow", call.method)
	}
	if !strings.Contains(text, "all green") {
		t.Errorf("text = %q, want the ready answer", text)
	}
}

func TestMCPWaitNoWaitRefusesATicket(t *testing.T) {
	f := newFakeLich(t, `null`)

	replies := speak(t, f, `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":
		{"name":"wait_for_answer","arguments":{"no_wait":true,"ticket":"a1b2c3d4"}}}`)

	text, failed := textOf(t, replies[0])
	if !failed || !strings.Contains(text, "takes no ticket") {
		t.Fatalf("text = %q (failed %v), want the refusal", text, failed)
	}
	if len(f.calls) != 0 {
		t.Errorf("a refused call reached the app: %+v", f.calls)
	}
}

// The instructions are what an orchestrator reads before its first turn: they
// have to name the no-wait look, where to take it, and that looking again in a
// loop is the polling they forbid.
func TestMCPInstructionsTellWhenToLookWithoutWaiting(t *testing.T) {
	for _, want := range []string{"no_wait", "before delegating more", "before the final synthesis", "in a loop"} {
		if !strings.Contains(mcpInstructions, want) {
			t.Errorf("instructions are missing %q", want)
		}
	}
}

func TestCollectNowOverTheRealDispatcher(t *testing.T) {
	env, term, _, _ := wiredRelay(t)
	openErrand(t, env, term)

	var stdout, stderr bytes.Buffer
	code := Run([]string{"wait", "--no-wait"}, "test", env, &stdout, &stderr)
	if code != ExitPending {
		t.Fatalf("exit = %d, stderr = %q", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), `Still working: "docs"`) {
		t.Errorf("stdout = %q, want the open errand", stdout.String())
	}
}
