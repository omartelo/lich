package terminal

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/omartelo/lich/internal/events"
)

// attachedModService is a service whose session s1 runs a mod that has polled
// once, so commands for it are queued.
func attachedModService(t *testing.T) *Service {
	t.Helper()
	svc := newModService(t, events.New(), 10*time.Millisecond)
	runModSession(svc, "s1")
	pollMod(t, svc.ws, "s1")
	return svc
}

type modRun struct {
	out ModOutcome
	err error
}

// runInBackground waits on RunModCommand off the test goroutine.
func runInBackground(ctx context.Context, svc *Service, cmd ModCommand) <-chan modRun {
	done := make(chan modRun, 1)
	go func() {
		out, err := svc.RunModCommand(ctx, "s1", cmd)
		done <- modRun{out, err}
	}()
	return done
}

// collectMod polls until the session's mod is handed a command, the way a mod
// re-polls after every empty answer.
func collectMod(t *testing.T, tr *transport, session string) ModCommand {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if cmds := pollMod(t, tr, session); len(cmds) > 0 {
			return cmds[0]
		}
	}
	t.Fatal("no command reached the mod")
	return ModCommand{}
}

func finished(t *testing.T, done <-chan modRun) modRun {
	t.Helper()
	select {
	case got := <-done:
		return got
	case <-time.After(5 * time.Second):
		t.Fatal("RunModCommand never returned")
		return modRun{}
	}
}

func ackBody(session, id, kind string, ok bool, reason string) string {
	return fmt.Sprintf(`{"session_id":%q,"id":%q,"kind":%q,"ok":%t,"error":%q}`, session, id, kind, ok, reason)
}

func TestRunModCommandReportsTheAck(t *testing.T) {
	tests := []struct {
		name   string
		cmd    ModCommand
		ok     bool
		reason string
		want   string
	}{
		{"applied", ModCommand{Kind: ModAbort}, true, "", ""},
		{"failed, with the mod's reason", ModCommand{Kind: ModAbort}, false, "no turn is running", "no turn is running"},
		{"refused by a Claude Code that knows no such command", ModCommand{Kind: ModRunCommand, Name: "nao-existe"},
			false, "$.command.run: no command named /nao-existe in this session",
			"$.command.run: no command named /nao-existe in this session"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			svc := attachedModService(t)
			done := runInBackground(context.Background(), svc, tc.cmd)
			cmd := collectMod(t, svc.ws, "s1")
			postModAck(t, svc.ws, ackBody("s1", cmd.ID, cmd.Kind, tc.ok, tc.reason))

			got := finished(t, done)
			want := ModOutcome{ID: cmd.ID, State: ModAcked, OK: tc.ok, Error: tc.want}
			if got.err != nil || got.out != want {
				t.Fatalf("RunModCommand = %+v, %v; want %+v", got.out, got.err, want)
			}
		})
	}
}

// A mod from before the command kind acks it as an unknown kind. What the user
// can do about that is update the plugin, so that is what they are told.
func TestRunModCommandNamesTheUpdateForAnOldMod(t *testing.T) {
	svc := attachedModService(t)
	done := runInBackground(context.Background(), svc, ModCommand{Kind: ModRunCommand, Name: "compact"})
	cmd := collectMod(t, svc.ws, "s1")
	postModAck(t, svc.ws, ackBody("s1", cmd.ID, ModRunCommand, false, "unknown kind"))

	got := finished(t, done)
	if got.out.State != ModAcked || got.out.OK || !strings.Contains(got.out.Error, "update lich-plugin to 0.15.0") {
		t.Fatalf("RunModCommand = %+v, want a failure naming the plugin update", got.out)
	}
}

