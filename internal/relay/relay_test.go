package relay

import (
	"errors"
	"strings"
	"sync"
	"testing"
	"time"
	"unicode/utf8"

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
	mu sync.Mutex
	// live is whether a PTY exists; settingUp is whether what reads it is still
	// the worktree setup script rather than the agent.
	live      map[string]bool
	settingUp map[string]bool
	writes    map[string][]string
	writeErr  error
}

func newFakeTerminal(live ...string) *fakeTerminal {
	t := &fakeTerminal{
		live: map[string]bool{}, writes: map[string][]string{}, settingUp: map[string]bool{},
	}
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

// Ready defaults to "as ready as it is live": most tests are about delivery and
// answers, not about a checkout that is still installing its dependencies. The
// ones that are set settingUp.
func (f *fakeTerminal) Ready(id string) bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.live[id] && !f.settingUp[id]
}

// setUp marks a session as still running its worktree setup script: live, and
// with nothing on the other end that could read a message.
func (f *fakeTerminal) setUp(id string, running bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.settingUp[id] = running
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
	inbox []InboxEvent
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
	case InboxEvent:
		if name == InboxEventName {
			f.inbox = append(f.inbox, event)
		}
	}
}

func (f *fakeEvents) stalled() []StalledEvent {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]StalledEvent(nil), f.halts...)
}

