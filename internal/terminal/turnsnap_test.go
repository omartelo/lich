package terminal

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// snapRepo creates a git repository with one commit and a Service wired to keep
// its snapshot indexes under the test's own directory, never the user's config
// directory. Skips when git is unavailable.
func snapRepo(t *testing.T) (*Service, string) {
	t.Helper()
	repo := t.TempDir()
	if out, err := exec.Command("git", "init", "-b", "main", repo).CombinedOutput(); err != nil {
		t.Skipf("git init unavailable: %v (%s)", err, out)
	}
	for _, args := range [][]string{
		{"config", "user.email", "t@example.com"},
		{"config", "user.name", "t"},
		{"config", "core.autocrlf", "false"},
	} {
		run(t, repo, args...)
	}
	writeIn(t, repo, "a.txt", "one\n")
	run(t, repo, "add", "-A")
	run(t, repo, "commit", "-m", "init")

	svc := &Service{}
	svc.snaps.root = t.TempDir()
	// Registered after both TempDirs, so it runs before either is removed:
	// track queues a warm-up snapshot nothing waits for, and a `git add -A`
	// still writing into a directory RemoveAll is walking fails the test with a
	// cleanup error that has nothing to do with what it asserted.
	t.Cleanup(func() { drainSnaps(t, svc) })
	return svc, repo
}

// drainSnaps blocks until the snapshot worker has run everything queued before
// it. FIFO is what makes this work: a job submitted last finishes last.
func drainSnaps(t *testing.T, svc *Service) {
	t.Helper()
	drained := make(chan struct{})
	svc.snaps.submit(func() { close(drained) })
	select {
	case <-drained:
	case <-time.After(30 * time.Second):
		t.Error("the snapshot worker never drained")
	}
}

func run(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %s: %v (%s)", strings.Join(args, " "), err, out)
	}
}

func writeIn(t *testing.T, dir, rel, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, rel), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

// snapBefore reads the opening tree of the turn a session has open, which is
// what a test has to wait for before editing: the snapshot runs on the worker,
// and an edit racing it would land on the wrong side of the window.
func snapBefore(s *turnSnaps, id string) string {
	s.mu.Lock()
	defer s.mu.Unlock()
	if state, ok := s.sessions[id]; ok {
		return state.before
	}
	return ""
}

// openAndWait opens a turn and blocks until its opening snapshot has landed.
func openAndWait(t *testing.T, svc *Service, id string) {
	t.Helper()
	svc.snaps.note(id, statusBusy)
	waitFor(t, func() bool { return snapBefore(&svc.snaps, id) != "" },
		"the turn's opening snapshot")
}

// closeAndWait ends a turn and blocks until the panel can answer for it.
func closeAndWait(t *testing.T, svc *Service, id, want string) LastTurn {
	t.Helper()
	svc.snaps.note(id, statusDone)
	var got LastTurn
	waitFor(t, func() bool {
		turn, err := svc.LastTurnDiff(id)
		if err != nil {
			return false
		}
		got = turn
		return turn.State == want
	}, "the last turn to read as "+want)
	return got
}

// The feature itself: what the checkout gained between a turn starting and
// ending, rendered as the unified diff the panel already knows how to draw.
func TestLastTurnDiffRendersTheWindow(t *testing.T) {
	svc, repo := snapRepo(t)
	svc.snaps.track("s1", repo)

	openAndWait(t, svc, "s1")
	writeIn(t, repo, "a.txt", "one\ntwo\n")
	writeIn(t, repo, "b.txt", "fresh\n")
	turn := closeAndWait(t, svc, "s1", turnDiffOK)

	for _, want := range []string{"a/a.txt", "+two", "+++ b/b.txt", "+fresh"} {
		if !strings.Contains(turn.Diff, want) {
			t.Errorf("the diff is missing %q:\n%s", want, turn.Diff)
		}
	}
	if turn.EndedAt == 0 {
		t.Error("a closed window carried no time")
	}
}

// The distinction the whole contract exists for: a turn that ran and touched
// nothing is an answer, and it must not wear the words for having no answer.
func TestATurnThatChangedNothingIsEmpty(t *testing.T) {
	svc, repo := snapRepo(t)
	svc.snaps.track("s1", repo)

	openAndWait(t, svc, "s1")
	turn := closeAndWait(t, svc, "s1", turnDiffEmpty)

	if turn.Diff != "" {
		t.Errorf("an empty turn carried a diff: %q", turn.Diff)
	}
	if turn.EndedAt == 0 {
		t.Error("an empty turn is still a window and should be dated")
	}
}

