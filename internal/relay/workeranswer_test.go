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
	got, err := svc.SendSubagent(context.Background(), SendOptions{From: "s1", Target: "docs", Prompt: "write the docs", WaitSeconds: 1})
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

	got, err := svc.Send(context.Background(), SendOptions{From: "s1", Target: "docs", Prompt: "write the docs", WaitSeconds: 1})
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

	got, err := svc.Wait(context.Background(), WaitOptions{Ticket: "t1", WaitSeconds: 1})
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

	if err := svc.Reply(ReplyOptions{From: "s2", Ticket: "t1", Answer: "the agent's own reply"}); err != nil {
		t.Fatalf("Reply: %v", err)
	}
	svc.WorkerAnswered("s2", "the mod's copy")

	got, err := svc.Wait(context.Background(), WaitOptions{Ticket: "t1", WaitSeconds: 1})
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
	got, err := svc.Wait(context.Background(), WaitOptions{Ticket: "t1", WaitSeconds: 1})
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

// expectOpen fails unless the errand is still open: no answer reached it.
func expectOpen(t *testing.T, svc *Service, id string) {
	t.Helper()
	svc.mu.Lock()
	defer svc.mu.Unlock()
	if svc.tickets[id] == nil {
		t.Fatalf("errand %s was closed", id)
	}
}

func expectAnswer(t *testing.T, svc *Service, id, want string) {
	t.Helper()
	got, err := svc.Wait(context.Background(), WaitOptions{Ticket: id, WaitSeconds: 1})
	if err != nil || got.Status != StatusAnswered || got.Answer != want {
		t.Fatalf("Wait(%s) = %+v, %v, want answered %q", id, got, err, want)
	}
}

// A worker that hands a task to a session of its own ends its turn with
// nothing in Claude Code's background list, since that session is not one of
// Claude Code's tasks. Its mod reports that turn, and the report is only "I
// handed it off": the errand must wait for the turn that has the result.
func TestAWorkersAnswerWaitsForTheErrandsItSent(t *testing.T) {
	term := newFakeTerminal("s1", "s2", "s3")
	svc := newRelay(workspace(), term, nil)
	plantAnsweredByMod(svc, "t1", "s1", "s2", "docs")
	plantAnsweredByMod(svc, "t2", "s2", "s3", "api")

	svc.Observe("s2", stateBusy)
	svc.WorkerAnswered("s2", "handed the api part to a worker")
	svc.Observe("s2", stateDone)
	expectOpen(t, svc, "t1")

	svc.WorkerAnswered("s3", "the api is done")
	if !awaitWritten(term, "s2", "api") {
		t.Fatal("the worker was never told its own worker finished")
	}
	svc.Observe("s2", stateBusy)
	svc.WorkerAnswered("s2", "docs and api are done")
	expectAnswer(t, svc, "t1", "docs and api are done")
}

// An ordinary errand the worker sent holds its answer the same way: its
// outcome reaches the worker's prompt and starts the turn that answers.
func TestAWorkersAnswerWaitsForAnOrdinaryErrandItSent(t *testing.T) {
	svc := newRelay(workspace(), newFakeTerminal("s1", "s2", "s3"), nil)
	plantAnsweredByMod(svc, "t1", "s1", "s2", "docs")
	plant(svc, "t2", "s2", "s3", "api")

	svc.WorkerAnswered("s2", "asked api, waiting")
	expectOpen(t, svc, "t1")
}

// A private errand reaches no prompt, so nothing would ever resume the worker
// for it: it does not hold the answer.
func TestAPrivateErrandDoesNotHoldAWorkersAnswer(t *testing.T) {
	svc := newRelay(workspace(), newFakeTerminal("s1", "s2", "s3"), nil)
	plantAnsweredByMod(svc, "t1", "s1", "s2", "docs")
	plant(svc, "t2", "s2", "s3", "api")
	svc.mu.Lock()
	svc.tickets["t2"].private = true
	svc.mu.Unlock()

	svc.WorkerAnswered("s2", "the report")
	expectAnswer(t, svc, "t1", "the report")
}

