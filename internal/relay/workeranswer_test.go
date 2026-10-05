package relay

import (
	"context"
	"strings"
	"testing"
	"time"
)

// ModAnswers answers with what answerByMod recorded for that session.
func (f *fakeTerminal) ModAnswers(id string) bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.answering[id]
}

// answerByMod gives a session a mod that answers its subagent errands itself
// (docs/hooks/mod-answer.md), polling with receipts as attachMod does.
func (f *fakeTerminal) answerByMod(id string, receipts ...PromptReceipt) {
	f.attachMod(id, receipts...)
	f.mu.Lock()
	defer f.mu.Unlock()
	f.answering[id] = true
}

// plantAnsweredByMod is plantSubagent for an errand whose worker's mod answers
// it.
func plantAnsweredByMod(svc *Service, id, fromID, targetID, target string) {
	plantSubagent(svc, id, fromID, targetID, target)
	svc.mu.Lock()
	svc.tickets[id].modAnswers = true
	svc.mu.Unlock()
}

func sendSubagentTask(t *testing.T, svc *Service) Result {
	t.Helper()
	got, err := svc.SendSubagent(context.Background(), "s1", "docs", "", "write the docs", 1)
	if err != nil {
		t.Fatalf("SendSubagent: %v", err)
	}
	return got
}

// A worker whose mod answers for it is handed the task the way a native
// subagent is: no ticket, no reply command, only who asked.
func TestATaskForAWorkerWhoseModAnswersCarriesNoTicket(t *testing.T) {
	term := newFakeTerminal("s1", "s2")
	term.answerByMod("s2", ackedOK)
	svc := viaMod(term, nil)

	got := sendSubagentTask(t, svc)
	if !awaitPrompts(term, "s2", 1) {
		t.Fatal("the worker's mod was never handed the task")
	}
	task := term.promptsTo("s2")[0]
	if !strings.Contains(task, "write the docs") || !strings.Contains(task, `"sender"`) {
		t.Errorf("task = %q, want the prompt under a line naming the caller", task)
	}
	for _, unwanted := range []string{got.Ticket, "reply", ToolReply} {
		if strings.Contains(task, unwanted) {
			t.Errorf("task carries %q:\n%s", unwanted, task)
		}
	}
}

// A worker whose mod does not answer (an older plugin, or none) still needs the
// ticket: its reply is the only way home.
func TestATaskForAWorkerWhoseModDoesNotAnswerKeepsTheTicket(t *testing.T) {
	term := newFakeTerminal("s1", "s2")
	term.attachMod("s2", ackedOK)
	svc := viaMod(term, nil)

	got := sendSubagentTask(t, svc)
	if !awaitPrompts(term, "s2", 1) {
		t.Fatal("the worker's mod was never handed the task")
	}
	if task := term.promptsTo("s2")[0]; !strings.Contains(task, "reply "+got.Ticket) {
		t.Errorf("task = %q, want the reply command naming ticket %s", task, got.Ticket)
	}
}

// Only a subagent errand is answered by the mod: an ordinary send to the same
// session keeps its ticket.
func TestAnOrdinaryTaskToAWorkerWhoseModAnswersKeepsTheTicket(t *testing.T) {
	term := newFakeTerminal("s1", "s2")
	term.answerByMod("s2", ackedOK)
	svc := viaMod(term, nil)

	got, err := svc.Send(context.Background(), "s1", "docs", "", "write the docs", 1)
	if err != nil {
		t.Fatalf("Send: %v", err)
	}
	if !awaitPrompts(term, "s2", 1) {
		t.Fatal("the mod was never handed the task")
	}
	if task := term.promptsTo("s2")[0]; !strings.Contains(task, "reply "+got.Ticket) {
		t.Errorf("task = %q, want the reply command naming ticket %s", task, got.Ticket)
	}
}

func TestAWorkersModAnswersItsErrand(t *testing.T) {
	svc := newRelay(workspace(), newFakeTerminal("s1", "s2"), nil)
	plantAnsweredByMod(svc, "t1", "s1", "s2", "docs")

	svc.WorkerAnswered("s2", "the report")

	got, err := svc.Wait(context.Background(), "t1", 1)
	if err != nil {
		t.Fatalf("Wait: %v", err)
	}
	if got.Status != StatusAnswered || got.Answer != "the report" {
		t.Errorf("Wait = %+v, want the mod's answer", got)
	}
}

// The caller reads a mod's answer exactly as it reads a reply: whole, through
// its own mod, as a finished background agent.
func TestAModsAnswerReachesTheCallerAsTheReport(t *testing.T) {
	term := newFakeTerminal("s1", "s2")
	term.attachMod("s1", ackedOK)
	svc := viaMod(term, nil)
	plantAnsweredByMod(svc, "t1", "s1", "s2", "docs")

	svc.WorkerAnswered("s2", "Rewrote docs/cli.md.")

	if !awaitPrompts(term, "s1", 1) {
		t.Fatal("the caller's mod was never handed the report")
	}
	if note := term.promptsTo("s1")[0]; !strings.Contains(note, "Rewrote docs/cli.md.") {
		t.Errorf("note = %q, want the report", note)
	}
	if got := term.notesTo("s1")[0]; got == nil || got.Status != NotifyCompleted {
		t.Errorf("the report was handed as %+v, want a completed notification", got)
	}
}