// Every provider reports `busy` again between tool calls, and Antigravity
// reports one before each model call. Treating those as new turns would walk the
// opening snapshot forward through the turn it is supposed to precede, so the
// panel would show only whatever happened after the last tool.
func TestARepeatBusyDoesNotMoveTheWindow(t *testing.T) {
	svc, repo := snapRepo(t)
	svc.snaps.track("s1", repo)

	openAndWait(t, svc, "s1")
	writeIn(t, repo, "early.txt", "before the tool\n")
	// The pre-tool report, arriving mid-turn.
	svc.snaps.note("s1", statusBusy)
	writeIn(t, repo, "late.txt", "after the tool\n")
	turn := closeAndWait(t, svc, "s1", turnDiffOK)

	for _, want := range []string{"+++ b/early.txt", "+++ b/late.txt"} {
		if !strings.Contains(turn.Diff, want) {
			t.Errorf("the window closed over %q:\n%s", want, turn.Diff)
		}
	}
}

// "Last turn" is the last turn, full stop. A quiet turn after a busy one must
// report itself as quiet — offering the earlier turn's diff instead would
// answer a question nobody asked, and would do it convincingly.
func TestAQuietTurnDoesNotFallBackToTheOneBefore(t *testing.T) {
	svc, repo := snapRepo(t)
	svc.snaps.track("s1", repo)

	openAndWait(t, svc, "s1")
	writeIn(t, repo, "a.txt", "changed\n")
	if turn := closeAndWait(t, svc, "s1", turnDiffOK); !strings.Contains(turn.Diff, "+changed") {
		t.Fatalf("the first turn did not record its change:\n%s", turn.Diff)
	}

	openAndWait(t, svc, "s1")
	closeAndWait(t, svc, "s1", turnDiffEmpty)
}

// The same rule against a lost snapshot rather than a quiet turn: a turn whose
// opening tree never landed — a dropped job, a git that failed — clears the
// record instead of leaving the previous turn standing as if it were this one.
func TestATurnWithNoOpeningSnapshotClearsTheRecord(t *testing.T) {
	svc, repo := snapRepo(t)
	svc.snaps.track("s1", repo)

	openAndWait(t, svc, "s1")
	writeIn(t, repo, "a.txt", "changed\n")
	closeAndWait(t, svc, "s1", turnDiffOK)

	// The state a dropped opening snapshot leaves behind: a turn is open and
	// nothing recorded where it started.
	svc.snaps.mu.Lock()
	svc.snaps.sessions["s1"].open = true
	svc.snaps.sessions["s1"].seq++
	svc.snaps.sessions["s1"].before = ""
	svc.snaps.mu.Unlock()

	closeAndWait(t, svc, "s1", turnDiffUnavailable)
}

// A session whose first turn is still running has nothing to show, and a
// session lich never tracked has nothing to show either. Both are the absence
// of an answer, never a quiet turn.
func TestNoClosedTurnIsUnavailable(t *testing.T) {
	svc, repo := snapRepo(t)

	turn, err := svc.LastTurnDiff("never-tracked")
	if err != nil {
		t.Fatalf("LastTurnDiff: %v", err)
	}
	if turn.State != turnDiffUnavailable {
		t.Errorf("an untracked session reads as %q, want %q", turn.State, turnDiffUnavailable)
	}

	svc.snaps.track("s1", repo)
	openAndWait(t, svc, "s1")
	turn, err = svc.LastTurnDiff("s1")
	if err != nil {
		t.Fatalf("LastTurnDiff: %v", err)
	}
	if turn.State != turnDiffUnavailable {
		t.Errorf("a turn still running reads as %q, want %q", turn.State, turnDiffUnavailable)
	}
	if turn.EndedAt != 0 {
		t.Error("an unrecorded turn was dated")
	}
}