// A result that lands while the worker is mid-turn is handed to it only when
// that turn ends, so the turn's own report was written without it.
func TestAWorkersAnswerWaitsForAResultItHasNotRead(t *testing.T) {
	term := newFakeTerminal("s1", "s2", "s3")
	svc := newRelay(workspace(), term, nil)
	plantAnsweredByMod(svc, "t1", "s1", "s2", "docs")
	plantAnsweredByMod(svc, "t2", "s2", "s3", "api")

	svc.Observe("s2", stateBusy)
	svc.WorkerAnswered("s3", "the api is done")
	svc.Observe("s2", stateDone)
	svc.WorkerAnswered("s2", "still waiting on api")
	expectOpen(t, svc, "t1")

	if !awaitWritten(term, "s2", "api") {
		t.Fatal("the worker was never told its own worker finished")
	}
	svc.Observe("s2", stateBusy)
	svc.WorkerAnswered("s2", "docs and api are done")
	expectAnswer(t, svc, "t1", "docs and api are done")
}

// A result the worker was told about and chose not to collect does not hold
// its answer forever: the turn after the notice is the one that read it.
func TestAResultTheWorkerWasToldAboutDoesNotHoldItsAnswer(t *testing.T) {
	term := newFakeTerminal("s1", "s2", "s3")
	svc := newRelay(workspace(), term, nil)
	plantAnsweredByMod(svc, "t1", "s1", "s2", "docs")
	plant(svc, "t2", "s2", "s3", "api")

	if err := svc.Reply(ReplyOptions{From: "s3", Ticket: "t2", Answer: "the api answer"}); err != nil {
		t.Fatalf("Reply: %v", err)
	}
	if !awaitWritten(term, "s2", "api") {
		t.Fatal("the worker was never told the result is waiting")
	}
	svc.Observe("s2", stateBusy)
	svc.WorkerAnswered("s2", "the report")
	expectAnswer(t, svc, "t1", "the report")
}

// A worker whose own worker is closed under it would wait forever on an errand
// that ended in silence. Someone is waiting on it, so it hears the errand
// stopped, and that is what resumes it.
func TestAWorkerHearsItsOwnWorkerStopped(t *testing.T) {
	svc := newRelay(workspace(), newFakeTerminal("s1", "s2", "s3"), nil)
	plantAnsweredByMod(svc, "t1", "s1", "s2", "docs")
	plantAnsweredByMod(svc, "t2", "s2", "s3", "api")

	svc.SessionClosed("s3", "s2")

	collected, err := svc.CollectNow(CollectNowOptions{From: "s2"})
	if err != nil {
		t.Fatalf("CollectNow: %v", err)
	}
	if len(collected.Results) != 1 || collected.Results[0].Status != StatusStopped {
		t.Fatalf("collect = %+v, want the errand stopped", collected)
	}
	svc.Observe("s2", stateBusy)
	svc.WorkerAnswered("s2", "api was stopped; docs are done")
	expectAnswer(t, svc, "t1", "api was stopped; docs are done")
}

// An errand the worker sent that nobody answers within ticketTTL would hold
// its report until something else started a turn there. Someone is waiting on
// the worker, so it hears the errand expired, and that is what resumes it.
func TestAWorkerHearsItsOwnErrandExpired(t *testing.T) {
	term := newFakeTerminal("s1", "s2", "s3")
	svc := newRelay(workspace(), term, nil)
	plantAnsweredByMod(svc, "t1", "s1", "s2", "docs")
	plant(svc, "t2", "s2", "s3", "api")
	svc.WorkerAnswered("s2", "asked api, waiting")

	later := time.Now().Add(2 * ticketTTL)
	svc.now = func() time.Time { return later }
	svc.expireTickets()

	if !awaitWritten(term, "s2", "api") {
		t.Fatal("the worker was never told its errand expired")
	}
	collected, err := svc.CollectNow(CollectNowOptions{From: "s2"})
	if err != nil {
		t.Fatalf("CollectNow: %v", err)
	}
	if len(collected.Results) != 1 || collected.Results[0].Status != StatusExpired {
		t.Fatalf("collect = %+v, want the errand expired", collected)
	}
	svc.Observe("s2", stateBusy)
	svc.WorkerAnswered("s2", "api never answered; docs are done")
	expectAnswer(t, svc, "t1", "api never answered; docs are done")
}

