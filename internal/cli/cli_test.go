package cli

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/omartelo/lich/internal/relay"
	"github.com/omartelo/lich/internal/singleton"
)

// recorded is one RPC the fake lich received.
type recorded struct {
	method string
	token  string
	args   []any
}

// fakeLich stands in for the running app: it answers /rpc/<method> with a
// canned body and records what was asked.
type fakeLich struct {
	server *httptest.Server
	calls  []recorded
	status int
	body   string
}

func newFakeLich(t *testing.T, body string) *fakeLich {
	t.Helper()
	f := &fakeLich{status: http.StatusOK, body: body}
	f.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		payload, _ := io.ReadAll(r.Body)
		var args []any
		_ = json.Unmarshal(payload, &args)
		f.calls = append(f.calls, recorded{
			method: strings.TrimPrefix(r.URL.Path, "/rpc/"),
			token:  r.URL.Query().Get("token"),
			args:   args,
		})
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(f.status)
		_, _ = io.WriteString(w, f.body)
	}))
	t.Cleanup(f.server.Close)
	return f
}

// env is the session environment lich exports into every PTY, pointed at the
// fake.
func (f *fakeLich) env(key string) string {
	switch key {
	case "LICH_PORT":
		return strconv.Itoa(f.server.Listener.Addr().(*net.TCPAddr).Port)
	case "LICH_TOKEN":
		return "tok en"
	case "LICH_SESSION_ID":
		return "s1"
	}
	return ""
}

func (f *fakeLich) only(t *testing.T) recorded {
	t.Helper()
	if len(f.calls) != 1 {
		t.Fatalf("got %d calls, want 1", len(f.calls))
	}
	return f.calls[0]
}

// run executes one command against a fake lich and returns its streams. It
// never goes through Run: that would resolve the runtime file of the lich
// actually running on this machine, and a test that reached it would deliver a
// message into a real session.
func run(t *testing.T, f *fakeLich, args ...string) (int, string, string) {
	t.Helper()
	var stdout, stderr bytes.Buffer
	code := dispatch(args, &client{env: f.env, stdout: &stdout, stderr: &stderr, running: noInstance})
	return code, stdout.String(), stderr.String()
}

// noInstance is a machine with no lich recorded — the fallback finds nothing,
// so only the environment can point a command anywhere.
func noInstance() (*singleton.Info, error) { return nil, nil }

func TestUnknownArgumentsAreNotACommand(t *testing.T) {
	f := newFakeLich(t, `null`)

	for _, args := range [][]string{{}, {"--"}, {"--", "--ozone-platform=wayland"}, {"nonsense"}} {
		if code, _, _ := run(t, f, args...); code != NotACommand {
			t.Errorf("Run(%q) = %d, want NotACommand", args, code)
		}
	}
	if len(f.calls) != 0 {
		t.Errorf("a non-command reached the app: %+v", f.calls)
	}
}

func TestHelpPrintsEveryCommand(t *testing.T) {
	f := newFakeLich(t, `null`)

	code, stdout, _ := run(t, f, "help")
	if code != 0 {
		t.Fatalf("exit = %d", code)
	}
	for _, want := range []string{"lich sessions", "lich send", "lich wait", "lich reply"} {
		if !strings.Contains(stdout, want) {
			t.Errorf("help does not document %q", want)
		}
	}
}

func TestSessionsListsPeers(t *testing.T) {
	f := newFakeLich(t, `[{"label":"docs","project":"lich","kind":"codex"},
	                      {"label":"api","project":"revu","kind":"crush"}]`)

	code, stdout, stderr := run(t, f, "sessions")
	if code != 0 {
		t.Fatalf("exit = %d, stderr = %q", code, stderr)
	}

	call := f.only(t)
	if call.method != "relay.Peers" {
		t.Errorf("method = %q", call.method)
	}
	if call.token != "tok en" {
		t.Errorf("token = %q — it must survive URL encoding", call.token)
	}
	if len(call.args) != 1 || call.args[0] != "s1" {
		t.Errorf("args = %v, want the caller's own session id", call.args)
	}
	for _, want := range []string{"docs\tlich\tcodex", "api\trevu\tcrush"} {
		if !strings.Contains(stdout, want) {
			t.Errorf("output is missing %q:\n%s", want, stdout)
		}
	}
}

