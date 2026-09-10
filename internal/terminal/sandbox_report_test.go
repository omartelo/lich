// The suite reuses stubBins from terminal_test.go, which is Unix-tagged for its
// PTY spawns; this file carries the same tag so the package still builds on
// Windows. Nothing here spawns anything.
//go:build !windows

package terminal

import (
	"errors"
	"testing"

	"github.com/omartelo/lich/internal/events"
	"github.com/omartelo/lich/internal/providers"
)

// TestReportSandboxRecordsAndAnnouncesTheSkippedLinks is the second half of the
// journey wrapSandbox starts (TestWrapSandboxNamesTheLinksItSkipped): the links
// are resolved once, by a spawn that outlives the page, so they have to reach
// both the window watching now and the row a reload reads them back from.
func TestReportSandboxRecordsAndAnnouncesTheSkippedLinks(t *testing.T) {
	hub, rec := newProbeHub(t)
	recorded := map[string][]string{}
	svc := New(stubBins{sandboxLinks: recorded}, nil, hub)

	svc.reportSandbox("s1", true, []string{".gitconfig", ".ssh/known_hosts"})

	want := []string{".gitconfig", ".ssh/known_hosts"}
	if got := recorded["s1"]; len(got) != len(want) || got[0] != want[0] || got[1] != want[1] {
		t.Errorf("recorded %v, want %v", got, want)
	}
	waitFor(t, func() bool {
		payload, ok := rec.payloadOf(sandboxEventName)
		if !ok {
			return false
		}
		fields, _ := payload.(map[string]any)
		links, _ := fields["skippedLinks"].([]any)
		return len(links) == len(want) && links[0] == want[0]
	}, "the sandbox event to carry the skipped links")
}

// A row that refuses the write costs the card its tooltip at the next reload
// and nothing else — least of all the session. The event has already gone out
// by then, so the window watching now still has the line; refusing to open a
// terminal over what its tooltip says would be the worse trade.
func TestReportSandboxSurvivesARowThatRefusesTheWrite(t *testing.T) {
	hub, rec := newProbeHub(t)
	svc := New(stubBins{sandboxLinksErr: errors.New("database is locked")}, nil, hub)

	svc.reportSandbox("s1", true, []string{".gitconfig"})

	waitFor(t, func() bool {
		_, ok := rec.payloadOf(sandboxEventName)
		return ok
	}, "the sandbox event despite the failed write")
}

// The links ride with the session through the spawn, not around it: Start reads
// them off the session wrapSandbox produced, so an unconfined spawn reports the
// empty answer rather than whatever the last one left behind.
func TestStartReportsNoLinksForAnUnconfinedSpawn(t *testing.T) {
	t.Setenv("SHELL", "sh")
	recorded := map[string][]string{}
	svc := New(stubBins{bin: stayAliveBin(t), sandboxLinks: recorded}, nil, events.New())
	t.Cleanup(func() { _ = svc.Close("s1") })

	if err := svc.Start("s1", "p1", t.TempDir(), providers.Claude, "", "", false, false, 80, 24); err != nil {
		t.Fatalf("Start = %v, want nil", err)
	}

	links, reported := recorded["s1"]
	if !reported {
		t.Fatal("the spawn recorded no answer at all; the card has nothing to read back")
	}
	if len(links) != 0 {
		t.Errorf("an unconfined spawn recorded skipped links %v", links)
	}
}
