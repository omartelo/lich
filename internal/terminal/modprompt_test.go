package terminal

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/omartelo/lich/internal/events"
	"github.com/omartelo/lich/internal/relay"
)

func TestSubmitPromptFindsNoModToHandItTo(t *testing.T) {
	unpolled := newModService(t, events.New(), 10*time.Millisecond)
	runModSession(unpolled, "s1")
	stopped := attachedModService(t)
	if err := stopped.Close("s1"); err != nil {
		t.Fatalf("close: %v", err)
	}
	tests := []struct {
		name string
		svc  *Service
		id   string
	}{
		{"no poll", unpolled, "s1"},
		{"not running", stopped, "s1"},
		{"never started", stopped, "never-started"},
		{"no transport", &Service{}, "s1"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := tc.svc.SubmitPrompt(tc.id, "run the tests"); !errors.Is(err, relay.ErrNoMod) {
				t.Fatalf("err = %v, want relay.ErrNoMod", err)
			}
		})
	}
}

// submitInBackground hands a prompt to s1's mod and waits on its receipt off
// the test goroutine.
func submitInBackground(t *testing.T, ctx context.Context, svc *Service) <-chan relay.PromptReceipt {
	t.Helper()
	handed, err := svc.SubmitPrompt("s1", "run the tests")
	if err != nil {
		t.Fatalf("SubmitPrompt: %v", err)
	}
	done := make(chan relay.PromptReceipt, 1)
	go func() { done <- handed.Await(ctx) }()
	return done
}

func receiptOf(t *testing.T, done <-chan relay.PromptReceipt) relay.PromptReceipt {
	t.Helper()
	select {
	case got := <-done:
		return got
	case <-time.After(5 * time.Second):
		t.Fatal("the receipt never came")
		return relay.PromptReceipt{}
	}
}

func TestSubmitPromptReportsTheAck(t *testing.T) {
	for _, want := range []relay.PromptReceipt{
		{State: relay.PromptAcked, OK: true},
		{State: relay.PromptAcked, Reason: "blocked by a hook"},
	} {
		svc := attachedModService(t)
		done := submitInBackground(t, context.Background(), svc)
		cmd := collectMod(t, svc.ws, "s1")
		if cmd.Kind != ModPrompt || cmd.Text != "run the tests" {
			t.Fatalf("the mod was handed %+v, want the prompt", cmd)
		}
		postModAck(t, svc.ws, ackBody("s1", cmd.ID, ModPrompt, want.OK, want.Reason))
		if got := receiptOf(t, done); got != want {
			t.Fatalf("receipt = %+v, want %+v", got, want)
		}
	}
}

func TestSubmitPromptNoPollCollectedIsWithdrawn(t *testing.T) {
	svc := attachedModService(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if got := receiptOf(t, submitInBackground(t, ctx, svc)); got.State != relay.PromptWithdrawn {
		t.Fatalf("receipt = %+v, want withdrawn", got)
	}
	if cmds := pollMod(t, svc.ws, "s1"); len(cmds) != 0 {
		t.Fatalf("poll after the withdrawal = %v, want nothing", cmds)
	}
}

func TestSubmitPromptCollectedThenUnackedIsDelivered(t *testing.T) {
	svc := attachedModService(t)
	ctx, cancel := context.WithCancel(context.Background())
	done := submitInBackground(t, ctx, svc)
	collectMod(t, svc.ws, "s1")
	cancel()
	if got := receiptOf(t, done); got.State != relay.PromptDelivered {
		t.Fatalf("receipt = %+v, want delivered", got)
	}
}

func TestSubmitPromptEndsWithTheSession(t *testing.T) {
	svc := attachedModService(t)
	done := submitInBackground(t, context.Background(), svc)
	collectMod(t, svc.ws, "s1")
	if err := svc.Close("s1"); err != nil {
		t.Fatalf("close: %v", err)
	}
	if got := receiptOf(t, done); got.State != relay.PromptEnded {
		t.Fatalf("receipt = %+v, want ended", got)
	}
}

func TestSubmitPromptIsCollectedOnceAPollCarriesIt(t *testing.T) {
	svc := attachedModService(t)
	handed, err := svc.SubmitPrompt("s1", "run the tests")
	if err != nil {
		t.Fatalf("SubmitPrompt: %v", err)
	}
	if handed.Collected() {
		t.Fatal("collected before any poll")
	}
	collectMod(t, svc.ws, "s1")
	if !handed.Collected() {
		t.Fatal("not collected after a poll carried it")
	}
}

func TestModAttachedOnlyWhileAModPollsFromARunningSession(t *testing.T) {
	unpolled := newModService(t, events.New(), 10*time.Millisecond)
	runModSession(unpolled, "s1")
	stopped := attachedModService(t)
	if err := stopped.Close("s1"); err != nil {
		t.Fatalf("close: %v", err)
	}
	tests := []struct {
		name string
		svc  *Service
		want bool
	}{
		{"polled", attachedModService(t), true},
		{"no poll", unpolled, false},
		{"not running", stopped, false},
		{"no transport", &Service{}, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.svc.ModAttached("s1"); got != tc.want {
				t.Fatalf("ModAttached = %v, want %v", got, tc.want)
			}
		})
	}
}
