package terminal

import (
	"context"
	"fmt"
	"net/http"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"
	"github.com/omartelo/lich/internal/events"
)

// The two readings of one `waiting` report, which is the whole point of
// turnLog: inside a turn a human is blocking it, outside one the session is
// merely sitting at its prompt.
func TestTurnLogTellsBlockedFromIdle(t *testing.T) {
	tests := []struct {
		name   string
		before []string
		want   bool
	}{
		{"permission prompt inside a turn", []string{statusBusy}, true},
		{"a tool ran first", []string{statusBusy, statusBusy}, true},
		{"a second prompt in the same turn", []string{statusBusy, statusWaiting}, true},
		{"the turn resumed after the first prompt", []string{statusBusy, statusWaiting, statusBusy}, true},
		{"idle at the prompt after a finished turn", []string{statusBusy, statusDone}, false},
		{"nothing was ever reported", nil, false},
		{"the session ended", []string{statusBusy, statusIdle}, false},
		{"a turn that ended and one that never started", []string{statusBusy, statusDone, statusWaiting}, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var log turnLog
			for _, state := range tc.before {
				log.report("s1", state)
			}
			if got := log.report("s1", statusWaiting); got != tc.want {
				t.Fatalf("waiting after %v published = %v, want %v", tc.before, got, tc.want)
			}
		})
	}
}

// Every state but `waiting` reaches the window as sent — the filter is one
// distinction, not a gate on the stream.
func TestTurnLogPublishesEveryOtherState(t *testing.T) {
	var log turnLog
	for _, state := range []string{statusBusy, statusDone, statusIdle, statusBusy} {
		if !log.report("s1", state) {
			t.Fatalf("%q was held back from the window", state)
		}
	}
}

// One session's turn says nothing about another's: a permission prompt in a
// busy session must not vouch for an idle one.
func TestTurnLogKeepsSessionsApart(t *testing.T) {
	var log turnLog
	log.report("busy-one", statusBusy)
	if log.report("idle-one", statusWaiting) {
		t.Fatal("an idle session was published as blocked because another one was busy")
	}
	if !log.report("busy-one", statusWaiting) {
		t.Fatal("the busy session's permission prompt was held back")
	}
}

// A PTY respawned under the same id starts from nothing: the turn the previous
// provider left open died with it, and reading the new one against it would
// badge its first idle notification as a block.
func TestTurnLogForgetsARespawnedSession(t *testing.T) {
	var log turnLog
	log.report("s1", statusBusy)
	log.forget("s1")
	if log.report("s1", statusWaiting) {
		t.Fatal("the respawned session inherited the dead provider's open turn")
	}
}

// The end of the wire the user sees: a report reaches the window through the
// hook endpoint and the /events socket, or it does not reach it at all. The
// same run covers both directions, because the second `waiting` is the one the
// author saw badge a worker that had just answered.
func TestHookPublishesOnlyABlockingWait(t *testing.T) {
	hub, rec := newProbeHub(t)
	svc := New(hookStore{}, nil, hub)
	if svc.wsErr != nil {
		t.Fatalf("transport: %v", svc.wsErr)
	}

	// A turn with a permission prompt in it, then a turn that ends and leaves
	// the session sitting at its prompt.
	for _, state := range []string{"busy", "waiting", "busy", "done", "waiting"} {
		postHook(t, svc, "s1", state)
	}

	// Every emit is written to the socket before its POST answers, and the
	// socket is FIFO, so a sentinel coming back proves the whole stream has
	// landed — including a fifth report that should not be in it. Asserting on a
	// count would pass on the first four and never see the one this test is for.
	hub.Emit(probeReadyEvent, nil)
	waitFor(t, func() bool { return slices.Contains(rec.snapshot(), probeReadyEvent) },
		"the window to be told about both turns")

	want := []string{"busy", "waiting", "busy", "done"}
	got := rec.statesOf("s1")
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("the window was told %v, want %v", got, want)
	}
}

// hookStore answers the one question a status report asks of the store: every
// non-idle report refreshes the context readout, which looks up the session's
// provider conversation. An empty one ends that lookup there. The rest of the
// interface is embedded and nil, so a path that starts using it panics loudly
// rather than being quietly stubbed out here.
type hookStore struct{ Store }

func (hookStore) ProviderSession(string) (string, error) { return "", nil }

