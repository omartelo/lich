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
	writes   map[string][]string
	writeErr error
}

func newFakeTerminal(live ...string) *fakeTerminal {
	t := &fakeTerminal{live: map[string]bool{}, writes: map[string][]string{}}
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
	f.writes[id] = append(f.writes[id], data)
	return nil
}

// writesTo is every write a session received, in order — the submit is its own,
// so a test can tell "pasted and sent" from "pasted and left sitting there".
func (f *fakeTerminal) writesTo(id string) []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]string(nil), f.writes[id]...)
}

func (f *fakeTerminal) written(id string) string {
	return strings.Join(f.writesTo(id), "")
}

// fakeEvents records what the relay announced to the window, in order.
type fakeEvents struct {
	mu    sync.Mutex
	sent  []RelayEvent
	halts []StalledEvent
}

func (f *fakeEvents) Emit(name string, data any) {
	f.mu.Lock()
	defer f.mu.Unlock()
	switch event := data.(type) {
	case RelayEvent:
		if name == RelayEventName {
			f.sent = append(f.sent, event)
		}
	case StalledEvent:
		if name == StalledEventName {
			f.halts = append(f.halts, event)
		}
	}
}

func (f *fakeEvents) stalled() []StalledEvent {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]StalledEvent(nil), f.halts...)
}

func (f *fakeEvents) snapshot() []RelayEvent {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]RelayEvent(nil), f.sent...)
}

// markOf is the last thing announced about one session, which is what its card
// would be showing.
func (f *fakeEvents) markOf(id string) (RelayEvent, bool) {
	events := f.snapshot()
	for i := len(events) - 1; i >= 0; i-- {
		if events[i].ID == id {
			return events[i], true
		}
	}
	return RelayEvent{}, false
}

// awaitMark blocks until one session's mark matches want, and returns whether
// it did. Marks are raised after the message is in the PTY, which is later than
// the ticket appearing in the map — waiting on the ticket instead would read
// the marks before they exist.
func (f *fakeEvents) awaitMark(id, direction string) bool {
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if mark, ok := f.markOf(id); ok && mark.Direction == direction {
			return true
		}
		time.Sleep(time.Millisecond)
	}
	return false
}

// newRelay is New with the paste-to-Enter delay dropped: the fakes have no TUI
// to settle, and paying it in every test would buy nothing.
func newRelay(sessions Sessions, term Terminal, events Events) *Service {
	svc := New(sessions, term, events)
	svc.submitDelay = 0
	return svc
}

// workspace is the roster most tests run against: two projects, one label
// ("api") deliberately repeated across them.
func workspace() fakeSessions {
	return fakeSessions{projects: []store.Project{
		{ID: "p1", Name: "lich", Path: "/src/lich", Sessions: []store.Session{
			{ID: "s1", Label: "sender", Kind: "claude"},
			{ID: "s2", Label: "docs", Kind: "codex"},
			{ID: "s3", Label: "api", Kind: "opencode"},
			{ID: "s4", Label: "parked", Kind: "claude"},
		}},
		{ID: "p2", Name: "revu", Path: "/src/revu", Sessions: []store.Session{
			{ID: "s5", Label: "api", Kind: "crush"},
		}},
	}}
}

func TestPeersListsLiveSessionsWithoutTheCaller(t *testing.T) {
	svc := newRelay(workspace(), newFakeTerminal("s1", "s2", "s3", "s5"), nil)

	peers, err := svc.Peers("s1")
	if err != nil {
		t.Fatalf("Peers: %v", err)
	}

	// Both names travel: an agent that sees only one of them treats the other
	// as a different session, which is what sent the first real run down two
	// channels at once.
	want := []Peer{
		{Label: "docs", Name: "lich-s2", Project: "lich", Kind: "codex"},
		{Label: "api", Name: "lich-s3", Project: "lich", Kind: "opencode"},
		{Label: "api", Name: "revu-s5", Project: "revu", Kind: "crush"},
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
	svc := newRelay(fakeSessions{err: errors.New("database is locked")}, newFakeTerminal(), nil)

	if _, err := svc.Peers("s1"); err == nil {
		t.Fatal("want an error when the workspace cannot be read")
	}
}

func TestSendDeliversAndReplyAnswers(t *testing.T) {
	term := newFakeTerminal("s1", "s2")
	svc := newRelay(workspace(), term, nil)

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

	// Two writes, and in this order. Claude Code collapses a multi-line paste
	// into a placeholder, and an Enter riding in that same burst is swallowed —
	// the message then sits at the target's prompt unsent, which is exactly what
	// the first real runs produced.
	writes := term.writesTo("s2")
	if len(writes) != 2 {
		t.Fatalf("got %d writes, want the paste and the submit apart: %q", len(writes), writes)
	}
	if !strings.HasPrefix(writes[0], "\x1b[200~") || !strings.HasSuffix(writes[0], "\x1b[201~") {
		t.Errorf("the message was not bracketed-pasted: %q", writes[0])
	}
	if strings.Contains(writes[0], "\r") {
		t.Errorf("an Enter rode along with the paste: %q", writes[0])
	}
	if writes[1] != "\r" {
		t.Errorf("second write = %q, want the submit alone", writes[1])
	}
	typed := term.written("s2")
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
	svc := newRelay(workspace(), term, nil)

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
			// The first real run lost an answer to a target that replied over
			// its provider's own peer channel. Saying the ticket is the only
			// route is the whole fix, so it is pinned rather than left to taste.
			if !strings.Contains(got, "only way back") {
				t.Errorf("the instruction does not say the ticket is exclusive:\n%s", got)
			}
			if !strings.Contains(got, "Do not answer by messaging a peer session") {
				t.Errorf("the instruction does not rule out the peer channel:\n%s", got)
			}
		})
	}
}

