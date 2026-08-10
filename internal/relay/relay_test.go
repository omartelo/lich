package relay

import (
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/omartelo/lich/internal/store"
)

// fakeSessions is the persisted workspace a test declares inline.
type fakeSessions struct {
	projects []store.Project
	err      error
}

func (f fakeSessions) LoadState() ([]store.Project, error) { return f.projects, f.err }

// fakeTerminal records what was typed at which session, and answers Live from
// an explicit set so a test can park a session without a PTY.
type fakeTerminal struct {
	mu       sync.Mutex
	live     map[string]bool
	writes   map[string]string
	writeErr error
}

func newFakeTerminal(live ...string) *fakeTerminal {
	t := &fakeTerminal{live: map[string]bool{}, writes: map[string]string{}}
	for _, id := range live {
		t.live[id] = true
	}
	return t
}

func (f *fakeTerminal) Live(id string) bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.live[id]
}

func (f *fakeTerminal) Write(id, data string) error {
	if f.writeErr != nil {
		return f.writeErr
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	f.writes[id] += data
	return nil
}

func (f *fakeTerminal) written(id string) string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.writes[id]
}

// workspace is the roster most tests run against: two projects, one label
// ("api") deliberately repeated across them.
func workspace() fakeSessions {
	return fakeSessions{projects: []store.Project{
		{ID: "p1", Name: "lich", Sessions: []store.Session{
			{ID: "s1", Label: "sender", Kind: "claude"},
			{ID: "s2", Label: "docs", Kind: "codex"},
			{ID: "s3", Label: "api", Kind: "opencode"},
			{ID: "s4", Label: "parked", Kind: "claude"},
		}},
		{ID: "p2", Name: "revu", Sessions: []store.Session{
			{ID: "s5", Label: "api", Kind: "crush"},
		}},
	}}
}

func TestPeersListsLiveSessionsWithoutTheCaller(t *testing.T) {
	svc := New(workspace(), newFakeTerminal("s1", "s2", "s3", "s5"))

	peers, err := svc.Peers("s1")
	if err != nil {
		t.Fatalf("Peers: %v", err)
	}

	want := []Peer{
		{Label: "docs", Project: "lich", Kind: "codex"},
		{Label: "api", Project: "lich", Kind: "opencode"},
		{Label: "api", Project: "revu", Kind: "crush"},
	}
	if len(peers) != len(want) {
		t.Fatalf("got %d peers %v, want %d", len(peers), peers, len(want))
	}
	for i := range want {
		if peers[i] != want[i] {
			t.Errorf("peer %d = %+v, want %+v", i, peers[i], want[i])
		}
	}
}

func TestPeersReportsAStoreFailure(t *testing.T) {
	svc := New(fakeSessions{err: errors.New("database is locked")}, newFakeTerminal())

	if _, err := svc.Peers("s1"); err == nil {
		t.Fatal("want an error when the workspace cannot be read")
	}
}

func TestSendDeliversAndReplyAnswers(t *testing.T) {
	term := newFakeTerminal("s1", "s2")
	svc := New(workspace(), term)

	var (
		got Result
		err error
		wg  sync.WaitGroup
	)
	wg.Add(1)
	go func() {
		defer wg.Done()
		got, err = svc.Send("s1", "docs", "", "run the tests", 30)
	}()

	ticketID := waitForTicket(svc)
	if err := svc.Reply(ticketID, "3 failures in foo_test"); err != nil {
		t.Fatalf("Reply: %v", err)
	}
	wg.Wait()

	if err != nil {
		t.Fatalf("Send: %v", err)
	}
	if got.Status != StatusAnswered {
		t.Errorf("status = %q, want %q", got.Status, StatusAnswered)
	}
	if got.Answer != "3 failures in foo_test" {
		t.Errorf("answer = %q", got.Answer)
	}
	if got.Target != "docs" {
		t.Errorf("target = %q, want %q", got.Target, "docs")
	}

	typed := term.written("s2")
	if !strings.HasPrefix(typed, "\x1b[200~") || !strings.HasSuffix(typed, "\x1b[201~\r") {
		t.Errorf("message was not bracketed-pasted and submitted: %q", typed)
	}
	for _, want := range []string{`"sender"`, "run the tests", `"$LICH_BIN" reply ` + ticketID} {
		if !strings.Contains(typed, want) {
			t.Errorf("typed message is missing %q:\n%s", want, typed)
		}
	}
	if other := term.written("s1"); other != "" {
		t.Errorf("the sender's own session was typed at: %q", other)
	}
}

func TestAMessageFromOutsideLichIsAttributedToTheCommandLine(t *testing.T) {
	term := newFakeTerminal("s2")
	svc := New(workspace(), term)

	go func() { _ = svc.Reply(waitForTicket(svc), "ok") }()
	if _, err := svc.Send("", "docs", "", "hello", 30); err != nil {
		t.Fatalf("Send: %v", err)
	}

	typed := term.written("s2")
	if !strings.Contains(typed, "Message relayed by the lich command line") {
		t.Errorf("a caller with no session was announced as one:\n%s", typed)
	}
	if strings.Contains(typed, "unknown") {
		t.Errorf("an external caller was named as an unknown session:\n%s", typed)
	}
}