// A wait that runs out before any poll withdraws the command: past the wait
// nobody is told it is still coming, so it must never run. The contract
// changed here: it used to stay queued for a poll inside the attach window.
func TestRunModCommandTimingOutBeforeAPollWithdrawsIt(t *testing.T) {
	svc := attachedModService(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	out, err := svc.RunModCommand(ctx, "s1", ModCommand{Kind: ModAbort})
	if err != nil {
		t.Fatalf("RunModCommand: %v", err)
	}
	if out.State != ModWithdrawn || out.ID == "" {
		t.Fatalf("outcome = %+v, want withdrawn with its id", out)
	}
	if got := pollMod(t, svc.ws, "s1"); len(got) != 0 {
		t.Fatalf("poll after the wait = %v, want nothing: the command was withdrawn", got)
	}
}

// Withdrawing one command leaves the others queued for the same session.
func TestRunModCommandWithdrawsOnlyItsOwnCommand(t *testing.T) {
	svc := attachedModService(t)
	kept, err := svc.EnqueueModCommand("s1", ModCommand{Kind: ModPrompt, Text: "go"})
	if err != nil {
		t.Fatalf("enqueue: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := svc.RunModCommand(ctx, "s1", ModCommand{Kind: ModAbort}); err != nil {
		t.Fatalf("RunModCommand: %v", err)
	}
	if got := pollMod(t, svc.ws, "s1"); !reflect.DeepEqual(got, []ModCommand{{ID: kept, Kind: ModPrompt, Text: "go"}}) {
		t.Fatalf("poll after the wait = %v, want only the prompt", got)
	}
}

func TestRunModCommandTimingOutAfterAPollIsDelivered(t *testing.T) {
	svc := attachedModService(t)
	ctx, cancel := context.WithCancel(context.Background())
	done := runInBackground(ctx, svc, ModCommand{Kind: ModPrompt, Text: "go"})
	cmd := collectMod(t, svc.ws, "s1")
	cancel()

	got := finished(t, done)
	if want := (ModOutcome{ID: cmd.ID, State: ModDelivered}); got.err != nil || got.out != want {
		t.Fatalf("RunModCommand = %+v, %v; want %+v", got.out, got.err, want)
	}
}

// An ack carries its session: another session's mod acking the same id does
// not settle the wait.
func TestRunModCommandIgnoresAnotherSessionsAck(t *testing.T) {
	svc := attachedModService(t)
	ctx, cancel := context.WithCancel(context.Background())
	done := runInBackground(ctx, svc, ModCommand{Kind: ModAbort})
	cmd := collectMod(t, svc.ws, "s1")
	postModAck(t, svc.ws, ackBody("s2", cmd.ID, ModAbort, true, ""))
	cancel()

	if got := finished(t, done); got.out.State != ModDelivered {
		t.Fatalf("outcome = %+v, want delivered: the ack was not this session's", got.out)
	}
}

func TestRunModCommandEndsWithTheSession(t *testing.T) {
	svc := attachedModService(t)
	done := runInBackground(context.Background(), svc, ModCommand{Kind: ModAbort})
	cmd := collectMod(t, svc.ws, "s1")
	if err := svc.Close("s1"); err != nil {
		t.Fatalf("close: %v", err)
	}

	got := finished(t, done)
	if want := (ModOutcome{ID: cmd.ID, State: ModEnded}); got.out != want {
		t.Fatalf("RunModCommand = %+v, want %+v", got.out, want)
	}
}

// A session whose process exited (or never started) is not running, which is
// not the same fix as a session whose mod is missing.
func TestRunModCommandRefusesASessionThatIsNotRunning(t *testing.T) {
	svc := attachedModService(t)
	if err := svc.Close("s1"); err != nil {
		t.Fatalf("close: %v", err)
	}
	for _, id := range []string{"s1", "never-started"} {
		if _, err := svc.RunModCommand(context.Background(), id, ModCommand{Kind: ModAbort}); !errors.Is(err, errModNotRunning) {
			t.Fatalf("%s: err = %v, want errModNotRunning", id, err)
		}
	}
}

func TestRunModCommandRefusesADetachedSession(t *testing.T) {
	svc := newModService(t, events.New(), 10*time.Millisecond)
	runModSession(svc, "s1")
	if _, err := svc.RunModCommand(context.Background(), "s1", ModCommand{Kind: ModAbort}); !errors.Is(err, errModDetached) {
		t.Fatalf("err = %v, want errModDetached", err)
	}
}

// An ack can land before anyone waits on it (a fast mod, a slow caller). It is
// kept for the wait rather than lost.
func TestModAckBeforeTheWaitIsKept(t *testing.T) {
	svc := attachedModService(t)
	w, err := svc.queueModCommand("s1", ModCommand{Kind: ModAbort})
	if err != nil {
		t.Fatalf("queue: %v", err)
	}
	svc.ws.mods.settle(modAckRequest{SessionID: "s1", ID: w.id, OK: true})
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if got := svc.ws.mods.await(ctx, w); got.State != ModAcked || !got.OK {
		t.Fatalf("await = %+v, want the ack that came first", got)
	}
}

func TestModCommandDropsTheLeadingSlash(t *testing.T) {
	svc := attachedModService(t)
	id, err := svc.EnqueueModCommand("s1", ModCommand{Kind: ModRunCommand, Name: " /compact", Args: "keep the plan"})
	if err != nil {
		t.Fatalf("enqueue: %v", err)
	}
	want := []ModCommand{{ID: id, Kind: ModRunCommand, Name: "compact", Args: "keep the plan"}}
	if got := pollMod(t, svc.ws, "s1"); !reflect.DeepEqual(got, want) {
		t.Fatalf("poll = %v, want %v", got, want)
	}
}

// An ask's answer rides its ack, and an answer runs past the 4 KiB every other
// hook body is held to.
func TestRunModCommandReturnsAnAsksAnswer(t *testing.T) {
	svc := attachedModService(t)
	done := runInBackground(context.Background(), svc, ModCommand{Kind: ModAsk, Question: "what now?"})
	cmd := collectMod(t, svc.ws, "s1")
	if cmd.Kind != ModAsk || cmd.Question != "what now?" {
		t.Fatalf("the mod got %+v, want the ask", cmd)
	}
	answer := strings.Repeat("é", 16000)
	postModAck(t, svc.ws, fmt.Sprintf(`{"session_id":"s1","id":%q,"kind":"ask","ok":true,"answer":%q}`, cmd.ID, answer))

	got := finished(t, done)
	want := ModOutcome{ID: cmd.ID, State: ModAcked, OK: true, Answer: answer}
	if got.err != nil || got.out != want {
		t.Fatalf("RunModCommand = %+v, %v; want the answer", got.out.State, got.err)
	}
}

func TestModAckOverItsLimitIsRefused(t *testing.T) {
	svc := attachedModService(t)
	body := fmt.Sprintf(`{"session_id":"s1","id":"m1","kind":"ask","ok":true,"answer":%q}`,
		strings.Repeat("x", modAckBodyLimit))
	url := fmt.Sprintf("http://127.0.0.1:%d/mod/acks?token=%s", svc.ws.port, svc.ws.token)
	resp, err := http.Post(url, "application/json", strings.NewReader(body))
	if err != nil {
		t.Fatalf("post ack: %v", err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("ack status = %d, want 400", resp.StatusCode)
	}
}