// TestARosterNameReachesTheSameSession proves the two names for one session are
// one address space now. A mention writes the roster name at a prompt; if the
// agent hands that to lich instead of to its own messaging, it must land on the
// session it names rather than on an error — that mismatch is what made an
// agent use both channels at once on the first real run.
func TestARosterNameReachesTheSameSession(t *testing.T) {
	term := newFakeTerminal("s1", "s2")
	svc := newRelay(workspace(), term, nil)

	go func() { _ = svc.Reply(waitForTicket(svc), "ok") }()
	got, err := svc.Send("s1", RosterName("/src/lich", "s2"), "", "hello", 30)
	if err != nil {
		t.Fatalf("Send by roster name: %v", err)
	}
	if got.Target != "docs" {
		t.Errorf("target = %q, want the card label", got.Target)
	}
	if term.written("s2") == "" {
		t.Error("the session the roster name points at was never typed at")
	}
}

// TestTheLabelWinsARosterCollision proves the tie-break: a label is the name the
// user chose and the one every message quotes, so it outranks a roster name that
// happens to read the same.
func TestTheLabelWinsARosterCollision(t *testing.T) {
	collide := fakeSessions{projects: []store.Project{{
		ID: "p1", Name: "lich", Path: "/src/lich",
		Sessions: []store.Session{
			{ID: "s1", Label: "sender", Kind: "claude"},
			// s3's roster name is "lich-s3"; s2 carries it as its label.
			{ID: "s2", Label: "lich-s3", Kind: "codex"},
			{ID: "s3", Label: "docs", Kind: "codex"},
		},
	}}}
	term := newFakeTerminal("s1", "s2", "s3")
	svc := newRelay(collide, term, nil)

	go func() { _ = svc.Reply(waitForTicket(svc), "ok") }()
	if _, err := svc.Send("s1", "lich-s3", "", "hello", 30); err != nil {
		t.Fatalf("Send: %v", err)
	}
	if term.written("s2") == "" {
		t.Error("the session whose label matched was not the one written to")
	}
	if term.written("s3") != "" {
		t.Error("the roster name won over a label")
	}
}

// TestAnUnknownNameNamesWhatIsReachable proves a miss is one step from a hit:
// both names of every live session come back with the error.
func TestAnUnknownNameNamesWhatIsReachable(t *testing.T) {
	svc := newRelay(workspace(), newFakeTerminal("s1", "s2"), nil)

	_, err := svc.Send("s1", "ghost", "", "hello", 30)
	if err == nil {
		t.Fatal("want an error for an unknown name")
	}
	for _, want := range []string{`"docs"`, "lich-s2"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error is missing %q: %v", want, err)
		}
	}
}

// TestWithNothingReachableTheErrorSaysSo proves the empty case reads as a fact
// about the workspace rather than as a bad name.
func TestWithNothingReachableTheErrorSaysSo(t *testing.T) {
	svc := newRelay(workspace(), newFakeTerminal("s1"), nil)

	_, err := svc.Send("s1", "docs", "", "hello", 30)
	if err == nil {
		t.Fatal("want an error when nothing is live")
	}
	if !strings.Contains(err.Error(), "No other session is live.") {
		t.Errorf("error = %v", err)
	}
}

