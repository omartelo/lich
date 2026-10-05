package relay

import (
	"context"
	"strings"
	"testing"
	"time"
)

// plantSubagent is plant for an errand handed over with SendSubagent, already
// submitted at the worker's prompt.
func plantSubagent(svc *Service, id, fromID, targetID, target string) {
	plant(svc, id, fromID, targetID, target)
	svc.mu.Lock()
	svc.tickets[id].subagent = true
	svc.tickets[id].submitted = true
	svc.mu.Unlock()
}

// branched is workspace with the worker s2 on a branch of its own.
func branched() fakeSessions {
	sessions := workspace()
	sessions.branches = map[string]string{"s2": "feat/docs"}
	return sessions
}

func TestSendSubagentMarksItsErrand(t *testing.T) {
	svc := newRelay(workspace(), newFakeTerminal("s1", "s2"), nil)

	got, err := svc.SendSubagent(context.Background(), "s1", "docs", "", "write the docs", 1)
	if err != nil {
		t.Fatalf("SendSubagent: %v", err)
	}
	svc.mu.Lock()
	defer svc.mu.Unlock()
	if tk := svc.tickets[got.Ticket]; tk == nil || !tk.subagent || tk.private {
		t.Errorf("ticket = %+v, want an open subagent errand", tk)
	}
}

func TestSendSubagentNeedsACallingSession(t *testing.T) {
	svc := newRelay(workspace(), newFakeTerminal("s2"), nil)

	_, err := svc.SendSubagent(context.Background(), "", "docs", "", "write the docs", 1)
	if err == nil || !strings.Contains(err.Error(), "session") {
		t.Fatalf("err = %v, want a refusal naming the missing session", err)
	}
}

// Through the caller's mod the whole report goes, so the model reads it the
// way it reads a native subagent's result, with no collect call.
func TestASubagentReportReachesItsCallerWholeThroughTheMod(t *testing.T) {
	term := newFakeTerminal("s1", "s2")
	term.attachMod("s1", ackedOK)
	svc := viaMod(term, nil)
	svc.sessions = branched()
	plantSubagent(svc, "t1", "s1", "s2", "docs")

	report := "Rewrote docs/cli.md; " + strings.Repeat("detail ", 400) + "nothing remains."
	if err := svc.Reply("", "t1", report); err != nil {
		t.Fatalf("Reply: %v", err)
	}
	if !awaitPrompts(term, "s1", 1) {
		t.Fatal("the caller's mod was never handed the report")
	}
	note := term.promptsTo("s1")[0]
	for _, want := range []string{report, `"docs"`, "feat/docs", "t1"} {
		if !strings.Contains(note, want) {
			t.Errorf("note is missing %.60q:\n%.300s", want, note)
		}
	}
	assertNotTyped(t, term, "s1")

	// Delivered once: a collect does not hand it over again, and the card's
	// count drops with it.
	collected, err := svc.CollectNow("s1")
	if err != nil || len(collected.Results) != 0 {
		t.Errorf("collect = %+v, %v; want nothing left to collect", collected, err)
	}
	// Its ticket still names it, for a caller that asks by number.
	got, err := svc.Wait(context.Background(), "t1", 1)
	if err != nil || got.Status != StatusAnswered || got.Answer != report || got.Private {
		t.Errorf("wait = %+v, %v; want the report under its ticket", got, err)
	}
}

// With nothing but a prompt to type at, the report is not typed: a long text
// typed into a TUI is what the mod route exists to avoid.
func TestASubagentReportIsNotTypedAtACallerWithoutAMod(t *testing.T) {
	term := newFakeTerminal("s1", "s2")
	svc := newRelay(branched(), term, nil)
	plantSubagent(svc, "t1", "s1", "s2", "docs")

	if err := svc.Reply("", "t1", "the long report"); err != nil {
		t.Fatalf("Reply: %v", err)
	}
	if !awaitWritten(term, "s1", "[lich]") {
		t.Fatal("the caller was never told")
	}
	if strings.Contains(term.written("s1"), "the long report") {
		t.Errorf("the report was typed: %q", term.written("s1"))
	}
	collected, err := svc.CollectNow("s1")
	if err != nil || len(collected.Results) != 1 || collected.Results[0].Answer != "the long report" {
		t.Errorf("collect = %+v, %v; want the report waiting", collected, err)
	}
}

// A mod that never collected the note leaves the report in the inbox, and the
// caller hears the short note typed instead.
func TestASubagentReportNoPollCollectedStaysCollectable(t *testing.T) {
	term := newFakeTerminal("s1", "s2")
	term.attachMod("s1", withdrawn)
	svc := viaMod(term, nil)
	plantSubagent(svc, "t1", "s1", "s2", "docs")

	if err := svc.Reply("", "t1", "the long report"); err != nil {
		t.Fatalf("Reply: %v", err)
	}
	if !awaitWritten(term, "s1", "[lich]") {
		t.Fatal("the short note was never typed")
	}
	if strings.Contains(term.written("s1"), "the long report") {
		t.Errorf("the report was typed: %q", term.written("s1"))
	}
	collected, err := svc.CollectNow("s1")
	if err != nil || len(collected.Results) != 1 {
		t.Errorf("collect = %+v, %v; want the report still waiting", collected, err)
	}
}

