package relay

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

// fakeMod stands in for a session's Claude Code mod. It answers each prompt
// with the next scripted receipt; once the script runs out the mod is gone,
// and the session reads as having none. An acked or ended receipt arrives at
// once, the way an ack or a session exit does; withdrawn and delivered only
// once the wait on them ends, which is the only moment they are known.
type fakeMod struct {
	receipts []PromptReceipt
	err      error
	prompts  []string
}

func (f *fakeTerminal) SubmitPrompt(id, text string) (func(context.Context) PromptReceipt, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	mod := f.mods[id]
	if mod == nil {
		return nil, ErrNoMod
	}
	if mod.err != nil {
		return nil, mod.err
	}
	if len(mod.prompts) >= len(mod.receipts) {
		return nil, ErrNoMod
	}
	receipt := mod.receipts[len(mod.prompts)]
	mod.prompts = append(mod.prompts, text)
	return func(ctx context.Context) PromptReceipt {
		if receipt.State != PromptAcked && receipt.State != PromptEnded {
			<-ctx.Done()
		}
		return receipt
	}, nil
}

// attachMod gives a session a mod that answers its prompts with receipts, in
// order.
func (f *fakeTerminal) attachMod(id string, receipts ...PromptReceipt) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.mods[id] = &fakeMod{receipts: receipts}
}

// failMod makes handing a prompt to that session's mod fail with err.
func (f *fakeTerminal) failMod(id string, err error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.mods[id] = &fakeMod{err: err}
}

// promptsTo is every prompt a session's mod was handed, in order.
func (f *fakeTerminal) promptsTo(id string) []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	if mod := f.mods[id]; mod != nil {
		return append([]string(nil), mod.prompts...)
	}
	return nil
}

// awaitPrompts blocks until a session's mod was handed n prompts, and returns
// whether it was.
func awaitPrompts(term *fakeTerminal, id string, n int) bool {
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if len(term.promptsTo(id)) >= n {
			return true
		}
		time.Sleep(time.Millisecond)
	}
	return false
}

var (
	ackedOK   = PromptReceipt{State: PromptAcked, OK: true}
	withdrawn = PromptReceipt{State: PromptWithdrawn}
	collected = PromptReceipt{State: PromptDelivered}
)

// viaMod is a relay whose sessions report state, with a receipt window short
// enough to run out inside a test.
func viaMod(term *fakeTerminal, events Events) *Service {
	svc := newRelay(workspace(), term, events)
	svc.receiptWindow = 40 * time.Millisecond
	svc.modAckWait = 40 * time.Millisecond
	svc.SetPlugins(fakePlugins{installed: true})
	return svc
}

func assertNotTyped(t *testing.T, term *fakeTerminal, id string) {
	t.Helper()
	if writes := term.writesTo(id); len(writes) != 0 {
		t.Errorf("%s was typed at: %q", id, writes)
	}
}

func TestATaskToASessionWithAModIsHandedToItNotTyped(t *testing.T) {
	term := newFakeTerminal("s1", "s2")
	term.attachMod("s2", ackedOK)
	events := &fakeEvents{}
	svc := viaMod(term, events)

	done := make(chan Result, 1)
	go func() {
		got, _ := svc.Send(context.Background(), "s1", "docs", "", "run the tests", 5)
		done <- got
	}()
	if !events.awaitMark("s2", DirectionIn) || !events.awaitMark("s1", DirectionOut) {
		t.Fatal("the cards were never marked")
	}
	ticketID := waitForTicket(svc)
	if err := svc.Reply("", ticketID, "all green"); err != nil {
		t.Fatalf("Reply: %v", err)
	}
	if got := <-done; got.Status != StatusAnswered || got.Answer != "all green" {
		t.Fatalf("result = %+v, want answered", got)
	}
	prompts := term.promptsTo("s2")
	if len(prompts) != 1 || !strings.Contains(prompts[0], "run the tests") || !strings.Contains(prompts[0], ticketID) {
		t.Errorf("the mod was handed %q, want the composed task once", prompts)
	}
	assertNotTyped(t, term, "s2")
}

// A prompt no poll collected never runs, so typing it keeps delivery at most
// once; from there it is a typed task like any other, receipt check included.
func TestATaskNoPollCollectedIsTypedInstead(t *testing.T) {
	term := newFakeTerminal("s1", "s2")
	term.attachMod("s2", withdrawn)
	svc := viaMod(term, nil)

	got, err := svc.Send(context.Background(), "s1", "docs", "", "run the tests", 5)
	if err != nil {
		t.Fatalf("Send: %v", err)
	}
	if got.Status != StatusUnread {
		t.Errorf("status = %q, want unread: the typed task was never picked up", got.Status)
	}
	if writes := term.writesTo("s2"); len(writes) != 4 || writes[1] != submit {
		t.Errorf("writes = %q, want the task typed, then typed again by the receipt check", writes)
	}
	if prompts := term.promptsTo("s2"); len(prompts) != 1 {
		t.Errorf("the mod was handed %d prompts, want 1", len(prompts))
	}
}