// TestBothCardsAreMarkedAndCleared proves the sidebar can tell the whole story
// from one errand: the target learns who asked, the sender learns who it is
// waiting on, and both marks come down when the answer lands.
func TestBothCardsAreMarkedAndCleared(t *testing.T) {
	events := &fakeEvents{}
	svc := newRelay(workspace(), newFakeTerminal("s1", "s2"), events)

	done := make(chan struct{})
	go func() {
		defer close(done)
		_, _ = svc.Send("s1", "docs", "", "run the tests", 30)
	}()
	ticketID := waitForTicket(svc)

	if !events.awaitMark("s2", DirectionIn) {
		t.Fatal("the target was never marked as asked")
	}
	if !events.awaitMark("s1", DirectionOut) {
		t.Fatal("the sender was never marked as waiting")
	}
	if target, _ := events.markOf("s2"); target.Peer != "sender" {
		t.Errorf("target mark names %q, want the sender's label", target.Peer)
	}
	if sender, _ := events.markOf("s1"); sender.Peer != "docs" {
		t.Errorf("sender mark names %q, want the target's label", sender.Peer)
	}

	if err := svc.Reply(ticketID, "3 failures"); err != nil {
		t.Fatalf("Reply: %v", err)
	}
	<-done

	for _, id := range []string{"s1", "s2"} {
		mark, _ := events.markOf(id)
		if mark.Direction != "" || mark.Peer != "" {
			t.Errorf("session %q kept the mark %+v after the answer", id, mark)
		}
	}
}

// TestAnExternalSenderMarksOnlyTheTarget proves a caller with no card leaves no
// mark of its own — there is no session to put one on — while the target still
// learns a request arrived.
func TestAnExternalSenderMarksOnlyTheTarget(t *testing.T) {
	events := &fakeEvents{}
	svc := newRelay(workspace(), newFakeTerminal("s2"), events)

	go func() {
		if !events.awaitMark("s2", DirectionIn) {
			return
		}
		_ = svc.Reply(waitForTicket(svc), "ok")
	}()
	if _, err := svc.Send("", "docs", "", "hello", 30); err != nil {
		t.Fatalf("Send: %v", err)
	}

	sent := events.snapshot()
	if len(sent) == 0 {
		t.Fatal("the target was never marked")
	}
	for _, event := range sent {
		if event.ID != "s2" {
			t.Errorf("a session that does not exist was marked: %+v", event)
		}
	}
	// The peer is empty rather than invented: the card is what words it.
	if sent[0].Direction != DirectionIn || sent[0].Peer != "" {
		t.Errorf("target mark = %+v, want an inbound one with no peer label", sent[0])
	}
}

// TestAnUndeliveredMessageMarksNobody proves the mark follows the bytes: a write
// that failed leaves no card claiming a request that never arrived.
func TestAnUndeliveredMessageMarksNobody(t *testing.T) {
	events := &fakeEvents{}
	term := newFakeTerminal("s1", "s2")
	term.writeErr = errors.New("pty closed")
	svc := newRelay(workspace(), term, events)

	if _, err := svc.Send("s1", "docs", "", "hello", 30); err == nil {
		t.Fatal("want an error when the message cannot be typed")
	}
	if got := events.snapshot(); len(got) != 0 {
		t.Errorf("an undelivered message marked %+v", got)
	}
}

// TestAnExpiredTicketTakesItsMarksDown proves a request nobody ever answered
// stops claiming both cards once it is swept, rather than marking them for the
// life of the process.
func TestAnExpiredTicketTakesItsMarksDown(t *testing.T) {
	events := &fakeEvents{}
	svc := newRelay(workspace(), newFakeTerminal("s1", "s2"), events)
	svc.tickets["stale"] = &ticket{
		fromID:   "s1",
		sender:   "sender",
		targetID: "s2",
		target:   "docs",
		created:  time.Now().Add(-2 * time.Hour),
		done:     make(chan struct{}),
	}

	if _, err := svc.Wait("missing", 1); err == nil {
		t.Fatal("want an error, and the sweep it triggers")
	}
	for _, id := range []string{"s1", "s2"} {
		mark, ok := events.markOf(id)
		if !ok {
			t.Errorf("session %q was never cleared", id)
			continue
		}
		if mark.Direction != "" {
			t.Errorf("session %q kept the mark %+v after the sweep", id, mark)
		}
	}
}