// A worker that answered by its ticket while its own worker still runs is not
// closed under that worker: the report it is owed would reach no one.
func TestAWorkerWithAnErrandOfItsOwnOpenIsNotFinished(t *testing.T) {
	svc, finished := finishing(t)
	plantSubagent(svc, "t1", "s1", "s2", "docs")
	plantSubagent(svc, "t2", "s2", "s3", "api")

	svc.Observe("s2", stateBusy)
	if err := svc.Reply(ReplyOptions{From: "s2", Ticket: "t1", Answer: "the report"}); err != nil {
		t.Fatalf("Reply: %v", err)
	}
	svc.Observe("s2", stateDone)
	expectNoneFinished(t, finished)
}

// A worker whose last turn ended with nothing to report (blank, an API error, a
// refusal) leaves its mod nothing to answer with. Kept open past ticketTTL
// while the worker runs, its errand would never end: the caller hears
// unanswered, with why.
func TestAWorkersUnansweredTurnEndsItsErrand(t *testing.T) {
	svc := newRelay(workspace(), newFakeTerminal("s1", "s2"), nil)
	plantAnsweredByMod(svc, "t1", "s1", "s2", "docs")

	svc.Observe("s2", stateBusy)
	svc.Observe("s2", stateDone)
	svc.WorkerUnanswered("s2", UnansweredRefusal)

	collected, err := svc.CollectNow(CollectNowOptions{From: "s1"})
	if err != nil {
		t.Fatalf("CollectNow: %v", err)
	}
	if len(collected.Results) != 1 {
		t.Fatalf("collect = %+v, want the errand's outcome", collected)
	}
	got := collected.Results[0]
	if got.Status != StatusUnanswered || !strings.Contains(got.Answer, "refusal") {
		t.Errorf("result = %+v, want unanswered naming the refusal", got)
	}
}