// A prompt a poll collected may still run: typing it again could run the task
// twice, so it is reported unread instead, and a late answer still lands.
func TestATaskTheModCollectedIsNeverTypedAgain(t *testing.T) {
	term := newFakeTerminal("s1", "s2")
	term.attachMod("s2", collected)
	svc := viaMod(term, nil)

	got, err := svc.Send(context.Background(), "s1", "docs", "", "run the tests", 5)
	if err != nil {
		t.Fatalf("Send: %v", err)
	}
	if got.Status != StatusUnread {
		t.Fatalf("status = %q, want unread", got.Status)
	}
	assertNotTyped(t, term, "s2")
	if err := svc.Reply("", got.Ticket, "done after all"); err != nil {
		t.Fatalf("a late answer = %v, want it filed", err)
	}
	if again, _ := svc.Wait(context.Background(), got.Ticket, 1); again.Status != StatusAnswered {
		t.Errorf("Wait = %+v, want the late answer", again)
	}
}

// A busy target runs the prompt once its turn ends, which can be long after
// the window: the errand waits for its turn instead of being written off.
func TestATaskTheModCollectedForABusyTargetWaitsForItsTurn(t *testing.T) {
	term := newFakeTerminal("s1", "s2")
	term.attachMod("s2", collected)
	svc := viaMod(term, nil)
	svc.Observe("s2", stateBusy)

	done := make(chan Result, 1)
	go func() {
		got, _ := svc.Send(context.Background(), "s1", "docs", "", "run the tests", 5)
		done <- got
	}()
	awaitDelivered(t, svc, 1)
	time.Sleep(3 * svc.receiptWindow)
	if open := openTickets(svc); len(open) != 1 {
		t.Fatalf("open tickets = %v, want the errand still waiting for its turn", open)
	}

	svc.Observe("s2", stateDone)
	svc.Observe("s2", stateBusy)
	svc.Observe("s2", stateDone)
	if got := <-done; got.Status != StatusUnanswered {
		t.Errorf("status = %q, want unanswered: its own turn ended without a reply", got.Status)
	}
	assertNotTyped(t, term, "s2")
}

// A UserPromptSubmit hook that blocks the prompt would block it typed as well,
// so it is reported undelivered rather than typed.
func TestATaskAHookDroppedComesBackUndelivered(t *testing.T) {
	term := newFakeTerminal("s1", "s2")
	term.attachMod("s2", PromptReceipt{State: PromptAcked, Reason: "blocked by a hook"})
	svc := viaMod(term, nil)

	got, err := svc.Send(context.Background(), "s1", "docs", "", "run the tests", 5)
	if err != nil {
		t.Fatalf("Send: %v", err)
	}
	if got.Status != StatusUndelivered {
		t.Errorf("status = %q, want undelivered", got.Status)
	}
	if open := openTickets(svc); len(open) != 0 {
		t.Errorf("open tickets = %v, want none", open)
	}
	assertNotTyped(t, term, "s2")
}

func TestAModTaskTheTargetStartedIsNotUnread(t *testing.T) {
	term := newFakeTerminal("s1", "s2")
	term.attachMod("s2", ackedOK)
	svc := viaMod(term, nil)
	go func() {
		time.Sleep(10 * time.Millisecond)
		svc.Observe("s2", stateBusy)
	}()

	got, err := svc.Send(context.Background(), "s1", "docs", "", "run the tests", 1)
	if err != nil {
		t.Fatalf("Send: %v", err)
	}
	if got.Status != StatusPending {
		t.Errorf("status = %q, want the errand still open", got.Status)
	}
	assertNotTyped(t, term, "s2")
}

func TestAnEndedSessionsModTaskIsNotTyped(t *testing.T) {
	term := newFakeTerminal("s1", "s2")
	term.attachMod("s2", PromptReceipt{State: PromptEnded})
	svc := viaMod(term, nil)

	got, err := svc.Send(context.Background(), "s1", "docs", "", "run the tests", 5)
	if err != nil {
		t.Fatalf("Send: %v", err)
	}
	if got.Status != StatusUnread {
		t.Errorf("status = %q, want unread", got.Status)
	}
	assertNotTyped(t, term, "s2")
}

// A fresh session is typed at while its mod has not polled yet; the receipt
// check's second attempt goes to the mod once it has.
func TestARetryReachesTheModOnceItAttaches(t *testing.T) {
	term := newFakeTerminal("s1", "s2")
	svc := viaMod(term, nil)
	term.onWrite = func(id, data string) {
		if id == "s2" && data == submit {
			term.attachMod("s2", ackedOK)
		}
	}

	got, err := svc.Send(context.Background(), "s1", "docs", "", "run the tests", 1)
	if err != nil {
		t.Fatalf("Send: %v", err)
	}
	if got.Status != StatusPending {
		t.Errorf("status = %q, want the errand open: the retry reached the mod", got.Status)
	}
	if writes := term.writesTo("s2"); len(writes) != 2 {
		t.Errorf("writes = %q, want the first attempt typed and the retry not", writes)
	}
	if prompts := term.promptsTo("s2"); len(prompts) != 1 {
		t.Errorf("the mod was handed %d prompts, want the retry", len(prompts))
	}
}