// TestATurnThatEndsWithoutAnAnswerEndsTheWait proves the exception this feature
// has to survive: the target worked and then answered somewhere lich cannot
// read. The wait ends there, saying so, instead of running its full clock out.
func TestATurnThatEndsWithoutAnAnswerEndsTheWait(t *testing.T) {
	events := &fakeEvents{}
	svc := newRelay(workspace(), newFakeTerminal("s1", "s2"), events)

	done := make(chan Result, 1)
	go func() {
		got, _ := svc.Send("s1", "docs", "", "run the tests", 30)
		done <- got
	}()
	waitForTicket(svc)
	if !events.awaitMark("s2", DirectionIn) {
		t.Fatal("the target was never marked, so the message never landed")
	}

	svc.Observe("s2", "busy")
	svc.Observe("s2", "done")

	got := <-done
	if got.Status != StatusUnanswered {
		t.Fatalf("status = %q, want %q", got.Status, StatusUnanswered)
	}
	if got.Answer != "" {
		t.Errorf("an unanswered result carried an answer: %q", got.Answer)
	}
	for _, id := range []string{"s1", "s2"} {
		if mark, _ := events.markOf(id); mark.Direction != "" {
			t.Errorf("session %q kept the mark %+v", id, mark)
		}
	}
}

// TestAQueuedRequestIgnoresTheTurnAlreadyRunning proves the guard that keeps
// this honest. A message handed to a busy session waits its turn, so the ending
// of the turn in progress says nothing — reporting it as unanswered would
// accuse a target that had not read the request yet.
func TestAQueuedRequestIgnoresTheTurnAlreadyRunning(t *testing.T) {
	svc := newRelay(workspace(), newFakeTerminal("s1", "s2"), nil)
	svc.Observe("s2", "busy")

	done := make(chan Result, 1)
	go func() {
		got, _ := svc.Send("s1", "docs", "", "run the tests", 2)
		done <- got
	}()
	ticketID := waitForTicket(svc)

	// The turn that was already running ends. Nothing about this request.
	svc.Observe("s2", "done")
	select {
	case got := <-done:
		t.Fatalf("the wait ended on someone else's turn: %+v", got)
	case <-time.After(50 * time.Millisecond):
	}

	// Now the queued request runs, and ends without an answer.
	svc.Observe("s2", "busy")
	svc.Observe("s2", "done")
	if got := <-done; got.Status != StatusUnanswered {
		t.Fatalf("status = %q, want %q", got.Status, StatusUnanswered)
	}
	if err := svc.Reply(ticketID, "late"); err == nil {
		t.Error("the ticket outlived the turn that closed it")
	}
}

// TestAnAnsweredTicketIsNotAlsoStalled proves the ordering that matters most: a
// target that replies and ends its turn in the same breath has answered, and a
// race between the two must not turn an answer into a shrug.
func TestAnAnsweredTicketIsNotAlsoStalled(t *testing.T) {
	svc := newRelay(workspace(), newFakeTerminal("s1", "s2"), nil)

	done := make(chan Result, 1)
	go func() {
		got, _ := svc.Send("s1", "docs", "", "hello", 30)
		done <- got
	}()
	ticketID := waitForTicket(svc)

	svc.Observe("s2", "busy")
	if err := svc.Reply(ticketID, "3 failures"); err != nil {
		t.Fatalf("Reply: %v", err)
	}
	svc.Observe("s2", "done")

	got := <-done
	if got.Status != StatusAnswered || got.Answer != "3 failures" {
		t.Fatalf("result = %+v, want the answer", got)
	}
}

// TestASessionThatEndedStopsTheWaitOutright proves SessionEnd needs no turn:
// the CLI has left the PTY, so nothing there will ever answer.
func TestASessionThatEndedStopsTheWaitOutright(t *testing.T) {
	events := &fakeEvents{}
	svc := newRelay(workspace(), newFakeTerminal("s1", "s2"), events)

	done := make(chan Result, 1)
	go func() {
		got, _ := svc.Send("s1", "docs", "", "hello", 30)
		done <- got
	}()
	waitForTicket(svc)
	svc.Observe("s2", "idle")

	if got := <-done; got.Status != StatusUnanswered {
		t.Fatalf("status = %q, want %q", got.Status, StatusUnanswered)
	}
	stalls := 0
	for _, event := range events.stalled() {
		if event.TargetID == "s2" && event.ID == "s1" && event.Target == "docs" {
			stalls++
		}
	}
	if stalls != 1 {
		t.Errorf("the window was told %d times, want once", stalls)
	}
}

// TestAnotherSessionsTurnLeavesTheTicketAlone proves the ticket only answers to
// its own target: a busy neighbour must not close it.
func TestAnotherSessionsTurnLeavesTheTicketAlone(t *testing.T) {
	svc := newRelay(workspace(), newFakeTerminal("s1", "s2", "s3"), nil)

	done := make(chan Result, 1)
	go func() {
		got, _ := svc.Send("s1", "docs", "", "hello", 1)
		done <- got
	}()
	waitForTicket(svc)

	svc.Observe("s3", "busy")
	svc.Observe("s3", "done")

	if got := <-done; got.Status != StatusPending {
		t.Fatalf("status = %q, want %q — a neighbour closed the ticket", got.Status, StatusPending)
	}
}

