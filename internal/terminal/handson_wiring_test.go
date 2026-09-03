// These cover the accumulator wired into the Service, which needs the suite's
// Store stub — and that stub lives in terminal_test.go, which is Unix-only for
// its real PTY spawns. Nothing here spawns anything; the tag is the stub's,
// not this file's. The accumulator's own logic is in handson_test.go and runs
// on every OS.
//go:build !windows

package terminal

import (
	"errors"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/omartelo/lich/internal/events"
)

type failingHandsOnStore struct {
	stubBins
	err error
}

func (s *failingHandsOnStore) AddHandsOn(sessionID string, seconds int64) error {
	if s.err != nil {
		return s.err
	}
	return s.stubBins.AddHandsOn(sessionID, seconds)
}

// newQuietService is a Service on a hand-wound clock with the debounced write
// disarmed. A beat that crosses the flush window spawns a writer goroutine of
// its own (see Service.beatHandsOn), and a test that both beats and reads the
// accumulator would be racing that writer for the same seconds — which is a
// flaky test, not a flaky product. Pushing lastFlush past anything a test winds
// the clock to leaves the arrears where the assertion can see them; the flush
// itself is exercised deliberately, by calling it.
func newQuietService(store Store, clock *fakeClock) *Service {
	svc := New(store, nil, events.New())
	svc.hands.now = clock.now
	svc.hands.lastFlush = clock.now().Add(24 * time.Hour)
	return svc
}

// TestOutputCountsOnlyWhileBusy is the gate that keeps the readout honest.
// A session's own output is not proof anybody is working — a `tail -f`, a dev
// server, a TUI repainting its spinner all draw forever — so it counts only
// while the provider's own report says a turn is open.
func TestOutputCountsOnlyWhileBusy(t *testing.T) {
	clock := newFakeClock()
	svc := newQuietService(stubBins{}, clock)
	sess := &session{}

	// Streaming with no turn open: an hour of it, and nothing is counted.
	for range 120 {
		svc.noteOutput("s1", sess, []byte("waiting for changes..."))
		clock.advance(30 * time.Second)
	}
	if got := drainSeconds(&svc.hands, "s1"); got != 0 {
		t.Fatalf("an idle session streaming output counted %ds, want 0", got)
	}

	// The same output, with the provider reporting a turn: now it is the agent
	// working, and it counts.
	svc.turns.report("s1", statusBusy)
	svc.noteOutput("s1", sess, []byte("thinking..."))
	clock.advance(30 * time.Second)
	svc.noteOutput("s1", sess, []byte("thinking..."))

	if got := drainSeconds(&svc.hands, "s1"); got != 30 {
		t.Errorf("a busy session's output counted %ds, want 30", got)
	}
}

// TestOutputBeatsAreThrottled proves the burst rate of a PTY never reaches the
// accumulator: output arrives in fragments, and one beat per fragment would
// chain gaps too small to ever look like a silence.
func TestOutputBeatsAreThrottled(t *testing.T) {
	clock := newFakeClock()
	svc := newQuietService(stubBins{}, clock)
	svc.turns.report("s1", statusBusy)
	sess := &session{}

	// One second of output, a fragment every 100ms — well inside the throttle.
	for range 10 {
		svc.noteOutput("s1", sess, []byte("."))
		clock.advance(100 * time.Millisecond)
	}

	if got := svc.hands.drain(); len(got) != 0 {
		t.Errorf("a burst of output fragments counted %v, want nothing", got)
	}
}

// TestKeystrokesCountWithoutAnyProviderReport proves the one beat source every
// session has: a shell, and a provider that reports no state at all, are
// counted from the user's own typing.
func TestKeystrokesCountWithoutAnyProviderReport(t *testing.T) {
	clock := newFakeClock()
	svc := newQuietService(stubBins{}, clock)

	svc.beatHandsOn("s1", 0)
	clock.advance(2 * time.Minute)
	svc.beatHandsOn("s1", 0)

	if got := drainSeconds(&svc.hands, "s1"); got != 120 {
		t.Errorf("typing counted %ds, want 120", got)
	}
}

