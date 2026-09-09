// These cover the read wired into the Service, which needs the suite's Store
// stub, and that stub lives in terminal_test.go, which is Unix-only for its
// real PTY spawns. Nothing here spawns anything; the tag is the stub's, not
// this file's. The read's own logic is in title_kiro_test.go and runs on every
// OS.
//go:build !windows

package terminal

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/omartelo/lich/internal/events"
	"github.com/omartelo/lich/internal/providers"
)

// titleStore records what reached the guarded write, which is the last thing
// lich does with a title before the card redraws.
type titleStore struct {
	stubBins
	titles chan string
}

func (s *titleStore) SetSessionTitle(_, title string) (bool, error) {
	s.titles <- title
	return true, nil
}

// plantKiroSession writes one conversation's metadata where kiroSessionPath
// looks for it, under a throwaway home.
func plantKiroSession(t *testing.T, providerSessionID, body string) {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	dir := filepath.Join(home, ".kiro", "sessions", "cli")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, providerSessionID+".json"), []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
}

// TestKiroTitlePublishedOnStop is the whole feature: Kiro reports no title, so
// the turn's end is where lich reads the one Kiro filed itself and sends it
// down the same path a reported title takes.
func TestKiroTitlePublishedOnStop(t *testing.T) {
	const id = "kiro-uuid"
	plantKiroSession(t, id, `{"session_id":"`+id+`","title":"Port the PTY seam to Windows"}`)
	store := &titleStore{stubBins: stubBins{providerSession: id}, titles: make(chan string, 1)}
	svc := New(store, nil, events.New())
	svc.spawns.Store("s1", spawn{kind: providers.Kiro})

	svc.onHookState(hookRequest{SessionID: "s1", State: statusDone})

	select {
	case got := <-store.titles:
		if got != "Port the PTY seam to Windows" {
			t.Errorf("title = %q, want %q", got, "Port the PTY seam to Windows")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("the turn ended and no title reached the store")
	}
}

// A tool call is not the turn's end. Kiro reports `busy` once per tool, and
// reading the metadata on each of them would buy a name lich already has at
// the cost of a file read per tool call.
func TestKiroTitleNotReadMidTurn(t *testing.T) {
	const id = "kiro-uuid"
	plantKiroSession(t, id, `{"session_id":"`+id+`","title":"Port the PTY seam to Windows"}`)
	store := &titleStore{stubBins: stubBins{providerSession: id}, titles: make(chan string, 1)}
	svc := New(store, nil, events.New())
	svc.spawns.Store("s1", spawn{kind: providers.Kiro})

	svc.onHookState(hookRequest{SessionID: "s1", State: statusBusy})

	select {
	case got := <-store.titles:
		t.Errorf("a mid-turn report titled the card %q", got)
	case <-time.After(200 * time.Millisecond):
	}
}

// The resolution every other session must miss on: the file is Kiro's, and a
// session with no conversation linked to it yet has no file to name.
func TestKiroTitleResolution(t *testing.T) {
	const id = "kiro-uuid"
	plantKiroSession(t, id, `{"session_id":"`+id+`","title":"Port the PTY seam to Windows"}`)
	tests := []struct {
		name            string
		kind            string
		providerSession string
		want            string
	}{
		{"a Kiro session naming its conversation", providers.Kiro, id, "Port the PTY seam to Windows"},
		{"another provider's session", providers.Claude, id, ""},
		{"a shell", KindShell, id, ""},
		{"before session-start linked a conversation", providers.Kiro, "", ""},
		{"a conversation whose file was pruned", providers.Kiro, "gone-uuid", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := New(stubBins{providerSession: tt.providerSession}, nil, events.New())
			svc.spawns.Store("s1", spawn{kind: tt.kind})
			got, ok := svc.kiroTitle("s1")
			if ok != (tt.want != "") {
				t.Fatalf("kiroTitle = %q, %v, want ok=%v", got, ok, tt.want != "")
			}
			if got != tt.want {
				t.Errorf("kiroTitle = %q, want %q", got, tt.want)
			}
		})
	}
}