func TestACallerHoldingTheLineHearsAWorkerWentUnanswered(t *testing.T) {
	svc := newRelay(workspace(), newFakeTerminal("s1", "s2"), nil)
	plantAnsweredByMod(svc, "t1", "s1", "s2", "docs")

	waited := make(chan Result, 1)
	go func() {
		got, _ := svc.Wait(context.Background(), WaitOptions{Ticket: "t1", WaitSeconds: 5})
		waited <- got
	}()
	awaitAttended(t, svc, "t1", 1)
	svc.WorkerUnanswered("s2", UnansweredError)

	select {
	case got := <-waited:
		if got.Status != StatusUnanswered || !strings.Contains(got.Answer, "error") {
			t.Errorf("Wait = %+v, want unanswered naming the error", got)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("the caller holding the line never heard")
	}
}

// The worker's card stays open: its user can still steer it to an answer, and
// that answer reaches the caller by the ticket.
func TestAWorkersAnswerAfterAnUnansweredTurnStillReachesTheCaller(t *testing.T) {
	svc := newRelay(workspace(), newFakeTerminal("s1", "s2"), nil)
	plantAnsweredByMod(svc, "t1", "s1", "s2", "docs")

	svc.WorkerUnanswered("s2", UnansweredBlank)
	svc.WorkerAnswered("s2", "the report after all")

	expectAnswer(t, svc, "t1", "the report after all")
}

// An errand already ended is not ended twice: a second empty turn files no
// second outcome.
func TestASecondUnansweredTurnFilesNothingMore(t *testing.T) {
	svc := newRelay(workspace(), newFakeTerminal("s1", "s2"), nil)
	plantAnsweredByMod(svc, "t1", "s1", "s2", "docs")

	svc.WorkerUnanswered("s2", UnansweredBlank)
	svc.WorkerUnanswered("s2", UnansweredBlank)

	svc.mu.Lock()
	defer svc.mu.Unlock()
	if len(svc.ready) != 1 {
		t.Errorf("inbox = %v, want one outcome", svc.ready)
	}
}

// An empty turn while an errand of the worker's own is out is the turn that
// handed it off: the outcome reaching the worker resumes it.
func TestAnUnansweredTurnWaitsForTheErrandsTheWorkerSent(t *testing.T) {
	svc := newRelay(workspace(), newFakeTerminal("s1", "s2", "s3"), nil)
	plantAnsweredByMod(svc, "t1", "s1", "s2", "docs")
	plant(svc, "t2", "s2", "s3", "api")

	svc.WorkerUnanswered("s2", UnansweredBlank)
	expectOpen(t, svc, "t1")
}

// A usage limit is an API error lich parks the continuation of, and that
// continuation is the turn that answers.
func TestAnErrorWhileAResumeIsParkedKeepsTheErrandOpen(t *testing.T) {
	svc := newRelay(workspace(), newFakeTerminal("s1", "s2"), nil)
	plantAnsweredByMod(svc, "t1", "s1", "s2", "docs")
	svc.markResume("s2")

	svc.WorkerUnanswered("s2", UnansweredError)
	expectOpen(t, svc, "t1")

	svc.WorkerUnanswered("s2", UnansweredRefusal)
	if _, err := svc.Wait(context.Background(), WaitOptions{Ticket: "t1", WaitSeconds: 1}); err != nil {
		t.Fatalf("Wait: %v", err)
	}
}

// An ordinary errand is not the mod's to end.
func TestAnUnansweredTurnLeavesAnOrdinaryErrandAlone(t *testing.T) {
	svc := newRelay(workspace(), newFakeTerminal("s1", "s2"), nil)
	plant(svc, "t1", "s1", "s2", "docs")

	svc.WorkerUnanswered("s2", UnansweredBlank)
	expectOpen(t, svc, "t1")
}

// A worker that answers by its ticket and runs a subagent of its own in the
// background ends its turn waiting for that subagent: the report reaching its
// prompt is what resumes it. That turn ending is not the worker finishing
// without an answer, whether the report is still on its way or already waits
// in its inbox for the prompt to free up.
func TestATurnEndingWhileTheWorkerAwaitsItsOwnErrandKeepsItsErrandOpen(t *testing.T) {
	for name, reportFirst := range map[string]bool{"report still running": false, "report waiting": true} {
		t.Run(name, func(t *testing.T) {
			term := newFakeTerminal("s1", "s2", "s3")
			svc := newRelay(workspace(), term, nil)
			plantSubagent(svc, "t1", "s1", "s2", "docs")
			plantSubagent(svc, "t2", "s2", "s3", "api")

			svc.Observe("s2", stateBusy)
			if reportFirst {
				if err := svc.Reply(ReplyOptions{From: "s3", Ticket: "t2", Answer: "the api is done"}); err != nil {
					t.Fatalf("Reply: %v", err)
				}
			}
			svc.Observe("s2", stateDone)
			expectOpen(t, svc, "t1")

			if !reportFirst {
				if err := svc.Reply(ReplyOptions{From: "s3", Ticket: "t2", Answer: "the api is done"}); err != nil {
					t.Fatalf("Reply: %v", err)
				}
			}
			if !awaitWritten(term, "s2", "api") {
				t.Fatal("the worker was never told its own worker finished")
			}
			svc.Observe("s2", stateBusy)
			if err := svc.Reply(ReplyOptions{From: "s2", Ticket: "t1", Answer: "docs and api are done"}); err != nil {
				t.Fatalf("Reply: %v", err)
			}
			expectAnswer(t, svc, "t1", "docs and api are done")
		})
	}
}
