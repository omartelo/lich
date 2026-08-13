// The suite spawns real PTYs and reuses stubBins from terminal_test.go, which
// is Unix-tagged for the same reason; this file carries the tag so the package
// still builds on Windows.
//go:build !windows

package terminal

import (
	"os"
	"strings"
	"testing"
	"time"

	"github.com/omartelo/lich/internal/events"
)

// A session waits for the setup marker only if something is going to print it.
// The wrap is what prints it, so the wrap is what has to say so: a provider
// whose binary happens to be named sh — a user's own $SHELL, a wrapper script —
// spawned into a fresh worktree that has no setup script would otherwise be
// armed to wait for a marker that never comes, and stay unready until every
// relay sent to it times out.
func TestASetupSpawnWithNoScriptIsReadyAnyway(t *testing.T) {
	t.Setenv("SHELL", "sh")
	svc := New(stubBins{}, nil, events.New())
	t.Cleanup(func() { _ = svc.Close("s1") })

	if err := svc.Start("s1", "p1", t.TempDir(), KindShell, "", "", true, 80, 24); err != nil {
		t.Fatalf("Start = %v, want nil", err)
	}

	time.Sleep(readySettle + 100*time.Millisecond)
	if !svc.Ready("s1") {
		t.Error("a session with no setup script to run was left waiting for its marker")
	}
}

// The other side of the same decision: a project that does have a setup script
// arms the wait, and the session is held back until the marker clears it.
func TestASetupSpawnWithAScriptWaitsForItsMarker(t *testing.T) {
	t.Setenv("SHELL", "sh")
	svc := New(stubBins{setup: "sleep 30"}, nil, events.New())
	t.Cleanup(func() { _ = svc.Close("s1") })

	if err := svc.Start("s1", "p1", t.TempDir(), KindShell, "", "", true, 80, 24); err != nil {
		t.Fatalf("Start = %v, want nil", err)
	}

	time.Sleep(readySettle + 100*time.Millisecond)
	if svc.Ready("s1") {
		t.Error("a session still running its setup script was offered work")
	}
}

// The marker is one write from the wrapper, but a PTY read is cut wherever the
// kernel's buffer ends — a setup script that finishes mid-buffer splits it. Half
// a marker matches nothing, so the session would stay settingUp for good and
// refuse every message it is ever sent.
func TestTheSetupMarkerSurvivesASplitRead(t *testing.T) {
	svc := New(stubBins{}, nil, events.New())
	sess := &session{settingUp: true}
	svc.mu.Lock()
	svc.sessions["s1"] = sess
	svc.mu.Unlock()

	head, tail := setupDone[:8], setupDone[8:]
	svc.noteOutput(sess, []byte("added 551 packages"+head))
	svc.mu.Lock()
	stillSettingUp := sess.settingUp
	svc.mu.Unlock()
	if !stillSettingUp {
		t.Fatal("half a marker ended the setup")
	}

	svc.noteOutput(sess, []byte(tail+"\x1b[?25l"))

	svc.mu.Lock()
	defer svc.mu.Unlock()
	if sess.settingUp {
		t.Error("the marker was missed because a read split it in two")
	}
}

// A chunk that only looks like the marker's beginning must not be carried
// forever: what is kept is a bounded tail, and it never ends the setup on its
// own.
func TestAPartialMarkerNeverEndsTheSetup(t *testing.T) {
	svc := New(stubBins{}, nil, events.New())
	sess := &session{settingUp: true}

	for range 3 {
		svc.noteOutput(sess, []byte(setupDone[:len(setupDone)-1]))
	}

	svc.mu.Lock()
	defer svc.mu.Unlock()
	if !sess.settingUp {
		t.Error("a marker that was never completed ended the setup")
	}
	if len(sess.setupTail) >= len(setupDone) {
		t.Errorf("the carried tail grew to %d bytes, want under %d", len(sess.setupTail), len(setupDone))
	}
}

// A spawn that cannot start is reported as itself: the session id is in the
// error, because the frontend surfaces it and "no such file" alone names
// neither the session nor what lich was trying to run.
func TestStartReportsAPtyThatCannotSpawn(t *testing.T) {
	svc := New(stubBins{bin: "/nonexistent/lich-test-provider"}, nil, events.New())

	err := svc.Start("s1", "p1", t.TempDir(), "claude", "", "", false, 80, 24)

	if err == nil {
		t.Fatal("Start = nil, want the spawn failure")
	}
	if !strings.Contains(err.Error(), `failed to start pty for "s1"`) {
		t.Errorf("Start = %v, want it to name the session it could not spawn", err)
	}
	if svc.Live("s1") {
		t.Error("a session that never spawned was registered as live")
	}
}

// A session opened with no directory of its own starts at home, and says so:
// the effective directory is what the card shows and what the cwd poller
// degrades back to.
func TestASpawnWithNoDirectoryStartsAtHome(t *testing.T) {
	svc := New(stubBins{bin: stayAliveBin(t)}, nil, events.New())
	t.Cleanup(func() { _ = svc.Close("s1") })

	_, cwd, err := svc.spawnSession("s1", "p1", "", "claude", "", "", false, 80, 24)
	if err != nil {
		t.Fatalf("spawnSession = %v, want nil", err)
	}

	home, err := os.UserHomeDir()
	if err != nil {
		t.Skipf("no home directory to fall back to: %v", err)
	}
	if cwd != home {
		t.Errorf("start directory = %q, want the home directory %q", cwd, home)
	}
}