// inboxCounts is every inbox size announced for one session, in order.
func (f *fakeEvents) inboxCounts(id string) []int {
	f.mu.Lock()
	defer f.mu.Unlock()
	var counts []int
	for _, e := range f.inbox {
		if e.ID == id {
			counts = append(counts, e.Count)
		}
	}
	return counts
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

// newRelay is New with the paste-to-Enter delay dropped — the fakes have no
// TUI to settle, and paying it in every test would buy nothing — and the nudge
// debounce shrunk for the same reason.
func newRelay(sessions Sessions, term Terminal, events Events) *Service {
	svc := New(sessions, term, events)
	svc.submitDelay = 0
	svc.nudgeDelay = time.Millisecond
	return svc
}

// awaitWritten blocks until want shows up in what a session was typed, and
// returns whether it did. The nudge rides a debounce timer, so a test asserting
// on it has to wait it out rather than race it.
func awaitWritten(term *fakeTerminal, id, want string) bool {
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if strings.Contains(term.written(id), want) {
			return true
		}
		time.Sleep(time.Millisecond)
	}
	return false
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
// the tool only for a session that has it, and always names the command — an
// agent whose shell is locked down could otherwise have no way to answer at
// all, and one pointed at a tool it does not have loses the turn to an error.
func TestReplyInstructionOffersTheToolOnlyWhereItExists(t *testing.T) {
	tests := []struct {
		name string
		tool bool
	}{
		{"a session with lich's tools", true},
		{"a session with none", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := replyInstruction(tt.tool, "a1b2c3d4")
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
			// The sender pays to read the answer, so the instruction pins the
			// shape: a report, not a transcript.
			if !strings.Contains(got, "concise report") {
				t.Errorf("the instruction does not ask for a concise report:\n%s", got)
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
	term := newFakeTerminal("s1", "s2")
	svc := newRelay(workspace(), term, events)

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
	// The waiter carried the news out itself; typing it at the prompt as well
	// would deliver it twice.
	if typed := term.written("s1"); typed != "" {
		t.Errorf("the stall was also typed at a prompt whose waiter already heard it: %q", typed)
	}
}

// The sender usually stops waiting long before the target's turn ends: it took
// a ticket and moved on, on the promise that news would reach its prompt. A
// turn that ends with no reply is that news — what arrives is the short nudge,
// and the outcome itself is collected, not typed.
func TestTheSenderIsToldAtItsPromptWhenTheTargetStallsUnattended(t *testing.T) {
	term := newFakeTerminal("s1", "s2")
	svc := newRelay(workspace(), term, nil)

	got, err := svc.Send("s1", "docs", "", "run the tests", 1)
	if err != nil {
		t.Fatalf("Send: %v", err)
	}
	if got.Status != StatusPending {
		t.Fatalf("status = %q, want the sender to have given up first", got.Status)
	}

	svc.Observe("s2", "busy")
	svc.Observe("s2", "done")

	if !awaitWritten(term, "s1", "[lich]") {
		t.Fatalf("the sender was never nudged: %q", term.written("s1"))
	}
	if typed := term.written("s1"); !strings.Contains(typed, `"docs"`) {
		t.Errorf("the nudge does not name who the news is from: %q", typed)
	}
	res, err := svc.Wait(got.Ticket, 1)
	if err != nil {
		t.Fatalf("Wait on the stalled ticket: %v", err)
	}
	if res.Status != StatusUnanswered {
		t.Errorf("status = %q, want %q collected off the inbox", res.Status, StatusUnanswered)
	}
}

// The stall and the give-up can also land together: Observe sees the waiter
// still attending, so nobody types the news, and the waiter's own timer fires
// in the same instant. The waiter that leaves last owns what is left behind —
// without that it hears "still working" about an errand already closed, on a
// ticket already gone from the map.
func TestAWaiterGivingUpAsTheErrandStallsHearsSo(t *testing.T) {
	svc := newRelay(workspace(), newFakeTerminal("s1", "s2"), nil)
	tk := &ticket{
		target:   "docs",
		done:     make(chan struct{}),
		stalled:  make(chan struct{}),
		unread:   make(chan struct{}),
		attended: 1,
	}
	close(tk.stalled)

	if got := svc.giveUp("tk1", tk); got != StatusUnanswered {
		t.Errorf("status = %q, want the caller told the target answered elsewhere", got)
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

// awaitDelivered blocks until n tickets have their message actually in a PTY.
// A ticket appears in the map before its delivery, so waiting on the map alone
// would let a test race the write it is about to assert on.
func awaitDelivered(t *testing.T, svc *Service, n int) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		svc.mu.Lock()
		count := 0
		for _, tk := range svc.tickets {
			if !tk.delivered.IsZero() {
				count++
			}
		}
		svc.mu.Unlock()
		if count == n {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatalf("never saw %d delivered tickets", n)
}

// TestADoneClosesOnlyTheTurnsOwnErrand proves a turn answers for one errand.
// Two messages delivered back to back — inside the second or two before the
// target's busy report lands — queue as two turns; the first turn's end must
// not report the second errand as answered elsewhere while its message is
// still queued, unread.
func TestADoneClosesOnlyTheTurnsOwnErrand(t *testing.T) {
	svc := newRelay(workspace(), newFakeTerminal("s1", "s2", "s3"), nil)

	first := make(chan Result, 1)
	go func() {
		got, _ := svc.Send("s1", "docs", "", "task one", 5)
		first <- got
	}()
	awaitDelivered(t, svc, 1)
	second := make(chan Result, 1)
	go func() {
		got, _ := svc.Send("s3", "docs", "", "task two", 5)
		second <- got
	}()
	awaitDelivered(t, svc, 2)

	// The target picks the first task up and ends that turn without replying.
	svc.Observe("s2", "busy")
	svc.Observe("s2", "done")
	if got := <-first; got.Status != StatusUnanswered {
		t.Fatalf("first errand = %q, want %q", got.Status, StatusUnanswered)
	}
	select {
	case got := <-second:
		t.Fatalf("the first turn's end closed the second errand too: %+v", got)
	case <-time.After(50 * time.Millisecond):
	}

	// The queued second task runs as its own turn.
	svc.Observe("s2", "busy")
	svc.Observe("s2", "done")
	if got := <-second; got.Status != StatusUnanswered {
		t.Fatalf("second errand = %q, want %q", got.Status, StatusUnanswered)
	}
}

// TestASecondErrandQueuedMidTurnSurvivesTheFirstsEnd is the same guarantee with
// the second message arriving while the first errand's turn is already running:
// its skipped turn is the same done that closes the first errand.
func TestASecondErrandQueuedMidTurnSurvivesTheFirstsEnd(t *testing.T) {
	svc := newRelay(workspace(), newFakeTerminal("s1", "s2", "s3"), nil)

	first := make(chan Result, 1)
	go func() {
		got, _ := svc.Send("s1", "docs", "", "task one", 5)
		first <- got
	}()
	awaitDelivered(t, svc, 1)
	svc.Observe("s2", "busy")

	second := make(chan Result, 1)
	go func() {
		got, _ := svc.Send("s3", "docs", "", "task two", 5)
		second <- got
	}()
	awaitDelivered(t, svc, 2)

	svc.Observe("s2", "done")
	if got := <-first; got.Status != StatusUnanswered {
		t.Fatalf("first errand = %q, want %q", got.Status, StatusUnanswered)
	}
	svc.Observe("s2", "busy")
	svc.Observe("s2", "done")
	if got := <-second; got.Status != StatusUnanswered {
		t.Fatalf("second errand = %q, want %q", got.Status, StatusUnanswered)
	}
}

// TestAnUndeliveredTicketIgnoresTheTargetsTurns proves turn accounting starts
// at delivery: a ticket still waiting out the target's setup script has no
// message in that PTY, so nothing that happens there can close it.
func TestAnUndeliveredTicketIgnoresTheTargetsTurns(t *testing.T) {
	svc := newRelay(workspace(), newFakeTerminal("s1", "s2"), nil)
	svc.tickets["held"] = &ticket{
		fromID: "s1", targetID: "s2", target: "docs",
		created: time.Now(),
		done:    make(chan struct{}),
		stalled: make(chan struct{}),
		unread:  make(chan struct{}),
	}

	svc.Observe("s2", "busy")
	svc.Observe("s2", "done")
	svc.Observe("s2", "idle")

	svc.mu.Lock()
	_, still := svc.tickets["held"]
	svc.mu.Unlock()
	if !still {
		t.Fatal("a turn on the target closed a ticket whose message was never delivered")
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

// TestAnAnswerNobodyWaitedForIsStashedAndNudged proves the path that makes a
// long errand work at all: the asker stops holding the line, the answer
// arrives later — and what reaches its prompt is one short nudge, never the
// answer itself. The full text comes back through Collect, inside a turn the
// sender chose, instead of restarting its turn as a prompt submission.
func TestAnAnswerNobodyWaitedForIsStashedAndNudged(t *testing.T) {
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

	if !awaitWritten(term, "s1", "[lich]") {
		t.Fatalf("the sender was never nudged: %q", term.written("s1"))
	}
	writes := term.writesTo("s1")
	if len(writes) != 2 {
		t.Fatalf("got %d writes to the sender, want the paste and the submit: %q", len(writes), writes)
	}
	if writes[1] != "\r" {
		t.Errorf("the nudge was left sitting at the prompt: %q", writes)
	}
	typed := term.written("s1")
	if strings.Contains(typed, "3 failures in foo_test") {
		t.Errorf("the full answer was typed at the prompt instead of held for Collect:\n%s", typed)
	}
	if !strings.Contains(typed, `"docs"`) {
		t.Errorf("the nudge does not name who answered: %q", typed)
	}

	collected, err := svc.Collect("s1", 1)
	if err != nil {
		t.Fatalf("Collect: %v", err)
	}
	if len(collected.Results) != 1 {
		t.Fatalf("collected %d results, want the one answer: %+v", len(collected.Results), collected)
	}
	res := collected.Results[0]
	if res.Status != StatusAnswered || res.Answer != "3 failures in foo_test" || res.Ticket != got.Ticket {
		t.Errorf("collected %+v, want the stashed answer under its own ticket", res)
	}
	if len(collected.Open) != 0 {
		t.Errorf("collect reports %v still open, want none", collected.Open)
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
	tests := []struct {
		name   string
		answer string
		want   int
	}{
		{"ascii is cut at the cap", strings.Repeat("a", 65537), 65536},
		// The byte cut lands inside the trailing rune; the half-rune is dropped
		// rather than typed into a PTY as garbage keystrokes.
		{"a rune is never cut in half", strings.Repeat("a", 65535) + "é", 65535},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := newRelay(workspace(), newFakeTerminal("s1", "s2"), nil)

			done := make(chan Result, 1)
			go func() {
				got, _ := svc.Send("s1", "docs", "", "hello", 30)
				done <- got
			}()
			ticketID := waitForTicket(svc)
			if err := svc.Reply(ticketID, tt.answer); err != nil {
				t.Fatalf("Reply: %v", err)
			}

			got := <-done
			if len(got.Answer) != tt.want {
				t.Errorf("answer is %d bytes, want %d", len(got.Answer), tt.want)
			}
			if !utf8.ValidString(got.Answer) {
				t.Error("the truncated answer is not valid UTF-8")
			}
		})
	}
}

// An ended session's row in the state map would never be read again — absent
// and idle mean the same "not working" — so it is dropped rather than kept for
// the life of the process.
func TestAnEndedSessionLeavesNoStateBehind(t *testing.T) {
	svc := newRelay(workspace(), newFakeTerminal("s1", "s2"), nil)

	svc.Observe("s2", "busy")
	svc.Observe("s2", "idle")

	svc.mu.Lock()
	_, kept := svc.state["s2"]
	svc.mu.Unlock()
	if kept {
		t.Error("a session that ended still holds a row in the state map")
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

// The numbers are spelled out rather than read from DefaultWait and MaxWait:
// derived from the constants the test says only that waitFor returns them, and
// stays green when they move. What the contract promises is these values — 100s
// under the 120s an agent's shell tool allows, and a 30 minute ceiling — so the
// cap is pinned from both sides of the boundary rather than from far away.
func TestWaitForClampsTheRequestedWait(t *testing.T) {
	tests := []struct {
		name    string
		seconds int
		want    time.Duration
	}{
		{"zero falls back to the default", 0, 100 * time.Second},
		{"negative falls back to the default", -5, 100 * time.Second},
		{"a plain wait is honoured", 42, 42 * time.Second},
		{"a wait just under the cap is honoured", 1799, 1799 * time.Second},
		{"a wait past the cap is capped", 1801, 30 * time.Minute},
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

// A session running its worktree setup script is live and has nothing that can
// read a message: the script owns the PTY until it execs the provider. Writing
// into it loses the request outright — `pnpm install` reads the bytes and drops
// them — and the sender then waits out a ticket nobody was ever asked to
// answer, which is how this was found.

func TestSendWaitsForTheSetupScriptToFinish(t *testing.T) {
	term := newFakeTerminal("s1", "s2")
	term.setUp("s2", true)
	svc := newRelay(workspace(), term, nil)

	go func() {
		time.Sleep(20 * time.Millisecond)
		term.setUp("s2", false)
	}()

	// One second covers both waits here: the setup finishes in 20ms, and nobody
	// answers, so the errand ends pending on purpose.
	result, err := svc.Send("s1", "docs", "", "run the tests", 1)
	if err != nil {
		t.Fatalf("Send = %v, want the message held until the agent was up", err)
	}
	if result.Status != StatusPending {
		t.Errorf("status = %q, want the errand open", result.Status)
	}
	if len(term.writesTo("s2")) == 0 {
		t.Error("nothing was typed at the target once its agent started")
	}
}

func TestSendRefusesRatherThanTypingIntoASetupScript(t *testing.T) {
	term := newFakeTerminal("s1", "s2")
	term.setUp("s2", true)
	svc := newRelay(workspace(), term, nil)

	_, err := svc.Send("s1", "docs", "", "run the tests", 1)
	if err == nil {
		t.Fatal("sent a message into a checkout that was still installing")
	}
	// The sender has to know nothing was delivered: a ticket here would be an
	// errand nobody can answer, waited out in full.
	if !strings.Contains(err.Error(), "Nothing was sent") {
		t.Errorf("error = %q, want it to say the message was not delivered", err)
	}
	if len(term.writesTo("s2")) != 0 {
		t.Errorf("typed %v into the setup script", term.writesTo("s2"))
	}
	svc.mu.Lock()
	open := len(svc.tickets)
	svc.mu.Unlock()
	if open != 0 {
		t.Errorf("left %d tickets open for a message that was never delivered", open)
	}
}

// TestSendSpendsOneBudgetAcrossSetupAndAnswer pins the deadline being shared:
// a setup that eats most of the wait leaves only the remainder for the answer,
// instead of a fresh full wait on top. Two full budgets would block past the
// HTTP client's own timeout (internal/cli, waitBudget) and report a failure on
// a message that was in fact delivered.
func TestSendSpendsOneBudgetAcrossSetupAndAnswer(t *testing.T) {
	term := newFakeTerminal("s1", "s2")
	term.setUp("s2", true)
	svc := newRelay(workspace(), term, nil)

	go func() {
		time.Sleep(750 * time.Millisecond)
		term.setUp("s2", false)
	}()

	start := time.Now()
	result, err := svc.Send("s1", "docs", "", "run the tests", 1)
	elapsed := time.Since(start)
	if err != nil {
		t.Fatalf("Send = %v, want the message delivered", err)
	}
	if result.Status != StatusPending {
		t.Errorf("status = %q, want the errand still open", result.Status)
	}
	// The old shape waited the setup out and then a full second more, ending
	// past 1.75s. The shared budget ends the call at the one second asked for.
	if elapsed > 1500*time.Millisecond {
		t.Errorf("Send blocked %v on a 1s wait — the setup and the answer each spent a full budget", elapsed)
	}
}

func TestSendGivesUpWhenTheTargetDiesDuringSetup(t *testing.T) {
	term := newFakeTerminal("s1", "s2")
	term.setUp("s2", true)
	svc := newRelay(workspace(), term, nil)

	go func() {
		time.Sleep(20 * time.Millisecond)
		term.mu.Lock()
		term.live["s2"] = false
		term.mu.Unlock()
	}()

	_, err := svc.Send("s1", "docs", "", "run the tests", 30)
	if err == nil {
		t.Fatal("waited on a session that is gone")
	}
	if !strings.Contains(err.Error(), "stopped before its agent started") {
		t.Errorf("error = %q", err)
	}
}

// Delivery was always provable — the bytes reached the PTY — and receipt never
// was. Between them sits everything that can be on a terminal instead of a
// prompt: a provider still starting, a dialog left open, Claude Code asking
// whether a new directory is trusted. The task is typed into that and is gone,
// and nothing fails: the sender waits out a ticket nobody was asked to answer.

// withReceipts is a relay that checks receipts, on a window a test can outlast.
func withReceipts(term Terminal) *Service {
	svc := newRelay(workspace(), term, nil)
	svc.receiptWindow = 40 * time.Millisecond
	svc.SetPlugins(fakePlugins{installed: true})
	return svc
}

// fakePlugins answers for the companion plugin: whether a provider's sessions
// report at all, and whether they carry lich's own operations.
type fakePlugins struct {
	installed bool
	tools     bool
}

func (f fakePlugins) Installed(string) bool { return f.installed }

func (f fakePlugins) HasTools(string) bool { return f.tools }

func TestATaskNobodyPicksUpComesBackUnread(t *testing.T) {
	term := newFakeTerminal("s1", "s2")
	svc := withReceipts(term)

	result, err := svc.Send("s1", "docs", "", "run the tests", 5)
	if err != nil {
		t.Fatalf("Send = %v, want nil", err)
	}
	if result.Status != StatusUnread {
		t.Errorf("status = %q, want the sender told nothing read it", result.Status)
	}
	svc.mu.Lock()
	open := len(svc.tickets)
	svc.mu.Unlock()
	if open != 0 {
		t.Errorf("left %d tickets open on a task that was never read", open)
	}
}

func TestATaskTheTargetStartsWorkingOnIsNotUnread(t *testing.T) {
	term := newFakeTerminal("s1", "s2")
	svc := withReceipts(term)

	go func() {
		time.Sleep(10 * time.Millisecond)
		svc.Observe("s2", stateBusy)
	}()

	result, err := svc.Send("s1", "docs", "", "run the tests", 1)
	if err != nil {
		t.Fatalf("Send = %v, want nil", err)
	}
	if result.Status != StatusPending {
		t.Errorf("status = %q, want the errand still open", result.Status)
	}
}

// A target that was already working is not checked at all: its provider queues
// the message for the next turn and is busy the whole time, so there is nothing
// here to tell apart.
func TestABusyTargetIsNotCheckedForReceipt(t *testing.T) {
	term := newFakeTerminal("s1", "s2")
	svc := withReceipts(term)
	svc.Observe("s2", stateBusy)

	result, err := svc.Send("s1", "docs", "", "run the tests", 1)
	if err != nil {
		t.Fatalf("Send = %v, want nil", err)
	}
	if result.Status == StatusUnread {
		t.Error("a target that was mid-turn was reported as never having read the task")
	}
}

// A target sitting on a mid-turn permission prompt reports waiting, and its
// silence afterwards is a human not answering — not a task nobody read. The
// message queues behind the open turn, so calling it unread would drop a ticket
// whose answer is still coming.
func TestAWaitingTargetIsTreatedAsMidTurn(t *testing.T) {
	term := newFakeTerminal("s1", "s2")
	svc := withReceipts(term)
	svc.Observe("s2", stateBusy)
	svc.Observe("s2", stateWaiting)

	result, err := svc.Send("s1", "docs", "", "run the tests", 1)
	if err != nil {
		t.Fatalf("Send = %v, want nil", err)
	}
	if result.Status == StatusUnread {
		t.Error("a target blocked on its own permission prompt was reported as never reading the task")
	}
	if result.Status != StatusPending {
		t.Errorf("status = %q, want the errand still open", result.Status)
	}
}

// waiting after done is the provider nudging its user, not an open turn: the
// target is at its prompt, so a task typed there is read within seconds and the
// receipt check has to stay armed.
func TestWaitingAfterDoneStillArmsTheReceiptCheck(t *testing.T) {
	term := newFakeTerminal("s1", "s2")
	svc := withReceipts(term)
	svc.Observe("s2", stateBusy)
	svc.Observe("s2", stateDone)
	svc.Observe("s2", stateWaiting)

	result, err := svc.Send("s1", "docs", "", "run the tests", 5)
	if err != nil {
		t.Fatalf("Send = %v, want nil", err)
	}
	if result.Status != StatusUnread {
		t.Errorf("status = %q, want the silence read as unread", result.Status)
	}
}

// Without the plugin a session reports nothing whatever happens in it, so its
// silence says nothing either.
func TestSilenceIsOnlyReadWhereTheProviderReports(t *testing.T) {
	term := newFakeTerminal("s1", "s2")
	svc := newRelay(workspace(), term, nil)
	svc.receiptWindow = 40 * time.Millisecond
	svc.SetPlugins(fakePlugins{installed: false})

	result, err := svc.Send("s1", "docs", "", "run the tests", 1)
	if err != nil {
		t.Fatalf("Send = %v, want nil", err)
	}
	if result.Status == StatusUnread {
		t.Error("read silence as an answer from a provider that never reports")
	}
}

// The sender usually stops waiting long before this: it took a ticket and
// moved on, so the news has to reach its prompt — as a nudge, with the unread
// outcome itself waiting in the inbox.
func TestTheSenderIsToldAtItsPromptWhenNobodyWasWaiting(t *testing.T) {
	term := newFakeTerminal("s1", "s2")
	svc := withReceipts(term)
	// A window the sender's own wait runs out before, which is the ordinary
	// case: an agent holds a tool call for a fraction of what an errand takes.
	svc.receiptWindow = 1200 * time.Millisecond

	result, err := svc.Send("s1", "docs", "", "run the tests", 1)
	if err != nil {
		t.Fatalf("Send = %v, want nil", err)
	}
	if result.Status != StatusPending {
		t.Fatalf("status = %q, want the sender to have given up first", result.Status)
	}
	if !awaitWritten(term, "s1", "[lich]") {
		t.Fatalf("the sender was never nudged: %v", term.writesTo("s1"))
	}
	collected, err := svc.Collect("s1", 1)
	if err != nil {
		t.Fatalf("Collect: %v", err)
	}
	if len(collected.Results) != 1 || collected.Results[0].Status != StatusUnread {
		t.Errorf("collected %+v, want the unread outcome", collected)
	}
}

// The two can also land together: the window closes the ticket while the sender
// is still counted as waiting, so nobody types the news at its prompt, and the
// sender's own wait runs out a moment later. The waiter that leaves last owns
// what is left behind — without that it hears "still working" about a task
// nothing read, on a ticket already gone from the map.
func TestAWaiterGivingUpAsTheTaskGoesUnreadHearsSo(t *testing.T) {
	svc := withReceipts(newFakeTerminal("s1", "s2"))
	tk := &ticket{
		target:   "docs",
		done:     make(chan struct{}),
		stalled:  make(chan struct{}),
		unread:   make(chan struct{}),
		attended: 1,
	}
	close(tk.unread)

	if got := svc.giveUp("tk1", tk); got != StatusUnread {
		t.Errorf("status = %q, want the sender told nothing read it", got)
	}
}

// Which providers carry lich's operations stopped being a fact about the
// provider the day the plugin started carrying them: Claude Code and Codex are
// told about the server at spawn, opencode and Crush get it with the plugin, and
// only if the installed one is new enough. The relay asks rather than assumes.

func TestTheMessageNamesTheToolWhenTheTargetHasIt(t *testing.T) {
	term := newFakeTerminal("s1", "s2")
	svc := newRelay(workspace(), term, nil)
	svc.SetPlugins(fakePlugins{installed: true, tools: true})

	if _, err := svc.Send("s1", "docs", "", "run the tests", 1); err != nil {
		t.Fatalf("Send = %v", err)
	}
	typed := strings.Join(term.writesTo("s2"), "")
	if !strings.Contains(typed, ToolReply) {
		t.Errorf("a target holding the tool was told to shell out:\n%s", typed)
	}
}

func TestTheMessageNamesOnlyTheCommandWithoutTheTool(t *testing.T) {
	term := newFakeTerminal("s1", "s2")
	svc := newRelay(workspace(), term, nil)
	svc.SetPlugins(fakePlugins{installed: true, tools: false})

	if _, err := svc.Send("s1", "docs", "", "run the tests", 1); err != nil {
		t.Fatalf("Send = %v", err)
	}
	typed := strings.Join(term.writesTo("s2"), "")
	if strings.Contains(typed, ToolReply) {
		t.Errorf("named a tool the target does not have:\n%s", typed)
	}
	// The command is the route that needs nothing registered anywhere.
	if !strings.Contains(typed, `"$LICH_BIN" reply`) {
		t.Errorf("left the target with no way to answer:\n%s", typed)
	}
}

// Nothing wired is the state a test — and a lich whose plugin check never ran —
// is in. It has to read as "no tools", never as "assume they are there".
func TestWithoutAPluginCheckTheMessageNamesTheCommand(t *testing.T) {
	term := newFakeTerminal("s1", "s2")
	svc := newRelay(workspace(), term, nil)

	if _, err := svc.Send("s1", "docs", "", "run the tests", 1); err != nil {
		t.Fatalf("Send = %v", err)
	}
	if typed := strings.Join(term.writesTo("s2"), ""); strings.Contains(typed, ToolReply) {
		t.Errorf("assumed a tool with nothing to ask:\n%s", typed)
	}
}

// The inbox: a result nobody was holding the line for is stashed and announced
// with one short nudge, and the text itself comes back through Collect — never
// typed at the sender's prompt, where every arrival restarts a turn.

// plant registers one delivered errand directly, so inbox tests need not spend
// real seconds waiting out a Send's own timeout first.
func plant(svc *Service, id, fromID, targetID, target string) {
	svc.mu.Lock()
	svc.tickets[id] = &ticket{
		fromID: fromID, targetID: targetID, target: target,
		created:   time.Now(),
		delivered: time.Now(),
		done:      make(chan struct{}),
		stalled:   make(chan struct{}),
		unread:    make(chan struct{}),
	}
	svc.mu.Unlock()
}

// A fan-out's workers finish in bursts. One nudge per result would restart the
// sender's turn once per worker — the debounce folds the burst into one line
// naming everything that is waiting.
func TestABurstOfAnswersCoalescesIntoOneNudge(t *testing.T) {
	term := newFakeTerminal("s1", "s2", "s3")
	svc := newRelay(workspace(), term, nil)
	svc.nudgeDelay = 20 * time.Millisecond
	plant(svc, "t1", "s1", "s2", "docs")
	plant(svc, "t2", "s1", "s3", "api")

	if err := svc.Reply("t1", "done here"); err != nil {
		t.Fatalf("Reply t1: %v", err)
	}
	if err := svc.Reply("t2", "done there"); err != nil {
		t.Fatalf("Reply t2: %v", err)
	}

	if !awaitWritten(term, "s1", "[lich]") {
		t.Fatalf("no nudge arrived: %q", term.writesTo("s1"))
	}
	// Long enough for a second nudge to have fired if one were coming.
	time.Sleep(50 * time.Millisecond)
	writes := term.writesTo("s1")
	if len(writes) != 2 {
		t.Fatalf("got %d writes, want one nudge (paste and submit): %q", len(writes), writes)
	}
	typed := term.written("s1")
	for _, want := range []string{"2 tasks", `"docs"`, `"api"`} {
		if !strings.Contains(typed, want) {
			t.Errorf("the nudge is missing %q:\n%s", want, typed)
		}
	}
}

// A delivery mid-turn queues as its own turn, which is the cost this whole
// path exists to avoid — a busy sender hears nothing until its turn ends.
func TestABusySenderIsNudgedOnlyWhenItsTurnEnds(t *testing.T) {
	term := newFakeTerminal("s1", "s2")
	svc := newRelay(workspace(), term, nil)
	svc.Observe("s1", stateBusy)
	plant(svc, "t1", "s1", "s2", "docs")

	if err := svc.Reply("t1", "all green"); err != nil {
		t.Fatalf("Reply: %v", err)
	}
	time.Sleep(20 * time.Millisecond)
	if typed := term.written("s1"); typed != "" {
		t.Fatalf("nudged a sender mid-turn: %q", typed)
	}

	svc.Observe("s1", stateDone)
	if !awaitWritten(term, "s1", "[lich]") {
		t.Fatalf("the turn ended and no nudge came: %q", term.writesTo("s1"))
	}
}

// One nudge per result. A sender that read the nudge and chose not to collect
// must not be nudged again at every turn end — each nudge starts a turn, and
// two of them chasing each other is a loop with a token bill.
func TestANudgeIsNotRepeatedOnLaterTurnEnds(t *testing.T) {
	term := newFakeTerminal("s1", "s2")
	svc := newRelay(workspace(), term, nil)
	plant(svc, "t1", "s1", "s2", "docs")
	if err := svc.Reply("t1", "all green"); err != nil {
		t.Fatalf("Reply: %v", err)
	}
	if !awaitWritten(term, "s1", "[lich]") {
		t.Fatalf("no nudge arrived: %q", term.writesTo("s1"))
	}

	svc.Observe("s1", stateBusy)
	svc.Observe("s1", stateDone)
	time.Sleep(20 * time.Millisecond)
	if writes := term.writesTo("s1"); len(writes) != 2 {
		t.Errorf("an uncollected result was nudged again: %q", writes)
	}
}

func TestCollectHoldsTheLineForTheNextResult(t *testing.T) {
	term := newFakeTerminal("s1", "s2")
	svc := newRelay(workspace(), term, nil)
	plant(svc, "t1", "s1", "s2", "docs")

	go func() {
		time.Sleep(20 * time.Millisecond)
		_ = svc.Reply("t1", "all green")
	}()
	collected, err := svc.Collect("s1", 2)
	if err != nil {
		t.Fatalf("Collect: %v", err)
	}
	if len(collected.Results) != 1 || collected.Results[0].Answer != "all green" {
		t.Fatalf("collected %+v, want the answer", collected)
	}
	// The collector carried the result out itself; a nudge on top would deliver
	// the news twice.
	time.Sleep(20 * time.Millisecond)
	if typed := term.written("s1"); typed != "" {
		t.Errorf("a result a collector carried out was also nudged: %q", typed)
	}
}

func TestCollectReportsWhoStillOwes(t *testing.T) {
	term := newFakeTerminal("s1", "s2", "s3")
	svc := newRelay(workspace(), term, nil)
	plant(svc, "t1", "s1", "s2", "docs")
	plant(svc, "t2", "s1", "s3", "api")
	if err := svc.Reply("t1", "all green"); err != nil {
		t.Fatalf("Reply: %v", err)
	}

	collected, err := svc.Collect("s1", 1)
	if err != nil {
		t.Fatalf("Collect: %v", err)
	}
	if len(collected.Results) != 1 || collected.Results[0].Target != "docs" {
		t.Fatalf("collected %+v, want the one finished errand", collected)
	}
	if len(collected.Open) != 1 || collected.Open[0] != "api" {
		t.Errorf("open = %v, want the session still owing", collected.Open)
	}
}

func TestCollectWithNothingOpenReturnsAtOnce(t *testing.T) {
	svc := newRelay(workspace(), newFakeTerminal("s1"), nil)

	start := time.Now()
	collected, err := svc.Collect("s1", 30)
	if err != nil {
		t.Fatalf("Collect: %v", err)
	}
	if len(collected.Results) != 0 || len(collected.Open) != 0 {
		t.Errorf("collected %+v from a caller with no errands", collected)
	}
	if time.Since(start) > time.Second {
		t.Error("Collect held the line with no errand to wait for")
	}
}

func TestCollectRefusesACallerWithNoSession(t *testing.T) {
	svc := newRelay(workspace(), newFakeTerminal(), nil)

	if _, err := svc.Collect("", 1); err == nil {
		t.Fatal("collected for a caller that has no prompt to be nudged at")
	}
}

func TestWaitPicksAStashedOutcomeUp(t *testing.T) {
	term := newFakeTerminal("s1", "s2")
	svc := newRelay(workspace(), term, nil)
	plant(svc, "t1", "s1", "s2", "docs")
	if err := svc.Reply("t1", "all green"); err != nil {
		t.Fatalf("Reply: %v", err)
	}

	res, err := svc.Wait("t1", 1)
	if err != nil {
		t.Fatalf("Wait: %v", err)
	}
	if res.Status != StatusAnswered || res.Answer != "all green" {
		t.Fatalf("result = %+v, want the stashed answer", res)
	}
	if _, err := svc.Wait("t1", 1); err == nil {
		t.Error("the same outcome was handed out twice")
	}
}

func TestAnUncollectedResultExpiresWithItsTTL(t *testing.T) {
	svc := newRelay(workspace(), newFakeTerminal("s1"), nil)
	svc.mu.Lock()
	svc.ready["old"] = &inboxEntry{
		ticket: "old", fromID: "s1", target: "docs",
		status: StatusAnswered, answer: "stale",
		ready: time.Now().Add(-2 * time.Hour),
	}
	svc.mu.Unlock()

	collected, err := svc.Collect("s1", 1)
	if err != nil {
		t.Fatalf("Collect: %v", err)
	}
	if len(collected.Results) != 0 {
		t.Errorf("an hour-old result was still handed over: %+v", collected.Results)
	}
}

// The window hears the inbox change size, so the card can say results are
// waiting: the nudge tells the agent, the event tells the person.
func TestTheInboxSizeIsAnnouncedAsItGrowsAndDrains(t *testing.T) {
	events := &fakeEvents{}
	term := newFakeTerminal("s1", "s2", "s3")
	svc := newRelay(workspace(), term, events)
	plant(svc, "t1", "s1", "s2", "docs")
	plant(svc, "t2", "s1", "s3", "api")

	if err := svc.Reply("t1", "one"); err != nil {
		t.Fatalf("Reply t1: %v", err)
	}
	if err := svc.Reply("t2", "two"); err != nil {
		t.Fatalf("Reply t2: %v", err)
	}
	if got := events.inboxCounts("s1"); len(got) != 2 || got[0] != 1 || got[1] != 2 {
		t.Fatalf("counts = %v, want [1 2]", got)
	}

	if _, err := svc.Collect("s1", 1); err != nil {
		t.Fatalf("Collect: %v", err)
	}
	counts := events.inboxCounts("s1")
	if len(counts) != 3 || counts[2] != 0 {
		t.Errorf("counts = %v, want the drain announced as 0", counts)
	}
}

func TestASingleTicketPickupAnnouncesTheSmallerInbox(t *testing.T) {
	events := &fakeEvents{}
	svc := newRelay(workspace(), newFakeTerminal("s1", "s2"), events)
	plant(svc, "t1", "s1", "s2", "docs")
	if err := svc.Reply("t1", "one"); err != nil {
		t.Fatalf("Reply: %v", err)
	}

	if _, err := svc.Wait("t1", 1); err != nil {
		t.Fatalf("Wait: %v", err)
	}
	counts := events.inboxCounts("s1")
	if len(counts) != 2 || counts[1] != 0 {
		t.Errorf("counts = %v, want the pickup announced as 0", counts)
	}
}

func TestAnExpiredInboxEntryAnnouncesItsAbsence(t *testing.T) {
	events := &fakeEvents{}
	svc := newRelay(workspace(), newFakeTerminal("s1"), events)
	svc.mu.Lock()
	svc.ready["old"] = &inboxEntry{
		ticket: "old", fromID: "s1", target: "docs",
		status: StatusAnswered, ready: time.Now().Add(-2 * time.Hour),
	}
	svc.mu.Unlock()

	if _, err := svc.Collect("s1", 1); err != nil {
		t.Fatalf("Collect: %v", err)
	}
	counts := events.inboxCounts("s1")
	if len(counts) != 1 || counts[0] != 0 {
		t.Errorf("counts = %v, want the expiry announced as 0", counts)
	}
}

func TestTheNudgeNamesTheToolOnlyWhereItExists(t *testing.T) {
	tests := []struct {
		name  string
		tools bool
	}{
		{"a sender with lich's tools", true},
		{"a sender with none", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			term := newFakeTerminal("s1", "s2")
			svc := newRelay(workspace(), term, nil)
			svc.SetPlugins(fakePlugins{installed: true, tools: tt.tools})
			plant(svc, "t1", "s1", "s2", "docs")
			if err := svc.Reply("t1", "all green"); err != nil {
				t.Fatalf("Reply: %v", err)
			}

			if !awaitWritten(term, "s1", "[lich]") {
				t.Fatalf("no nudge arrived: %q", term.writesTo("s1"))
			}
			typed := term.written("s1")
			if named := strings.Contains(typed, ToolCollect); named != tt.tools {
				t.Errorf("names %s = %v, want %v:\n%s", ToolCollect, named, tt.tools, typed)
			}
			if !strings.Contains(typed, `"$LICH_BIN" wait`) {
				t.Errorf("the command route is missing:\n%s", typed)
			}
		})
	}
}