// TestFlushWritesThroughToTheStore proves the arrears reach the row, and reach
// it as an addition — a second flush must not restate the first.
func TestFlushWritesThroughToTheStore(t *testing.T) {
	clock := newFakeClock()
	store := stubBins{handsOn: map[string]int64{}}
	svc := newQuietService(store, clock)

	svc.beatHandsOn("s1", 0)
	clock.advance(time.Minute)
	svc.beatHandsOn("s1", 0)
	svc.FlushHandsOn()
	clock.advance(time.Minute)
	svc.beatHandsOn("s1", 0)
	svc.FlushHandsOn()

	total, err := svc.HandsOn("s1")
	if err != nil {
		t.Fatalf("HandsOn: %v", err)
	}
	if total != 120 {
		t.Errorf("two minutes over two flushes read back as %ds, want 120", total)
	}
}

func TestFlushRestoresTimeAfterAStoreFailure(t *testing.T) {
	clock := newFakeClock()
	store := &failingHandsOnStore{
		stubBins: stubBins{handsOn: map[string]int64{}},
		err:      errors.New("busy"),
	}
	svc := newQuietService(store, clock)
	svc.beatHandsOn("s1", 0)
	clock.advance(time.Minute)
	svc.beatHandsOn("s1", 0)

	svc.FlushHandsOn()
	store.err = nil
	svc.FlushHandsOn()

	if got := store.handsOn["s1"]; got != 60 {
		t.Errorf("retry wrote %ds, want the failed 60s restored", got)
	}
}

// TestClosingASessionWritesWhatItWasWorked proves the close flushes before the
// window deletes the row: a write that loses that race is dropped silently, and
// this is a closed card's last chance to keep its number.
func TestClosingASessionWritesWhatItWasWorked(t *testing.T) {
	clock := newFakeClock()
	store := stubBins{handsOn: map[string]int64{}}
	svc := newQuietService(store, clock)

	svc.beatHandsOn("s1", 0)
	clock.advance(20 * time.Second)
	svc.beatHandsOn("s1", 0)
	if err := svc.Close("s1"); err != nil {
		t.Fatalf("Close: %v", err)
	}

	if store.handsOn["s1"] != 20 {
		t.Errorf("closing wrote %ds, want 20", store.handsOn["s1"])
	}
}

// TestEveryHookReportBeats is the rung Crush is on. Its only hook event is
// `PreToolUse`, and neither script the plugin registers there reports a state
// (docs/hooks/session-state.md) — so a Crush turn reaches lich as
// `/session-start`, once per tool call (measured 2026-09-03 against Crush
// 0.88.0, three tool calls in one turn, three POSTs). A beat wired to the state
// report alone counts that turn as nothing at all, and the hour the user
// watched it work reads as the seconds they typed in.
//
// Driven over the transport rather than by calling beatHandsOn, because the
// wiring is the whole claim: the beat has to survive being written into a
// callback that is about something else.
func TestEveryHookReportBeats(t *testing.T) {
	for _, tc := range []struct{ name, path, body string }{
		{"session-start", "/session-start", `{"session_id":"s1","provider_session_id":"u9","provider":"crush"}`},
		{"session-title", "/session-title", `{"session_id":"s1","title":"Explore this directory"}`},
		{"session-touched", "/session-touched", `{"session_id":"s1"}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			clock := newFakeClock()
			svc := newQuietService(stubBins{}, clock)
			if svc.ws == nil {
				t.Fatalf("transport did not start: %v", svc.wsErr)
			}
			url := fmt.Sprintf("http://127.0.0.1:%d%s?token=%s", svc.ws.port, tc.path, svc.ws.token)
			post := func() {
				resp, err := http.Post(url, "application/json", strings.NewReader(tc.body))
				if err != nil {
					t.Fatalf("post %s: %v", tc.path, err)
				}
				_ = resp.Body.Close()
				if resp.StatusCode != http.StatusNoContent {
					t.Fatalf("%s status = %d, want 204", tc.path, resp.StatusCode)
				}
			}
			post()
			clock.advance(2 * time.Minute)
			post()

			if got := drainSeconds(&svc.hands, "s1"); got != 120 {
				t.Errorf("two reports two minutes apart counted %ds, want 120", got)
			}
		})
	}
}