// An answer from a session nobody handed a subagent errand is dropped: the mod
// cannot tell a worker from a session whose subagents the user kept native.
func TestAnAnswerFromASessionWithNoSubagentErrandIsIgnored(t *testing.T) {
	svc := newRelay(workspace(), newFakeTerminal("s1", "s2"), nil)
	plant(svc, "t1", "s1", "s2", "docs")

	svc.WorkerAnswered("s2", "unrelated chatter")

	svc.mu.Lock()
	defer svc.mu.Unlock()
	if svc.tickets["t1"] == nil || len(svc.ready) != 0 {
		t.Errorf("an ordinary errand was answered by a mod's report: tickets %v, inbox %v", svc.tickets, svc.ready)
	}
}

// The worker may still call reply_to_session itself: the first answer wins and
// the mod's, landing after it, is dropped.
func TestTheFirstAnswerWins(t *testing.T) {
	svc := newRelay(workspace(), newFakeTerminal("s1", "s2"), nil)
	plantAnsweredByMod(svc, "t1", "s1", "s2", "docs")

	if err := svc.Reply("s2", "t1", "the agent's own reply"); err != nil {
		t.Fatalf("Reply: %v", err)
	}
	svc.WorkerAnswered("s2", "the mod's copy")

	got, err := svc.Wait(context.Background(), "t1", 1)
	if err != nil {
		t.Fatalf("Wait: %v", err)
	}
	if got.Answer != "the agent's own reply" {
		t.Errorf("answer = %q, want the first one", got.Answer)
	}
}

// With two subagent errands open nothing says which the text answers, so
// neither is closed on a guess.
func TestAnAnswerBetweenTwoSubagentErrandsIsIgnored(t *testing.T) {
	svc := newRelay(workspace(), newFakeTerminal("s1", "s2"), nil)
	plantAnsweredByMod(svc, "t1", "s1", "s2", "docs")
	plantAnsweredByMod(svc, "t2", "s1", "s2", "docs")

	svc.WorkerAnswered("s2", "which one?")

	svc.mu.Lock()
	defer svc.mu.Unlock()
	if svc.tickets["t1"] == nil || svc.tickets["t2"] == nil {
		t.Error("an errand was closed on a guess")
	}
}

// A worker that hands work to the background ends its turn with nothing to
// report yet; its mod answers when it resumes. That turn ending must not tell
// the caller the errand went unanswered.
func TestATurnEndingWithoutTheModsAnswerKeepsTheErrandOpen(t *testing.T) {
	term := newFakeTerminal("s1", "s2")
	svc := newRelay(workspace(), term, nil)
	plantAnsweredByMod(svc, "t1", "s1", "s2", "docs")

	svc.Observe("s2", stateBusy)
	svc.Observe("s2", stateDone)
	time.Sleep(20 * time.Millisecond)

	svc.mu.Lock()
	open, inbox := svc.tickets["t1"] != nil, len(svc.ready)
	svc.mu.Unlock()
	if !open || inbox != 0 {
		t.Fatalf("errand open = %v, inbox = %d: want it open and nothing filed", open, inbox)
	}
	if typed := term.written("s1"); typed != "" {
		t.Errorf("the caller was told %q", typed)
	}

	svc.Observe("s2", stateBusy)
	svc.WorkerAnswered("s2", "finished after the sleep")
	got, err := svc.Wait(context.Background(), "t1", 1)
	if err != nil || got.Answer != "finished after the sleep" {
		t.Errorf("Wait = %+v, %v, want the answer from the resumed turn", got, err)
	}
}

// The mod's answer and the turn's done report race: whichever lands last
// finishes the worker.
func TestAWorkerWhoseAnswerLandsAfterItsTurnEndedIsFinished(t *testing.T) {
	svc, finished := finishing(t)
	plantAnsweredByMod(svc, "t1", "s1", "s2", "docs")

	svc.Observe("s2", stateBusy)
	svc.Observe("s2", stateDone)
	expectNoneFinished(t, finished)
	svc.WorkerAnswered("s2", "the report")
	expectFinished(t, finished, "s2")
}

func TestAWorkerWhoseAnswerLandsMidTurnIsFinishedWhenItEnds(t *testing.T) {
	svc, finished := finishing(t)
	plantAnsweredByMod(svc, "t1", "s1", "s2", "docs")

	svc.Observe("s2", stateBusy)
	svc.WorkerAnswered("s2", "the report")
	expectNoneFinished(t, finished)
	svc.Observe("s2", stateDone)
	expectFinished(t, finished, "s2")
}