func TestSessionsSaysWhenThereAreNone(t *testing.T) {
	f := newFakeLich(t, `[]`)

	code, stdout, _ := run(t, f, "sessions")
	if code != 0 {
		t.Fatalf("exit = %d", code)
	}
	if !strings.Contains(stdout, "No other live sessions.") {
		t.Errorf("empty roster printed %q", stdout)
	}
}

func TestSendPrintsTheAnswer(t *testing.T) {
	f := newFakeLich(t, `{"ticket":"a1b2c3d4","target":"docs","status":"answered","answer":"3 failures"}`)

	code, stdout, stderr := run(t, f, "send", "docs", "run the tests")
	if code != 0 {
		t.Fatalf("exit = %d, stderr = %q", code, stderr)
	}

	call := f.only(t)
	if call.method != "relay.Send" {
		t.Errorf("method = %q", call.method)
	}
	want := []any{"s1", "docs", "", "run the tests", float64(0)}
	if len(call.args) != len(want) {
		t.Fatalf("args = %v, want %v", call.args, want)
	}
	for i := range want {
		if call.args[i] != want[i] {
			t.Errorf("argument %d = %v, want %v", i, call.args[i], want[i])
		}
	}
	if strings.TrimSpace(stdout) != "3 failures" {
		t.Errorf("stdout = %q, want the answer alone", stdout)
	}
}

func TestSendPassesProjectAndTimeout(t *testing.T) {
	f := newFakeLich(t, `{"ticket":"a1b2c3d4","target":"api","status":"answered","answer":"ok"}`)

	code, _, stderr := run(t, f, "send", "--project", "revu", "--timeout", "5", "api", "hello")
	if code != 0 {
		t.Fatalf("exit = %d, stderr = %q", code, stderr)
	}

	call := f.only(t)
	if call.args[2] != "revu" {
		t.Errorf("project = %v", call.args[2])
	}
	if call.args[4] != float64(5) {
		t.Errorf("timeout = %v", call.args[4])
	}
}

func TestSendPointsAtTheTicketWhenTheWaitRunsOut(t *testing.T) {
	f := newFakeLich(t, `{"ticket":"a1b2c3d4","target":"docs","status":"pending","answer":""}`)

	code, stdout, _ := run(t, f, "send", "docs", "long errand")
	if code != 0 {
		t.Fatalf("exit = %d", code)
	}
	// A wait running out is not a dead end anymore: the answer comes back to the
	// sending session's prompt on its own, and holding the line again is the
	// option rather than the instruction.
	for _, want := range []string{
		"docs is still working",
		"typed at the sending session's prompt",
		"lich wait a1b2c3d4",
	} {
		if !strings.Contains(stdout, want) {
			t.Errorf("output is missing %q:\n%s", want, stdout)
		}
	}
}

func TestWaitPicksUpAnAnswer(t *testing.T) {
	f := newFakeLich(t, `{"ticket":"a1b2c3d4","target":"docs","status":"answered","answer":"late but here"}`)

	code, stdout, _ := run(t, f, "wait", "a1b2c3d4")
	if code != 0 {
		t.Fatalf("exit = %d", code)
	}

	call := f.only(t)
	if call.method != "relay.Wait" {
		t.Errorf("method = %q", call.method)
	}
	if call.args[0] != "a1b2c3d4" {
		t.Errorf("ticket = %v", call.args[0])
	}
	if strings.TrimSpace(stdout) != "late but here" {
		t.Errorf("stdout = %q", stdout)
	}
}