// TestReplyInstructionOffersTheToolOnlyWhereItExists proves the message names
// the MCP tool exactly for the providers lich registers its server with, and
// always names the command — an agent whose shell is locked down could
// otherwise have no way to answer at all.
func TestReplyInstructionOffersTheToolOnlyWhereItExists(t *testing.T) {
	tests := []struct {
		kind string
		tool bool
	}{
		{"claude", true},
		{"codex", true},
		{"opencode", false},
		{"crush", false},
		{"omp", false},
		{"shell", false},
	}
	for _, tt := range tests {
		t.Run(tt.kind, func(t *testing.T) {
			got := replyInstruction(tt.kind, "a1b2c3d4")
			if !strings.Contains(got, `"$LICH_BIN" reply a1b2c3d4`) {
				t.Errorf("the command is missing:\n%s", got)
			}
			if named := strings.Contains(got, ToolReply); named != tt.tool {
				t.Errorf("names the %s tool = %v, want %v:\n%s", ToolReply, named, tt.tool, got)
			}
		})
	}
}

func TestSendRefusesAnAmbiguousLabel(t *testing.T) {
	svc := New(workspace(), newFakeTerminal("s1", "s3", "s5"))

	_, err := svc.Send("s1", "api", "", "hello", 30)
	if err == nil {
		t.Fatal("want an error for a label two live sessions answer to")
	}
	for _, want := range []string{"lich", "revu"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error should name project %q: %v", want, err)
		}
	}
}

func TestSendNarrowsAnAmbiguousLabelByProject(t *testing.T) {
	term := newFakeTerminal("s1", "s3", "s5")
	svc := New(workspace(), term)

	go func() {
		ticketID := waitForTicket(svc)
		_ = svc.Reply(ticketID, "ok")
	}()
	got, err := svc.Send("s1", "api", "revu", "hello", 30)
	if err != nil {
		t.Fatalf("Send: %v", err)
	}
	if got.Answer != "ok" {
		t.Fatalf("answer = %q", got.Answer)
	}
	if term.written("s5") == "" {
		t.Error("the revu session was never typed at")
	}
	if term.written("s3") != "" {
		t.Error("the lich session was typed at despite the project narrowing")
	}
}

func TestSendRejectsUnreachableTargets(t *testing.T) {
	svc := New(workspace(), newFakeTerminal("s1", "s2"))

	tests := []struct {
		name   string
		target string
	}{
		{"no such session", "ghost"},
		{"session with no pty", "parked"},
		{"the caller itself", "sender"},
		{"empty target", "  "},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := svc.Send("s1", tt.target, "", "hello", 30); err == nil {
				t.Fatalf("want an error for target %q", tt.target)
			}
		})
	}
}

func TestSendReportsADeliveryFailure(t *testing.T) {
	term := newFakeTerminal("s1", "s2")
	term.writeErr = errors.New("pty closed")
	svc := New(workspace(), term)

	if _, err := svc.Send("s1", "docs", "", "hello", 30); err == nil {
		t.Fatal("want an error when the message cannot be typed")
	}
	svc.mu.Lock()
	open := len(svc.tickets)
	svc.mu.Unlock()
	if open != 0 {
		t.Errorf("a ticket outlived its undelivered message: %d open", open)
	}
}

func TestSendPendsWhenTheWaitRunsOutAndWaitPicksItUp(t *testing.T) {
	svc := New(workspace(), newFakeTerminal("s1", "s2"))

	got, err := svc.Send("s1", "docs", "", "long errand", 1)
	if err != nil {
		t.Fatalf("Send: %v", err)
	}
	if got.Status != StatusPending {
		t.Fatalf("status = %q, want %q", got.Status, StatusPending)
	}
	if got.Answer != "" {
		t.Errorf("a pending result carried an answer: %q", got.Answer)
	}

	go func() {
		time.Sleep(10 * time.Millisecond)
		_ = svc.Reply(got.Ticket, "late but here")
	}()
	again, err := svc.Wait(got.Ticket, 30)
	if err != nil {
		t.Fatalf("Wait: %v", err)
	}
	if again.Status != StatusAnswered || again.Answer != "late but here" {
		t.Errorf("Wait returned %+v", again)
	}
}

