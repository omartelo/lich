// The suite reuses stubBins from terminal_test.go, which is Unix-tagged for its
// PTY spawns; this file carries the same tag so the package still builds on
// Windows. Nothing here is platform-specific.
//go:build !windows

package terminal

import (
	"strings"
	"sync"
	"testing"
)

// parkRecorder stands in for the relay's ParkResume.
type parkRecorder struct {
	mu    sync.Mutex
	parks map[string]int64
}

func (p *parkRecorder) park(id string, resetsAt int64) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.parks == nil {
		p.parks = make(map[string]int64)
	}
	p.parks[id] = resetsAt
}

func (p *parkRecorder) of(id string) (int64, bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	at, ok := p.parks[id]
	return at, ok
}

// Claude Code reports the turn a limit ended as `done` (StopFailure, through the
// plugin); the transcript says it was a limit and when it lifts.
func TestADoneOnAClaudeLimitTellsTheWindowAndParksTheResume(t *testing.T) {
	writeTranscript(t, "uuid-abc", claudeLimitLine+"\n")
	hub, rec := newProbeHub(t)
	svc := New(stubBins{providerSession: "uuid-abc"}, nil, hub)
	parks := &parkRecorder{}
	svc.SetUsageLimit(parks.park)

	svc.onHookState(hookRequest{SessionID: "s1", State: statusBusy})
	svc.onHookState(hookRequest{SessionID: "s1", State: statusDone})

	waitFor(t, func() bool { _, ok := parks.of("s1"); return ok }, "the resume to be parked")
	if at, _ := parks.of("s1"); at != 1788653400 {
		t.Errorf("parked for %d, want the transcript's reset 1788653400", at)
	}
	waitFor(t, func() bool { _, ok := rec.payloadOf(limitEventName); return ok }, "the limit event")
	payload, _ := rec.payloadOf(limitEventName)
	got, _ := payload.(map[string]any)
	if got["id"] != "s1" || got["window"] != limitWindowSession || got["resetsAt"] != float64(1788653400) {
		t.Errorf("limit event = %v", payload)
	}
}

func TestATurnThatFinishedParksNothing(t *testing.T) {
	writeTranscript(t, "uuid-abc", `{"type":"assistant","message":{"content":[{"type":"text","text":"All green."}]}}`+"\n")
	svc := New(stubBins{providerSession: "uuid-abc"}, nil, nil)
	parks := &parkRecorder{}
	svc.SetUsageLimit(parks.park)

	if svc.noteLimit("s1") {
		t.Fatal("a turn that ended with an answer read as a limit")
	}
	if _, ok := parks.of("s1"); ok {
		t.Fatal("a resume was parked for a turn that finished")
	}
}

// Codex raises no hook when a limit ends its turn, so the watch is what ends it:
// the card stops spinning and the resume is parked all the same.
func TestTheWatchEndsACodexTurnALimitStopped(t *testing.T) {
	plantCodexRollout(t, "codex-1", strings.Join([]string{
		codexTaskStarted, codexUserMessage, codexLimitTokens, codexSilentFinish,
	}, "\n")+"\n")
	hub, rec := newProbeHub(t)
	svc := New(stubBins{providerSession: "codex-1"}, nil, hub)
	parks := &parkRecorder{}
	svc.SetUsageLimit(parks.park)
	svc.turns.report("s1", statusBusy)

	svc.watchCodexLimits()

	if svc.turns.busy("s1") {
		t.Error("the turn a limit ended is still open")
	}
	if at, ok := parks.of("s1"); !ok || at != 1791432079 {
		t.Errorf("parked %d, %v; want the rollout's reset 1791432079", at, ok)
	}
	waitFor(t, func() bool { return len(rec.statesOf("s1")) > 0 }, "the turn's end to reach the window")
	if got := rec.statesOf("s1"); got[len(got)-1] != statusInterrupted {
		t.Errorf("window was told %v, want the turn interrupted", got)
	}
}

// Only an open turn is looked at: a Codex session sitting at its prompt has
// nothing a limit could still be ending.
func TestTheWatchLeavesAClosedTurnAlone(t *testing.T) {
	plantCodexRollout(t, "codex-1", strings.Join([]string{
		codexTaskStarted, codexLimitTokens, codexSilentFinish,
	}, "\n")+"\n")
	svc := New(stubBins{providerSession: "codex-1"}, nil, nil)
	parks := &parkRecorder{}
	svc.SetUsageLimit(parks.park)

	svc.watchCodexLimits()

	if _, ok := parks.of("s1"); ok {
		t.Fatal("a session with no open turn had a resume parked")
	}
}