func TestReplySendsTheAnswerHome(t *testing.T) {
	f := newFakeLich(t, `null`)

	code, stdout, stderr := run(t, f, "reply", "a1b2c3d4", "3 failures in foo_test")
	if code != 0 {
		t.Fatalf("exit = %d, stderr = %q", code, stderr)
	}

	call := f.only(t)
	if call.method != "relay.Reply" {
		t.Errorf("method = %q", call.method)
	}
	if call.args[0] != "a1b2c3d4" || call.args[1] != "3 failures in foo_test" {
		t.Errorf("args = %v", call.args)
	}
	if !strings.Contains(stdout, "Answer sent.") {
		t.Errorf("stdout = %q", stdout)
	}
}

func TestWrongArgumentCountsFailWithUsage(t *testing.T) {
	f := newFakeLich(t, `null`)

	tests := [][]string{
		{"send", "docs"},
		{"send", "docs", "a prompt", "extra"},
		{"wait"},
		{"reply", "a1b2c3d4"},
	}
	for _, args := range tests {
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			code, _, stderr := run(t, f, args...)
			if code != 1 {
				t.Fatalf("exit = %d, want 1", code)
			}
			if !strings.Contains(stderr, "usage: lich "+args[0]) {
				t.Errorf("stderr = %q, want the subcommand's usage", stderr)
			}
		})
	}
	if len(f.calls) != 0 {
		t.Errorf("a malformed command reached the app: %+v", f.calls)
	}
}

func TestAnUnknownFlagFailsWithoutCallingTheApp(t *testing.T) {
	f := newFakeLich(t, `null`)

	code, _, stderr := run(t, f, "send", "--nonsense", "docs", "hello")
	if code != 1 {
		t.Fatalf("exit = %d, want 1", code)
	}
	if !strings.Contains(stderr, "lich: ") {
		t.Errorf("stderr = %q, want the failure on one line", stderr)
	}
	if len(f.calls) != 0 {
		t.Errorf("a rejected command reached the app: %+v", f.calls)
	}
}

func TestTheAppsErrorReachesTheUser(t *testing.T) {
	f := newFakeLich(t, `{"error":"no live session named \"ghost\""}`)
	f.status = http.StatusInternalServerError

	code, _, stderr := run(t, f, "send", "ghost", "hello")
	if code != 1 {
		t.Fatalf("exit = %d, want 1", code)
	}
	if !strings.Contains(stderr, `no live session named "ghost"`) {
		t.Errorf("stderr = %q, want the app's own message", stderr)
	}
}

func TestAFailureWithNoEnvelopeFallsBackToTheStatus(t *testing.T) {
	f := newFakeLich(t, `<html>gateway</html>`)
	f.status = http.StatusBadGateway

	code, _, stderr := run(t, f, "sessions")
	if code != 1 {
		t.Fatalf("exit = %d, want 1", code)
	}
	if !strings.Contains(stderr, "502") {
		t.Errorf("stderr = %q, want the HTTP status", stderr)
	}
}

func TestWithNoLichAnywhereEveryCommandSaysSo(t *testing.T) {
	bare := func(string) string { return "" }

	for _, args := range [][]string{
		{"sessions"},
		{"send", "docs", "hello"},
		{"wait", "a1b2c3d4"},
		{"reply", "a1b2c3d4", "done"},
	} {
		t.Run(args[0], func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			c := &client{env: bare, stdout: &stdout, stderr: &stderr, running: noInstance}
			if code := dispatch(args, c); code != 1 {
				t.Fatalf("exit = %d, want 1", code)
			}
			if !strings.Contains(stderr.String(), "no lich is running") {
				t.Errorf("stderr = %q", stderr.String())
			}
		})
	}
}

