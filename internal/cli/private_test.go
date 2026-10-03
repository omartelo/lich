package cli

import (
	"strings"
	"testing"
)

const privatePending = `{"ticket":"a1b2c3d4","target":"docs","status":"pending","private":true}`

func TestSendPrivateCallsSendPrivate(t *testing.T) {
	f := newFakeLich(t, privatePending)

	code, stdout, _ := run(t, f, "send", "--private", "docs", "long errand")
	if code != ExitPending {
		t.Fatalf("exit = %d, want %d", code, ExitPending)
	}
	call := f.only(t)
	if call.method != "relay.SendPrivate" || len(call.args) != 5 {
		t.Fatalf("call = %+v, want relay.SendPrivate with Send's five arguments", call)
	}
	// No note is coming, so the text must not promise one.
	if strings.Contains(stdout, "typed at the sending session's prompt when") {
		t.Errorf("a private errand promised a note:\n%s", stdout)
	}
	for _, want := range []string{"private", "lich wait a1b2c3d4"} {
		if !strings.Contains(stdout, want) {
			t.Errorf("output is missing %q:\n%s", want, stdout)
		}
	}
}

func TestOpenPrivateHandsOverPrivately(t *testing.T) {
	f := newFakeLich(t, openedBody)
	f.answers = map[string]answer{"relay.SendPrivate": {status: 200, body: privatePending}}

	code, _, stderr := run(t, f, "open", "--worktree", "auth-fix", "--prompt", "port the parser", "--private")
	if code != 0 {
		t.Fatalf("exit = %d, stderr = %q", code, stderr)
	}
	if len(f.calls) != 2 || f.calls[1].method != "relay.SendPrivate" {
		t.Fatalf("calls = %+v, want spawn.Open then relay.SendPrivate", f.calls)
	}
}

func TestOpenPrivateWithoutAPromptIsRefusedBeforeOpening(t *testing.T) {
	f := newFakeLich(t, openedBody)

	code, _, stderr := run(t, f, "open", "--worktree", "auth-fix", "--private")
	if code == 0 {
		t.Fatal("opened a session for a private hand-off that had no task")
	}
	if len(f.calls) != 0 {
		t.Errorf("calls = %+v, want nothing opened", f.calls)
	}
	if !strings.Contains(stderr, "--prompt") {
		t.Errorf("stderr = %q, want it to name --prompt", stderr)
	}
}

func TestMCPSendPrivateCallsSendPrivate(t *testing.T) {
	f := newFakeLich(t, privatePending)

	replies := speak(t, f, `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":
		{"name":"send_to_session","arguments":{"session":"docs","prompt":"long errand","private":true}}}`)

	text, failed := textOf(t, replies[0])
	if failed {
		t.Fatalf("tool reported a failure: %s", text)
	}
	if call := f.only(t); call.method != "relay.SendPrivate" {
		t.Errorf("method = %q, want relay.SendPrivate", call.method)
	}
	if strings.Contains(text, "a short note will arrive") {
		t.Errorf("a private errand promised a note: %s", text)
	}
	for _, want := range []string{"private", "a1b2c3d4", "wait_for_answer"} {
		if !strings.Contains(text, want) {
			t.Errorf("text is missing %q: %s", want, text)
		}
	}
}

func TestMCPOpenPrivateHandsOverPrivately(t *testing.T) {
	f := newFakeLich(t, openedBody)
	f.answers = map[string]answer{"relay.SendPrivate": {status: 200, body: privatePending}}

	replies := speak(t, f, `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":
		{"name":"open_session","arguments":{"worktree":"auth-fix","prompt":"port the parser","private":true}}}`)

	if text, failed := textOf(t, replies[0]); failed {
		t.Fatalf("tool reported a failure: %s", text)
	}
	if len(f.calls) != 2 || f.calls[1].method != "relay.SendPrivate" {
		t.Fatalf("calls = %+v, want spawn.Open then relay.SendPrivate", f.calls)
	}
}

func TestMCPOpenPrivateWithoutAPromptIsRefusedBeforeOpening(t *testing.T) {
	f := newFakeLich(t, openedBody)

	replies := speak(t, f, `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":
		{"name":"open_session","arguments":{"worktree":"auth-fix","private":true}}}`)

	text, failed := textOf(t, replies[0])
	if !failed {
		t.Fatalf("opened a session for a private hand-off that had no task: %s", text)
	}
	if len(f.calls) != 0 {
		t.Errorf("calls = %+v, want nothing opened", f.calls)
	}
}

// A private hand-off that failed must not be retried in the open: the recovery
// it names keeps the errand private.
func TestAFailedPrivateHandOffNamesAPrivateRetry(t *testing.T) {
	f := newFakeLich(t, openedBody)
	f.answers = map[string]answer{"relay.SendPrivate": {status: 500, body: "boom"}}

	_, _, stderr := run(t, f, "open", "--worktree", "auth-fix", "--prompt", "port the parser", "--private")
	if !strings.Contains(stderr, "lich send --private") {
		t.Errorf("stderr = %q, want the retry to keep --private", stderr)
	}

	f.calls = nil
	replies := speak(t, f, `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":
		{"name":"open_session","arguments":{"worktree":"auth-fix","prompt":"port the parser","private":true}}}`)
	if text, _ := textOf(t, replies[0]); !strings.Contains(text, "send_to_session with private") {
		t.Errorf("text = %q, want the retry to keep private", text)
	}
}
