package cli

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/omartelo/lich/internal/spawn"
)

func answered(answer string) string {
	out, _ := json.Marshal(spawn.Answered{ID: "s2", Project: "lich", Label: "auth-fix", Answer: answer})
	return string(out)
}

func TestAskPostsTheQuestionAndPrintsTheAnswer(t *testing.T) {
	f := newFakeLich(t, answered("Fixing the login test."))
	code, stdout, stderr := run(t, f, "ask", "--project", "lich", "auth-fix", "what", "are you on?")
	if code != 0 {
		t.Fatalf("exit = %d, stderr = %q", code, stderr)
	}
	call := f.only(t)
	if call.method != "spawn.Ask" {
		t.Errorf("method = %q", call.method)
	}
	want := spawn.AskOptions{From: "s1", Target: "auth-fix", Project: "lich", Question: "what are you on?"}
	if got := optionsOf[spawn.AskOptions](t, call); got != want {
		t.Errorf("options = %+v, want %+v", got, want)
	}
	if stdout != "Fixing the login test.\n" {
		t.Errorf("stdout = %q", stdout)
	}
}

func TestAskJSON(t *testing.T) {
	f := newFakeLich(t, answered("Fixing it."))
	code, stdout, _ := run(t, f, "ask", "--json", "auth-fix", "status?")
	if code != 0 {
		t.Fatalf("exit = %d", code)
	}
	var got map[string]any
	if err := json.Unmarshal([]byte(stdout), &got); err != nil {
		t.Fatalf("stdout %q is not JSON: %v", stdout, err)
	}
	if got["answer"] != "Fixing it." || got["label"] != "auth-fix" {
		t.Errorf("json = %v", got)
	}
}

func TestAskRefusesAMalformedCall(t *testing.T) {
	f := newFakeLich(t, `null`)
	if code, _, stderr := run(t, f, "ask", "auth-fix"); code != 1 || !strings.Contains(stderr, "usage: lich ask") {
		t.Errorf("exit = %d, stderr %q; want 1 with the usage line", code, stderr)
	}
	if len(f.calls) != 0 {
		t.Errorf("a malformed call reached the app: %+v", f.calls)
	}
}

func TestAskReportsTheRefusal(t *testing.T) {
	f := newFakeLich(t, `{"error":"\"Session 3\" is this session, and a session cannot ask itself"}`)
	f.status = 400
	code, stdout, stderr := run(t, f, "ask", "Session 3", "why?")
	if code != 1 || stdout != "" || !strings.Contains(stderr, "cannot ask itself") {
		t.Fatalf("exit = %d, stdout = %q, stderr = %q", code, stdout, stderr)
	}
}

func TestMCPAskSessionAnswersWithTheAnswer(t *testing.T) {
	f := newFakeLich(t, answered("Fixing the login test."))
	replies := speak(t, f, `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":
		{"name":"ask_session","arguments":{"session":"auth-fix","question":"what are you on?"}}}`)

	text, failed := textOf(t, replies[0])
	if failed || text != "Fixing the login test." {
		t.Fatalf("text = %q, failed = %v", text, failed)
	}
	want := spawn.AskOptions{From: "s1", Target: "auth-fix", Question: "what are you on?"}
	if got := optionsOf[spawn.AskOptions](t, f.only(t)); got != want {
		t.Errorf("options = %+v, want %+v", got, want)
	}
}
