// The hold is pure Service logic, but it builds on stubBins, the Store stub that
// lives in the Unix-only suite (terminal_test.go), so this file rides the same
// tag to keep the package building on Windows.
//go:build !windows

package terminal

import (
	"testing"
	"time"

	"github.com/omartelo/lich/internal/events"
	"github.com/omartelo/lich/internal/providers"
)

// quietSession is a session that went quiet long enough ago to have latched ready.
func quietSession(svc *Service, id string, awaitsStart bool) *session {
	sess := &session{lastOut: time.Now().Add(-time.Second), awaitsStart: awaitsStart}
	svc.mu.Lock()
	svc.sessions[id] = sess
	svc.mu.Unlock()
	return sess
}

// A trust question is as quiet as a prompt. The session waits for the report
// its provider sends only from the agent's own prompt, however long the screen
// has been still.
func TestReadyHoldsASessionUntilItsProviderReportsStart(t *testing.T) {
	svc := New(stubBins{}, nil, events.New())
	quietSession(svc, "s1", true)

	if svc.Ready("s1") {
		t.Fatal("a quiet session whose provider has not reported start was offered work")
	}
	svc.markStarted("s1")
	if !svc.Ready("s1") {
		t.Error("a session whose provider reported start was still held")
	}
}

// The hold comes first even when the quiet was already latched: a session can
// be found quiet before the hold is recorded on it (holdForStart), and that
// latch must not let work through.
func TestReadyHoldsASessionWhoseQuietWasAlreadyLatched(t *testing.T) {
	svc := New(stubBins{}, nil, events.New())
	sess := quietSession(svc, "s1", false)
	if !svc.Ready("s1") {
		t.Fatal("a settled session was not ready")
	}

	svc.holdForStart("s1", sess, true)

	if svc.Ready("s1") {
		t.Error("a latched quiet let work through before the provider reported start")
	}
}

func TestReadyDoesNotHoldASessionThatReportsNoStart(t *testing.T) {
	svc := New(stubBins{}, nil, events.New())
	quietSession(svc, "s1", false)

	if !svc.Ready("s1") {
		t.Error("a session with no start report to wait for was held")
	}
}

func TestAwaitsStartOnlyForProvidersThatReportFromTheirPrompt(t *testing.T) {
	svc := New(stubBins{}, nil, events.New())
	if svc.awaitsStart(providers.Claude) {
		t.Error("a session was held with nothing wired to say whether hooks run")
	}

	svc.SetStartReports(func(string) bool { return true })
	for _, kind := range []string{providers.Claude, providers.Cursor, providers.OMP, providers.Kiro} {
		if !svc.awaitsStart(kind) {
			t.Errorf("awaitsStart(%q) = false, want true", kind)
		}
	}
	for _, kind := range []string{
		providers.Codex, providers.Antigravity, providers.Crush, providers.OpenCode, KindShell,
	} {
		if svc.awaitsStart(kind) {
			t.Errorf("awaitsStart(%q) = true, want false: it reports nothing before its first turn", kind)
		}
	}
}

// Without lich's hooks in a provider no report ever comes, and waiting for one
// would leave its sessions unable to take work at all.
func TestAwaitsStartIsOffWhereTheHooksAreNotInstalled(t *testing.T) {
	svc := New(stubBins{}, nil, events.New())
	svc.SetStartReports(func(provider string) bool { return provider != providers.Kiro })

	if svc.awaitsStart(providers.Kiro) {
		t.Error("a Kiro CLI session without lich's hooks was set to wait for a report")
	}
	if !svc.awaitsStart(providers.Claude) {
		t.Error("a Claude Code session with lich's hooks was not set to wait")
	}
}

func TestMarkStartedIgnoresASessionThatIsNotRunning(t *testing.T) {
	svc := New(stubBins{}, nil, events.New())
	svc.markStarted("ghost")
	if svc.Ready("ghost") {
		t.Error("a report for a session that is not running made it ready")
	}
}

// A respawn is a new session under the same id: the report its predecessor
// made does not release it.
func TestHoldForStartIgnoresASessionThatWasReplaced(t *testing.T) {
	svc := New(stubBins{}, nil, events.New())
	old := quietSession(svc, "s1", false)
	quietSession(svc, "s1", false)

	svc.holdForStart("s1", old, true)

	if !svc.Ready("s1") {
		t.Error("a hold meant for the replaced session landed on its successor")
	}
}

// Through the real spawn: the session settles, stays held, and is released by
// the provider's report coming through the same path the hook's POST takes.
func TestStartHoldsAProviderSessionUntilItsReport(t *testing.T) {
	bin := stayAliveBin(t)
	svc := New(stubBins{bin: bin}, nil, events.New())
	svc.SetStartReports(func(string) bool { return true })
	t.Cleanup(func() { _ = svc.Close("s1") })

	if err := svc.Start("s1", "p1", t.TempDir(), providers.Claude, "", "", false, false, 80, 24); err != nil {
		t.Fatalf("Start = %v, want nil", err)
	}
	time.Sleep(readySettle + 100*time.Millisecond)
	if svc.Ready("s1") {
		t.Fatal("a Claude Code session was offered work before it reported start")
	}

	if err := svc.onSessionStart("s1", "6f1c1f0e-0000-4000-8000-000000000001", providers.Claude); err != nil {
		t.Fatalf("onSessionStart = %v, want nil", err)
	}
	if !svc.Ready("s1") {
		t.Error("the session stayed held after its provider reported start")
	}
}