func TestSubmitPromptErrorIsTheSendsError(t *testing.T) {
	term := newFakeTerminal("s1", "s2")
	refused := errors.New("the transport is down")
	term.failMod("s2", refused)
	svc := viaMod(term, nil)

	_, err := svc.Send(context.Background(), "s1", "docs", "", "run the tests", 5)
	if !errors.Is(err, refused) {
		t.Fatalf("Send = %v, want the mod's error", err)
	}
	if open := openTickets(svc); len(open) != 0 {
		t.Errorf("open tickets = %v, want none", open)
	}
	assertNotTyped(t, term, "s2")
}

// The mod never touches the draft, so a sender typing at its prompt is told at
// once instead of after its line goes stale.
func TestANudgeReachesASenderThroughItsMod(t *testing.T) {
	term := newFakeTerminal("s1", "s2")
	term.attachMod("s1", ackedOK)
	term.typeAt("s1", true)
	svc := viaMod(term, nil)
	plant(svc, "t1", "s1", "s2", "docs")

	if err := svc.Reply("", "t1", "all green"); err != nil {
		t.Fatalf("Reply: %v", err)
	}
	if !awaitPrompts(term, "s1", 1) {
		t.Fatal("the sender's mod was never handed the nudge")
	}
	if prompt := term.promptsTo("s1")[0]; !strings.Contains(prompt, "[lich]") {
		t.Errorf("nudge = %q, want the [lich] note", prompt)
	}
	assertNotTyped(t, term, "s1")
}

// Mirrors TestANudgeThatNeverLandedIsSentAgain for a nudge a hook refused.
func TestANudgeTheModRefusedIsSentAgain(t *testing.T) {
	term := newFakeTerminal("s1", "s2")
	term.attachMod("s1", PromptReceipt{State: PromptAcked, Reason: "blocked"}, ackedOK)
	svc := viaMod(term, nil)
	plant(svc, "t1", "s1", "s2", "docs")

	if err := svc.Reply("", "t1", "all green"); err != nil {
		t.Fatalf("Reply: %v", err)
	}
	if !awaitPrompts(term, "s1", 1) {
		t.Fatal("the nudge was never attempted")
	}
	svc.Observe("s1", stateDone)
	if !awaitPrompts(term, "s1", 2) {
		t.Fatal("a nudge the mod refused was never sent again")
	}
	assertNotTyped(t, term, "s1")
}

func TestANudgeNoPollCollectedIsTyped(t *testing.T) {
	term := newFakeTerminal("s1", "s2")
	term.attachMod("s1", withdrawn)
	svc := viaMod(term, nil)
	plant(svc, "t1", "s1", "s2", "docs")

	if err := svc.Reply("", "t1", "all green"); err != nil {
		t.Fatalf("Reply: %v", err)
	}
	if !awaitWritten(term, "s1", "[lich]") {
		t.Fatalf("a nudge no poll collected was never typed: %q", term.writesTo("s1"))
	}
}

func TestAScheduledPromptGoesThroughTheMod(t *testing.T) {
	writer := &scheduleWriter{}
	term := newFakeTerminal("s1")
	term.attachMod("s1", ackedOK)
	svc := newRelay(scheduled(1000, "run the release checklist", writer), term, nil)
	svc.now = at(1000)

	svc.deliverDue()

	if prompts := term.promptsTo("s1"); len(prompts) != 1 || !strings.Contains(prompts[0], "run the release checklist") {
		t.Fatalf("the mod was handed %q, want the scheduled prompt", prompts)
	}
	assertNotTyped(t, term, "s1")
}

func TestTheTicketNoticeGoesThroughTheMod(t *testing.T) {
	term := newFakeTerminal("s1", "s2", "s3")
	term.attachMod("s2", ackedOK, ackedOK, ackedOK)
	svc := viaMod(term, nil)

	for _, from := range []string{"s1", "s3"} {
		go func() { _, _ = svc.Send(context.Background(), from, "docs", "", "task from "+from, 5) }()
	}
	awaitDelivered(t, svc, 2)
	svc.Observe("s2", stateBusy)
	svc.Observe("s2", stateDone)

	if !awaitPrompts(term, "s2", 3) {
		t.Fatalf("the worker's mod was never asked to name the ticket: %q", term.promptsTo("s2"))
	}
	if notice := term.promptsTo("s2")[2]; !strings.Contains(notice, "has to name its ticket") {
		t.Errorf("third prompt = %q, want the notice", notice)
	}
	assertNotTyped(t, term, "s2")
}