// A report the caller collected first is not delivered a second time.
func TestASubagentReportCollectedFirstIsNotAlsoDelivered(t *testing.T) {
	term := newFakeTerminal("s1", "s2")
	term.attachMod("s1", ackedOK)
	svc := viaMod(term, nil)
	svc.Observe("s1", stateBusy)
	plantSubagent(svc, "t1", "s1", "s2", "docs")

	if err := svc.Reply("", "t1", "the report"); err != nil {
		t.Fatalf("Reply: %v", err)
	}
	collected, err := svc.CollectNow("s1")
	if err != nil || len(collected.Results) != 1 {
		t.Fatalf("collect = %+v, %v; want the report", collected, err)
	}
	svc.Observe("s1", stateDone)
	svc.flushNudge("s1")
	if prompts := term.promptsTo("s1"); len(prompts) != 0 {
		t.Errorf("handed %q after the report was collected", prompts)
	}
}

// Results that are not subagent reports ride the same note as a nudge.
func TestASubagentReportCarriesTheNudgeForTheRest(t *testing.T) {
	term := newFakeTerminal("s1", "s2", "s3")
	term.attachMod("s1", ackedOK)
	svc := viaMod(term, nil)
	svc.Observe("s1", stateBusy)
	plantSubagent(svc, "t1", "s1", "s2", "docs")
	plant(svc, "t2", "s1", "s3", "api")

	for ticket, answer := range map[string]string{"t1": "the report", "t2": "the other answer"} {
		if err := svc.Reply("", ticket, answer); err != nil {
			t.Fatalf("Reply %s: %v", ticket, err)
		}
	}
	svc.Observe("s1", stateDone)
	if !awaitPrompts(term, "s1", 1) {
		t.Fatal("nothing reached the caller")
	}
	note := term.promptsTo("s1")[0]
	if !strings.Contains(note, "the report") || strings.Contains(note, "the other answer") {
		t.Errorf("note = %q, want the report inline and the other answer left to collect", note)
	}
	if !strings.Contains(note, `The task you sent "api" has its result ready`) {
		t.Errorf("note = %q, want the nudge for api", note)
	}
	collected, _ := svc.CollectNow("s1")
	if len(collected.Results) != 1 || collected.Results[0].Ticket != "t2" {
		t.Errorf("collect = %+v, want only the other answer", collected)
	}
}

func TestASubagentErrandOutlivesTheTTLWhileItsWorkerLives(t *testing.T) {
	term := newFakeTerminal("s1", "s2")
	svc := newRelay(workspace(), term, nil)
	plantSubagent(svc, "t1", "s1", "s2", "docs")
	plantSubagent(svc, "t2", "s1", "s2", "docs")
	svc.mu.Lock()
	svc.tickets["t1"].created = time.Now().Add(-2 * ticketTTL)
	lapsed := svc.tickets["t2"]
	delete(svc.tickets, "t2")
	lapsed.lapsed = time.Now().Add(-2 * ticketTTL)
	svc.lapsed["t2"] = lapsed
	svc.mu.Unlock()

	svc.CollectNow("s1")
	svc.mu.Lock()
	_, open := svc.tickets["t1"]
	_, late := svc.lapsed["t2"]
	svc.mu.Unlock()
	if !open || !late {
		t.Fatalf("open %v, lapsed %v; want both kept while the worker lives", open, late)
	}

	term.mu.Lock()
	term.live["s2"] = false
	term.mu.Unlock()
	svc.CollectNow("s1")
	svc.mu.Lock()
	_, open = svc.tickets["t1"]
	_, late = svc.lapsed["t2"]
	svc.mu.Unlock()
	if open || late {
		t.Errorf("open %v, lapsed %v; want both gone with the worker", open, late)
	}
}

func TestAWorkerBlockedOnAPermissionTellsItsCallerOncePerBlock(t *testing.T) {
	term := newFakeTerminal("s1", "s2")
	svc := newRelay(workspace(), term, nil)
	plantSubagent(svc, "t1", "s1", "s2", "docs")

	svc.Observe("s2", stateBusy)
	svc.Observe("s2", stateWaiting)
	svc.Observe("s2", stateWaiting)
	if !awaitWrites(term, "s1", 2) {
		t.Fatal("the caller was never told its worker is blocked")
	}
	note := term.written("s1")
	if !strings.Contains(note, `"docs"`) || !strings.Contains(note, "permission") {
		t.Errorf("note = %q, want the worker and the permission named", note)
	}

	svc.Observe("s2", stateBusy)
	svc.Observe("s2", stateWaiting)
	if !awaitWrites(term, "s1", 4) {
		t.Fatal("a second block was not told")
	}
	time.Sleep(50 * time.Millisecond)
	if writes := term.writesTo("s1"); len(writes) != 4 {
		t.Errorf("got %d writes, want one note per block: %q", len(writes), writes)
	}
}