// lich reads an interrupt off the PTY because no provider reports one, and a
// stopped turn still changed files. Without this the panel would hold the turn
// before it open until some later one finished.
func TestAnInterruptClosesTheWindow(t *testing.T) {
	svc, repo := snapRepo(t)
	svc.snaps.track("s1", repo)

	openAndWait(t, svc, "s1")
	writeIn(t, repo, "a.txt", "half done\n")
	svc.snaps.closeTurn("s1")

	waitFor(t, func() bool {
		turn, err := svc.LastTurnDiff("s1")
		return err == nil && turn.State == turnDiffOK && strings.Contains(turn.Diff, "+half done")
	}, "the interrupted turn to be recorded")
}

// SessionEnd says the CLI left mid-turn: there is no closing snapshot coming,
// so that turn is abandoned — but the last one that did close is still the last
// one that closed, and the session ending does not un-happen it.
func TestSessionEndAbandonsTheOpenTurnAndKeepsTheClosedOne(t *testing.T) {
	svc, repo := snapRepo(t)
	svc.snaps.track("s1", repo)

	openAndWait(t, svc, "s1")
	writeIn(t, repo, "a.txt", "recorded\n")
	closeAndWait(t, svc, "s1", turnDiffOK)

	openAndWait(t, svc, "s1")
	writeIn(t, repo, "b.txt", "never closed\n")
	svc.snaps.note("s1", statusIdle)

	turn, err := svc.LastTurnDiff("s1")
	if err != nil {
		t.Fatalf("LastTurnDiff: %v", err)
	}
	if !strings.Contains(turn.Diff, "+recorded") {
		t.Errorf("the closed turn was lost with the session:\n%s", turn.Diff)
	}
	if strings.Contains(turn.Diff, "never closed") {
		t.Error("a turn nothing closed was reported as finished")
	}
	// And the abandoned turn cannot be closed later by the next `done`.
	svc.snaps.note("s1", statusDone)
	svc.snaps.mu.Lock()
	open := svc.snaps.sessions["s1"].open
	svc.snaps.mu.Unlock()
	if open {
		t.Error("a session that ended still holds a turn open")
	}
}

// A closed card's row is deleted, so its accounting is dead weight — and its
// index file would outlive it under lich's config directory.
func TestForgetDropsTheRecordAndTheIndex(t *testing.T) {
	svc, repo := snapRepo(t)
	svc.snaps.track("s1", repo)

	openAndWait(t, svc, "s1")
	writeIn(t, repo, "a.txt", "changed\n")
	closeAndWait(t, svc, "s1", turnDiffOK)

	index := filepath.Join(svc.snaps.root, "s1.idx")
	if _, err := os.Stat(index); err != nil {
		t.Fatalf("the index was never written: %v", err)
	}

	svc.snaps.forget("s1")
	if _, err := os.Stat(index); !os.IsNotExist(err) {
		t.Errorf("the index survived the close: %v", err)
	}
	turn, err := svc.LastTurnDiff("s1")
	if err != nil {
		t.Fatalf("LastTurnDiff: %v", err)
	}
	if turn.State != turnDiffUnavailable {
		t.Errorf("a forgotten session reads as %q, want %q", turn.State, turnDiffUnavailable)
	}
}

// A respawn under the same id is a new session's turns. Whatever the provider
// that left had recorded belongs to it, not to the one now typing.
func TestRespawnDropsThePreviousProvidersTurn(t *testing.T) {
	svc, repo := snapRepo(t)
	svc.snaps.track("s1", repo)

	openAndWait(t, svc, "s1")
	writeIn(t, repo, "a.txt", "the last provider's work\n")
	closeAndWait(t, svc, "s1", turnDiffOK)

	svc.snaps.track("s1", repo)
	turn, err := svc.LastTurnDiff("s1")
	if err != nil {
		t.Fatalf("LastTurnDiff: %v", err)
	}
	if turn.State != turnDiffUnavailable {
		t.Errorf("a respawned session inherited a turn: %q", turn.State)
	}
}

// A `done` for a session with no turn open is what every idle prompt produces,
// and it must not invent a window out of two unrelated snapshots.
func TestADoneWithNoTurnOpenRecordsNothing(t *testing.T) {
	svc, repo := snapRepo(t)
	svc.snaps.track("s1", repo)

	svc.snaps.note("s1", statusDone)
	turn, err := svc.LastTurnDiff("s1")
	if err != nil {
		t.Fatalf("LastTurnDiff: %v", err)
	}
	if turn.State != turnDiffUnavailable {
		t.Errorf("a stray done recorded %q", turn.State)
	}
}