func TestReplyRefusesUnknownAndRepeatedTickets(t *testing.T) {
	term := newFakeTerminal("s1", "s2")
	svc := New(workspace(), term)

	if err := svc.Reply("deadbeef", "hello"); err == nil {
		t.Error("want an error replying to a ticket that never existed")
	}

	done := make(chan Result, 1)
	go func() {
		got, _ := svc.Send("s1", "docs", "", "hello", 30)
		done <- got
	}()
	ticketID := waitForTicket(svc)
	if err := svc.Reply(ticketID, "first"); err != nil {
		t.Fatalf("Reply: %v", err)
	}
	got := <-done
	if got.Answer != "first" {
		t.Fatalf("answer = %q", got.Answer)
	}
	if err := svc.Reply(ticketID, "second"); err == nil {
		t.Error("want an error replying to an answered ticket")
	}
}

func TestWaitRefusesAnUnknownTicket(t *testing.T) {
	svc := New(workspace(), newFakeTerminal())

	if _, err := svc.Wait("deadbeef", 1); err == nil {
		t.Fatal("want an error waiting on a ticket that never existed")
	}
}

func TestExpiredTicketsAreSweptAway(t *testing.T) {
	svc := New(workspace(), newFakeTerminal("s1", "s2"))
	svc.tickets["stale"] = &ticket{
		target:  "docs",
		created: time.Now().Add(-2 * time.Hour),
		done:    make(chan struct{}),
	}

	if _, err := svc.Wait("missing", 1); err == nil {
		t.Fatal("want an error, and the sweep it triggers")
	}
	svc.mu.Lock()
	_, still := svc.tickets["stale"]
	svc.mu.Unlock()
	if still {
		t.Error("an hours-old ticket survived the sweep")
	}
}

func TestSendBoundsThePrompt(t *testing.T) {
	term := newFakeTerminal("s1", "s2")
	svc := New(workspace(), term)

	if _, err := svc.Send("s1", "docs", "", "", 30); err == nil {
		t.Error("want an error for an empty prompt")
	}
	if _, err := svc.Send("s1", "docs", "", "\x1b\x07", 30); err == nil {
		t.Error("want an error for a prompt that is only control characters")
	}
	if _, err := svc.Send("s1", "docs", "", strings.Repeat("a", 8193), 30); err == nil {
		t.Error("want an error for a prompt of 8193 characters")
	}

	go func() {
		ticketID := waitForTicket(svc)
		_ = svc.Reply(ticketID, "ok")
	}()
	if _, err := svc.Send("s1", "docs", "", strings.Repeat("a", 8192), 30); err != nil {
		t.Errorf("a prompt of 8192 characters was refused: %v", err)
	}
}

func TestReplyTruncatesAnOversizedAnswer(t *testing.T) {
	svc := New(workspace(), newFakeTerminal("s1", "s2"))

	done := make(chan Result, 1)
	go func() {
		got, _ := svc.Send("s1", "docs", "", "hello", 30)
		done <- got
	}()
	ticketID := waitForTicket(svc)
	if err := svc.Reply(ticketID, strings.Repeat("a", 65537)); err != nil {
		t.Fatalf("Reply: %v", err)
	}

	if got := <-done; len(got.Answer) != 65536 {
		t.Errorf("answer is %d characters, want it capped at 65536", len(got.Answer))
	}
}

func TestSanitizeKeepsWhitespaceAndDropsEscapes(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"keeps newlines and tabs", "one\ntwo\tthree", "one\ntwo\tthree"},
		{"drops the paste terminator", "before\x1b[201~after", "before[201~after"},
		{"drops a carriage return", "no\rsubmit", "nosubmit"},
		{"drops the bell", "ding\x07", "ding"},
		{"keeps unicode", "não — ok", "não — ok"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := sanitize(tt.in); got != tt.want {
				t.Errorf("sanitize(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestWaitForClampsTheRequestedWait(t *testing.T) {
	tests := []struct {
		name    string
		seconds int
		want    time.Duration
	}{
		{"zero falls back to the default", 0, DefaultWait},
		{"negative falls back to the default", -5, DefaultWait},
		{"a plain wait is honoured", 42, 42 * time.Second},
		{"an over-long wait is capped", 4000, MaxWait},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := waitFor(tt.seconds); got != tt.want {
				t.Errorf("waitFor(%d) = %v, want %v", tt.seconds, got, tt.want)
			}
		})
	}
}

func TestTicketIDsAreDistinct(t *testing.T) {
	seen := map[string]bool{}
	for range 100 {
		id, err := newTicketID()
		if err != nil {
			t.Fatalf("newTicketID: %v", err)
		}
		if seen[id] {
			t.Fatalf("ticket id %q was handed out twice", id)
		}
		seen[id] = true
	}
}

// waitForTicket blocks until Send has registered its ticket and returns its id.
// Send delivers the message before it starts waiting, so a test that replies
// has to let it get that far first.
func waitForTicket(svc *Service) string {
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		svc.mu.Lock()
		for id := range svc.tickets {
			svc.mu.Unlock()
			return id
		}
		svc.mu.Unlock()
		time.Sleep(time.Millisecond)
	}
	// Timed out: the caller's own assertion is what reports it.
	return ""
}