// postHook reports one state the way the companion plugin does, and fails the
// test unless lich accepted it (docs/hooks/session-state.md).
func postHook(t *testing.T, svc *Service, id, state string) {
	t.Helper()
	url := fmt.Sprintf("http://127.0.0.1:%d/hook?token=%s", svc.ws.port, svc.ws.token)
	body := fmt.Sprintf(`{"session_id":%q,"state":%q}`, id, state)
	resp, err := http.Post(url, "application/json", strings.NewReader(body))
	if err != nil {
		t.Fatalf("post %q: %v", state, err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("hook %q: status = %d, want 204", state, resp.StatusCode)
	}
}

// The turn an interrupt ends is one lich heard start. Ctrl+C at a prompt with
// nothing running is how a line is thrown away, and it must say nothing at all.
func TestTurnLogInterruptOnlyEndsAnOpenTurn(t *testing.T) {
	tests := []struct {
		name   string
		before []string
		want   bool
	}{
		{"a turn is running", []string{statusBusy}, true},
		{"blocked on a permission inside a turn", []string{statusBusy, statusWaiting}, true},
		{"nothing was ever reported", nil, false},
		{"the turn already finished", []string{statusBusy, statusDone}, false},
		{"the session ended", []string{statusBusy, statusIdle}, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var log turnLog
			for _, state := range tc.before {
				log.report("s1", state)
			}
			if got := log.interrupt("s1"); got != tc.want {
				t.Fatalf("interrupt after %v ended a turn = %v, want %v", tc.before, got, tc.want)
			}
		})
	}
}

// An interrupt ends the turn for good: the report after it is read against a
// session at its prompt, so a `waiting` that follows is the provider's "your
// turn" nudge rather than a human being blocked.
func TestTurnLogInterruptClosesTheTurn(t *testing.T) {
	var log turnLog
	log.report("s1", statusBusy)
	if !log.interrupt("s1") {
		t.Fatal("the open turn was not ended by the interrupt")
	}
	if log.interrupt("s1") {
		t.Fatal("a second interrupt ended a turn that was already over")
	}
	if log.report("s1", statusWaiting) {
		t.Fatal("an idle prompt after an interrupt was published as a block")
	}
}

// One session's interrupt is not another's: the key was pressed at one prompt.
func TestTurnLogInterruptKeepsSessionsApart(t *testing.T) {
	var log turnLog
	log.report("one", statusBusy)
	log.report("two", statusBusy)
	log.interrupt("one")
	if !log.report("two", statusWaiting) {
		t.Fatal("interrupting one session ended another session's turn")
	}
}

// The end of the wire for the fallback: a key pressed in a busy session's
// terminal reaches the window as the turn ending, and the provider's own report
// still outranks it when one finally arrives.
func TestInterruptedTurnReachesTheWindow(t *testing.T) {
	hub, rec := newProbeHub(t)
	svc := New(hookStore{}, nil, hub)
	if svc.wsErr != nil {
		t.Fatalf("transport: %v", svc.wsErr)
	}
	svc.mu.Lock()
	svc.sessions["s1"] = &session{}
	svc.mu.Unlock()

	postHook(t, svc, "s1", statusBusy)
	sendInput(t, svc, "s1", []byte{esc})
	postHook(t, svc, "s1", statusDone)

	hub.Emit(probeReadyEvent, nil)
	waitFor(t, func() bool { return slices.Contains(rec.snapshot(), probeReadyEvent) },
		"the window to be told about the interrupted turn")

	want := []string{statusBusy, statusInterrupted, statusDone}
	if got := rec.statesOf("s1"); strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("the window was told %v, want %v", got, want)
	}
}

// The state lich raises itself is not one a hook may claim: it says something
// only lich is in a position to know, and the endpoint stays closed to it.
func TestHookRejectsTheInterruptedState(t *testing.T) {
	svc := New(hookStore{}, nil, events.New())
	if svc.wsErr != nil {
		t.Fatalf("transport: %v", svc.wsErr)
	}
	url := fmt.Sprintf("http://127.0.0.1:%d/hook?token=%s", svc.ws.port, svc.ws.token)
	body := fmt.Sprintf(`{"session_id":"s1","state":%q}`, statusInterrupted)
	resp, err := http.Post(url, "application/json", strings.NewReader(body))
	if err != nil {
		t.Fatalf("post: %v", err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("hook %q: status = %d, want 400", statusInterrupted, resp.StatusCode)
	}
}

// sendInput writes one terminal input frame the way the window's xterm does —
// through the socket, so the whole path the keystroke takes is under test and
// not just the two halves of it.
func sendInput(t *testing.T, svc *Service, id string, data []byte) {
	t.Helper()
	conn, err := dial(t, svc.ws, svc.ws.token)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer func() { _ = conn.CloseNow() }()
	frame, err := encodeFrame(id, data)
	if err != nil {
		t.Fatalf("encodeFrame: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := conn.Write(ctx, websocket.MessageBinary, frame); err != nil {
		t.Fatalf("write: %v", err)
	}
	// The socket carries no acknowledgement, so the wait is on the turn the
	// keystroke ends. Asked with the log's own question — a `waiting` is
	// recorded neither way, so this reads the turn without moving it.
	waitFor(t, func() bool { return !svc.turns.report(id, statusWaiting) },
		"the keystroke to end the turn")
}