// A card is closed far more often than it is running: its agent exits, the
// terminal says so, and the user closes the dead card some time later. The
// accounting and the index file have to go with it either way.
func TestCloseForgetsASessionWhosePTYAlreadyExited(t *testing.T) {
	svc, repo := snapRepo(t)
	svc.sessions = map[string]*session{}
	svc.snaps.track("s1", repo)

	openAndWait(t, svc, "s1")
	writeIn(t, repo, "a.txt", "changed\n")
	closeAndWait(t, svc, "s1", turnDiffOK)

	index := filepath.Join(svc.snaps.root, "s1.idx")
	if _, err := os.Stat(index); err != nil {
		t.Fatalf("the index was never written: %v", err)
	}
	// No entry in s.sessions: the PTY was reaped by stream, which is the state
	// every session that ended on its own leaves behind.
	if err := svc.Close("s1"); err != nil {
		t.Fatalf("Close: %v", err)
	}

	if _, err := os.Stat(index); !os.IsNotExist(err) {
		t.Errorf("the index outlived the card: %v", err)
	}
	svc.snaps.mu.Lock()
	_, held := svc.snaps.sessions["s1"]
	svc.snaps.mu.Unlock()
	if held {
		t.Error("the accounting outlived the card")
	}
}

// The panel is told when the record lands, not when the turn ends: the two are
// different moments, and only the first one has an answer to give.
func TestFilingATurnNotifiesOnce(t *testing.T) {
	svc, repo := snapRepo(t)
	filed := make(chan string, 4)
	svc.snaps.filed = func(id string) { filed <- id }
	svc.snaps.track("s1", repo)

	openAndWait(t, svc, "s1")
	select {
	case id := <-filed:
		t.Fatalf("an opening snapshot filed nothing, yet notified for %q", id)
	default:
	}

	writeIn(t, repo, "a.txt", "changed\n")
	svc.snaps.note("s1", statusDone)
	select {
	case id := <-filed:
		if id != "s1" {
			t.Errorf("notified for %q, want s1", id)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("the closing snapshot landed without telling the panel")
	}
}

// A close still on the worker must not leave the turn before it on screen
// wearing this turn's name: until the snapshot lands there is no last turn.
func TestACloseInFlightNeverShowsTheTurnBefore(t *testing.T) {
	svc, repo := snapRepo(t)
	svc.snaps.track("s1", repo)

	openAndWait(t, svc, "s1")
	writeIn(t, repo, "a.txt", "one turn\n")
	closeAndWait(t, svc, "s1", turnDiffOK)

	openAndWait(t, svc, "s1")
	writeIn(t, repo, "a.txt", "another turn\n")

	// Occupies the single worker, so the closing snapshot below is queued and
	// cannot land — the same window a dropped job leaves open forever.
	held := make(chan struct{})
	svc.snaps.submit(func() { <-held })
	svc.snaps.note("s1", statusDone)

	turn, err := svc.LastTurnDiff("s1")
	if err != nil {
		t.Fatalf("LastTurnDiff: %v", err)
	}
	if turn.State != turnDiffUnavailable {
		t.Errorf("a close in flight reads as %q (%q), want %q",
			turn.State, turn.Diff, turnDiffUnavailable)
	}
	close(held)
	waitFor(t, func() bool {
		turn, err := svc.LastTurnDiff("s1")
		return err == nil && turn.State == turnDiffOK
	}, "the closing snapshot to land")
}

// A session opened outside a repository has nothing to snapshot. It is dropped
// on the warm-up rather than warned about twice a turn for the rest of its life.
func TestACheckoutGitCannotSnapshotIsDropped(t *testing.T) {
	svc, _ := snapRepo(t)
	plain := t.TempDir()
	svc.snaps.track("s1", plain)

	waitFor(t, func() bool {
		svc.snaps.mu.Lock()
		defer svc.snaps.mu.Unlock()
		_, held := svc.snaps.sessions["s1"]
		return !held
	}, "the untrackable checkout to be dropped")

	svc.snaps.note("s1", statusBusy)
	svc.snaps.note("s1", statusDone)
	turn, err := svc.LastTurnDiff("s1")
	if err != nil {
		t.Fatalf("LastTurnDiff: %v", err)
	}
	if turn.State != turnDiffUnavailable {
		t.Errorf("a directory outside a repository reads as %q, want %q",
			turn.State, turnDiffUnavailable)
	}
}