// waiting after a turn ended is a prompt at rest, and an ordinary errand's
// worker is the user's to watch: neither is news for the caller.
func TestOnlyABlockedSubagentWorkerIsReported(t *testing.T) {
	term := newFakeTerminal("s1", "s2", "s3")
	svc := newRelay(workspace(), term, nil)
	plantSubagent(svc, "t1", "s1", "s2", "docs")
	plant(svc, "t2", "s1", "s3", "api")
	svc.mu.Lock()
	svc.tickets["t2"].submitted = true
	svc.mu.Unlock()

	svc.Observe("s3", stateBusy)
	svc.Observe("s3", stateWaiting)
	svc.Observe("s2", stateWaiting)
	time.Sleep(50 * time.Millisecond)
	if typed := term.written("s1"); typed != "" {
		t.Errorf("the caller was told %q", typed)
	}
}

// finishing wires a relay whose finished workers are sent down the returned
// channel.
func finishing(t *testing.T) (*Service, chan string) {
	t.Helper()
	svc := newRelay(workspace(), newFakeTerminal("s1", "s2", "s3"), nil)
	finished := make(chan string, 4)
	svc.SetWorkerFinished(func(id string) error {
		finished <- id
		return nil
	})
	return svc, finished
}

func expectFinished(t *testing.T, finished chan string, want string) {
	t.Helper()
	select {
	case got := <-finished:
		if got != want {
			t.Errorf("finished %q, want %q", got, want)
		}
	case <-time.After(2 * time.Second):
		t.Fatalf("%s was never reported finished", want)
	}
}

func expectNoneFinished(t *testing.T, finished chan string) {
	t.Helper()
	select {
	case got := <-finished:
		t.Errorf("finished %q, want no worker finished", got)
	case <-time.After(50 * time.Millisecond):
	}
}

// A worker that reported and then ended the turn it reported in is done, as a
// native subagent is once its result is back.
func TestAWorkerThatReportedAndEndedItsTurnIsFinished(t *testing.T) {
	svc, finished := finishing(t)
	plantSubagent(svc, "t1", "s1", "s2", "docs")

	if err := svc.Reply("s2", "t1", "the report"); err != nil {
		t.Fatalf("Reply: %v", err)
	}
	expectNoneFinished(t, finished)
	svc.Observe("s2", stateDone)
	expectFinished(t, finished, "s2")

	// Once: a later turn of the same session is not another finish.
	svc.Observe("s2", stateBusy)
	svc.Observe("s2", stateDone)
	expectNoneFinished(t, finished)
}

// A report that arrives after the errand lapsed (the worker ended a turn
// without answering, then answered later) finishes the worker the same way.
func TestAWorkerThatReportedLateIsFinished(t *testing.T) {
	svc, finished := finishing(t)
	plantSubagent(svc, "t1", "s1", "s2", "docs")
	svc.Observe("s2", stateBusy)
	svc.Observe("s2", stateDone)
	expectNoneFinished(t, finished)

	svc.Observe("s2", stateBusy)
	if err := svc.Reply("s2", "t1", "the late report"); err != nil {
		t.Fatalf("Reply: %v", err)
	}
	svc.Observe("s2", stateDone)
	expectFinished(t, finished, "s2")
}

// Only a subagent errand ends with its worker: a session answering an ordinary
// send is somebody's peer, and one still holding another errand has work left.
func TestAWorkerWithMoreToDoIsNotFinished(t *testing.T) {
	t.Run("ordinary errand", func(t *testing.T) {
		svc, finished := finishing(t)
		plant(svc, "t1", "s1", "s2", "docs")
		if err := svc.Reply("s2", "t1", "an answer"); err != nil {
			t.Fatalf("Reply: %v", err)
		}
		svc.Observe("s2", stateDone)
		expectNoneFinished(t, finished)
	})
	t.Run("another errand open", func(t *testing.T) {
		svc, finished := finishing(t)
		plantSubagent(svc, "t1", "s1", "s2", "docs")
		plant(svc, "t2", "s3", "s2", "docs")
		if err := svc.Reply("s2", "t1", "the report"); err != nil {
			t.Fatalf("Reply: %v", err)
		}
		svc.Observe("s2", stateDone)
		expectNoneFinished(t, finished)
	})
	t.Run("interrupted", func(t *testing.T) {
		svc, finished := finishing(t)
		plantSubagent(svc, "t1", "s1", "s2", "docs")
		if err := svc.Reply("s2", "t1", "the report"); err != nil {
			t.Fatalf("Reply: %v", err)
		}
		svc.Observe("s2", stateInterrupted)
		expectNoneFinished(t, finished)
	})
}