func TestOutsideASessionTheRuntimeFileIsUsed(t *testing.T) {
	f := newFakeLich(t, `[{"label":"docs","project":"lich","kind":"codex"}]`)
	port := f.server.Listener.Addr().(*net.TCPAddr).Port

	var stdout, stderr bytes.Buffer
	c := &client{
		env:     func(string) string { return "" },
		stdout:  &stdout,
		stderr:  &stderr,
		running: func() (*singleton.Info, error) { return &singleton.Info{Port: port, Token: "tok"}, nil },
	}
	if code := dispatch([]string{"sessions"}, c); code != 0 {
		t.Fatalf("exit = %d, stderr = %q", code, stderr.String())
	}

	call := f.only(t)
	if call.token != "tok" {
		t.Errorf("token = %q, want the one from the runtime file", call.token)
	}
	if len(call.args) != 1 || call.args[0] != "" {
		t.Errorf("args = %v, want an empty sender: this caller has no session", call.args)
	}
	if !strings.Contains(stdout.String(), "docs") {
		t.Errorf("stdout = %q", stdout.String())
	}
}

func TestTheEnvironmentWinsOverTheRuntimeFile(t *testing.T) {
	session := newFakeLich(t, `[]`)
	other := newFakeLich(t, `[]`)
	otherPort := other.server.Listener.Addr().(*net.TCPAddr).Port

	var stdout, stderr bytes.Buffer
	c := &client{
		env:     session.env,
		stdout:  &stdout,
		stderr:  &stderr,
		running: func() (*singleton.Info, error) { return &singleton.Info{Port: otherPort, Token: "other"}, nil },
	}
	if code := dispatch([]string{"sessions"}, c); code != 0 {
		t.Fatalf("exit = %d, stderr = %q", code, stderr.String())
	}

	if len(session.calls) != 1 {
		t.Errorf("the session's own lich got %d calls, want 1", len(session.calls))
	}
	if len(other.calls) != 0 {
		t.Errorf("a session answered to the wrong lich: %+v", other.calls)
	}
}

func TestTheRuntimeFileFailureIsReported(t *testing.T) {
	var stdout, stderr bytes.Buffer
	c := &client{
		env:     func(string) string { return "" },
		stdout:  &stdout,
		stderr:  &stderr,
		running: func() (*singleton.Info, error) { return nil, errors.New("permission denied") },
	}
	if code := dispatch([]string{"sessions"}, c); code != 1 {
		t.Fatalf("exit = %d, want 1", code)
	}
	if !strings.Contains(stderr.String(), "permission denied") {
		t.Errorf("stderr = %q", stderr.String())
	}
}

func TestJSONOutputIsMachineReadable(t *testing.T) {
	peers := newFakeLich(t, `[{"label":"docs","project":"lich","kind":"codex"}]`)
	code, stdout, stderr := run(t, peers, "sessions", "--json")
	if code != 0 {
		t.Fatalf("exit = %d, stderr = %q", code, stderr)
	}
	var got []relay.Peer
	if err := json.Unmarshal([]byte(stdout), &got); err != nil {
		t.Fatalf("sessions --json is not JSON: %v (%q)", err, stdout)
	}
	if len(got) != 1 || got[0].Label != "docs" {
		t.Errorf("peers = %+v", got)
	}

	answered := newFakeLich(t, `{"ticket":"a1b2c3d4","target":"docs","status":"answered","answer":"3 failures"}`)
	code, stdout, stderr = run(t, answered, "send", "--json", "docs", "hello")
	if code != 0 {
		t.Fatalf("exit = %d, stderr = %q", code, stderr)
	}
	var result relay.Result
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatalf("send --json is not JSON: %v (%q)", err, stdout)
	}
	if result.Answer != "3 failures" || result.Ticket != "a1b2c3d4" {
		t.Errorf("result = %+v", result)
	}
}

func TestEmptyPeersAreAnEmptyJSONArray(t *testing.T) {
	f := newFakeLich(t, `null`)

	code, stdout, _ := run(t, f, "sessions", "--json")
	if code != 0 {
		t.Fatalf("exit = %d", code)
	}
	if strings.TrimSpace(stdout) != "[]" {
		t.Errorf("stdout = %q, want [] — a script should not have to handle null", stdout)
	}
}
