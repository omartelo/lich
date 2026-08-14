package terminal

import (
	"fmt"
	"net/http"
	"slices"
	"strings"
	"testing"
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