// TestAnAnswerNobodyWaitedForIsTypedAtTheSendersPrompt proves the path that
// makes a long errand work at all: the asker stops holding the line, the answer
// arrives later, and it comes back the same way the request went out.
func TestAnAnswerNobodyWaitedForIsTypedAtTheSendersPrompt(t *testing.T) {
	term := newFakeTerminal("s1", "s2")
	svc := newRelay(workspace(), term, nil)

	got, err := svc.Send("s1", "docs", "", "run the tests", 1)
	if err != nil {
		t.Fatalf("Send: %v", err)
	}
	if got.Status != StatusPending {
		t.Fatalf("status = %q, want the wait to run out first", got.Status)
	}
	if term.written("s1") != "" {
		t.Fatalf("the sender was typed at before any answer existed: %q", term.written("s1"))
	}

	if err := svc.Reply(got.Ticket, "3 failures in foo_test"); err != nil {
		t.Fatalf("Reply: %v", err)
	}

	writes := term.writesTo("s1")
	if len(writes) != 2 {
		t.Fatalf("got %d writes to the sender, want the paste and the submit: %q", len(writes), writes)
	}
	if writes[1] != "\r" {
		t.Errorf("the answer was left sitting at the prompt: %q", writes)
	}
	typed := term.written("s1")
	for _, want := range []string{`Answer from session "docs"`, "3 failures in foo_test"} {
		if !strings.Contains(typed, want) {
			t.Errorf("returned answer is missing %q:\n%s", want, typed)
		}
	}
}

// TestAnAnswerSomeoneIsWaitingForIsNotAlsoTyped proves the other half: a caller
// still holding the line carries the answer out itself, and typing it at the
// prompt as well would deliver it twice.
func TestAnAnswerSomeoneIsWaitingForIsNotAlsoTyped(t *testing.T) {
	term := newFakeTerminal("s1", "s2")
	svc := newRelay(workspace(), term, nil)

	done := make(chan Result, 1)
	go func() {
		got, _ := svc.Send("s1", "docs", "", "hello", 30)
		done <- got
	}()
	ticketID := waitForTicket(svc)
	if err := svc.Reply(ticketID, "all green"); err != nil {
		t.Fatalf("Reply: %v", err)
	}

	if got := <-done; got.Answer != "all green" {
		t.Fatalf("the waiter did not get the answer: %+v", got)
	}
	if typed := term.written("s1"); typed != "" {
		t.Errorf("the answer was also typed at a prompt that was already reading it: %q", typed)
	}
}

// TestAnExternalSenderHasNoPromptToAnswerAt proves the one case with nowhere to
// return to: the CLI run from a script is not a session, so an answer nobody
// waited for is simply not delivered rather than typed at a stranger.
func TestAnExternalSenderHasNoPromptToAnswerAt(t *testing.T) {
	term := newFakeTerminal("s2")
	svc := newRelay(workspace(), term, nil)

	got, err := svc.Send("", "docs", "", "hello", 1)
	if err != nil {
		t.Fatalf("Send: %v", err)
	}
	if err := svc.Reply(got.Ticket, "all green"); err != nil {
		t.Fatalf("Reply: %v", err)
	}

	for id, writes := range term.writes {
		if id != "s2" {
			t.Errorf("session %q was typed at: %q", id, writes)
		}
	}
}

func TestSendRefusesAnAmbiguousLabel(t *testing.T) {
	svc := newRelay(workspace(), newFakeTerminal("s1", "s3", "s5"), nil)

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
	svc := newRelay(workspace(), term, nil)

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
	svc := newRelay(workspace(), newFakeTerminal("s1", "s2"), nil)

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
	svc := newRelay(workspace(), term, nil)

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
	svc := newRelay(workspace(), newFakeTerminal("s1", "s2"), nil)

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
	svc := newRelay(workspace(), term, nil)

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
	svc := newRelay(workspace(), newFakeTerminal(), nil)

	if _, err := svc.Wait("deadbeef", 1); err == nil {
		t.Fatal("want an error waiting on a ticket that never existed")
	}
}

func TestExpiredTicketsAreSweptAway(t *testing.T) {
	svc := newRelay(workspace(), newFakeTerminal("s1", "s2"), nil)
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
	svc := newRelay(workspace(), term, nil)

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
	svc := newRelay(workspace(), newFakeTerminal("s1", "s2"), nil)

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
