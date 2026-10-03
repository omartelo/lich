package terminal

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/omartelo/lich/internal/events"
)

// newModService is a service with a live transport whose empty polls answer
// after wait instead of modPollWait.
func newModService(t *testing.T, hub *events.Hub, wait time.Duration) *Service {
	t.Helper()
	svc := New(hookStore{}, nil, hub)
	if svc.wsErr != nil {
		t.Fatalf("transport: %v", svc.wsErr)
	}
	svc.ws.mods.mu.Lock()
	svc.ws.mods.wait = wait
	svc.ws.mods.mu.Unlock()
	return svc
}

// modPTY stands in for the Claude Code process a mod runs in. onClose runs
// while Close kills it: the last moment its mod can still poll.
type modPTY struct{ onClose func() }

func (p *modPTY) Write(b []byte) (int, error) { return len(b), nil }
func (p *modPTY) Read([]byte) (int, error)    { return 0, io.EOF }
func (p *modPTY) Resize(int, int) error       { return nil }
func (p *modPTY) Pid() int                    { return 0 }
func (p *modPTY) Wait() (int, error)          { return 0, nil }
func (p *modPTY) Close() error {
	if p.onClose != nil {
		p.onClose()
	}
	return nil
}

// runModSession registers a running session for id, the only kind
// EnqueueModCommand queues for.
func runModSession(svc *Service, id string) *modPTY {
	p := &modPTY{}
	svc.mu.Lock()
	svc.sessions[id] = &session{pty: p, done: make(chan struct{})}
	svc.mu.Unlock()
	return p
}

// attachedMods is how many sessions the queue counts a mod as attached for.
func attachedMods(tr *transport) int {
	tr.mods.mu.Lock()
	defer tr.mods.mu.Unlock()
	return len(tr.mods.lastSeen) + len(tr.mods.pending)
}

func modCommandsURL(tr *transport, session string) string {
	return fmt.Sprintf("http://127.0.0.1:%d/mod/commands?token=%s&session_id=%s", tr.port, tr.token, session)
}

// fetchMod is one GET /mod/commands the way the mod sends it. It is safe off
// the test goroutine; pollMod is its asserting form.
func fetchMod(ctx context.Context, tr *transport, session string) ([]ModCommand, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, modCommandsURL(tr, session), nil)
	if err != nil {
		return nil, err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("status = %d, want 200", resp.StatusCode)
	}
	if ct := resp.Header.Get("Content-Type"); ct != "application/json" {
		return nil, fmt.Errorf("Content-Type = %q, want application/json", ct)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	var cmds []ModCommand
	if err := json.Unmarshal(body, &cmds); err != nil {
		return nil, fmt.Errorf("body %q: %w", body, err)
	}
	if cmds == nil {
		return nil, fmt.Errorf("body %q is not an array", body)
	}
	return cmds, nil
}

func pollMod(t *testing.T, tr *transport, session string) []ModCommand {
	t.Helper()
	cmds, err := fetchMod(context.Background(), tr, session)
	if err != nil {
		t.Fatalf("poll: %v", err)
	}
	return cmds
}

// pollInBackground parks a poll off the test goroutine and hands back its
// answer.
func pollInBackground(t *testing.T, tr *transport, session string) <-chan []ModCommand {
	t.Helper()
	got := make(chan []ModCommand, 1)
	go func() {
		cmds, err := fetchMod(context.Background(), tr, session)
		if err != nil {
			t.Errorf("background poll: %v", err)
		}
		got <- cmds
	}()
	waitFor(t, func() bool { return parkedPolls(tr, session) == 1 }, "the poll to park")
	return got
}

// parkedPolls is how many polls for session are waiting on its queue. The
// cancel test needs it: whether the server has seen the client leave is not
// observable from the client.
func parkedPolls(tr *transport, session string) int {
	tr.mods.mu.Lock()
	defer tr.mods.mu.Unlock()
	return len(tr.mods.waiters[session])
}

func postModAck(t *testing.T, tr *transport, body string) {
	t.Helper()
	url := fmt.Sprintf("http://127.0.0.1:%d/mod/acks?token=%s", tr.port, tr.token)
	resp, err := http.Post(url, "application/json", strings.NewReader(body))
	if err != nil {
		t.Fatalf("post ack: %v", err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("ack status = %d, want 204", resp.StatusCode)
	}
}

func TestModCommandsRefusesBadRequests(t *testing.T) {
	tr := newNilTransport(t)
	tests := []struct {
		name   string
		method string
		url    string
		want   int
	}{
		{"not a GET", http.MethodPost, modCommandsURL(tr, "s1"), http.StatusMethodNotAllowed},
		{"bad token", http.MethodGet,
			fmt.Sprintf("http://127.0.0.1:%d/mod/commands?token=wrong&session_id=s1", tr.port), http.StatusUnauthorized},
		{"missing session_id", http.MethodGet,
			fmt.Sprintf("http://127.0.0.1:%d/mod/commands?token=%s", tr.port, tr.token), http.StatusBadRequest},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req, err := http.NewRequest(tc.method, tc.url, nil)
			if err != nil {
				t.Fatalf("new request: %v", err)
			}
			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				t.Fatalf("do: %v", err)
			}
			_ = resp.Body.Close()
			if resp.StatusCode != tc.want {
				t.Fatalf("status = %d, want %d", resp.StatusCode, tc.want)
			}
		})
	}
}

// A poll that times out is what attaches the mod, and a command queued after it
// is handed out exactly once.
func TestModCommandIsDeliveredOnce(t *testing.T) {
	svc := newModService(t, events.New(), 10*time.Millisecond)
	runModSession(svc, "s1")
	if got := pollMod(t, svc.ws, "s1"); len(got) != 0 {
		t.Fatalf("first poll = %v, want []", got)
	}
	id, err := svc.EnqueueModCommand("s1", ModCommand{Kind: ModModel, Model: "opus"})
	if err != nil {
		t.Fatalf("enqueue: %v", err)
	}
	want := []ModCommand{{ID: id, Kind: ModModel, Model: "opus"}}
	if got := pollMod(t, svc.ws, "s1"); !reflect.DeepEqual(got, want) {
		t.Fatalf("poll = %v, want %v", got, want)
	}
	if got := pollMod(t, svc.ws, "s1"); len(got) != 0 {
		t.Fatalf("the command was delivered twice: %v", got)
	}
}

// Commands queued between two polls come back in one response, in order, and
// each gets an id of its own.
func TestModCommandsArriveInOrder(t *testing.T) {
	svc := newModService(t, events.New(), 10*time.Millisecond)
	runModSession(svc, "s1")
	pollMod(t, svc.ws, "s1")
	first, err := svc.EnqueueModCommand("s1", ModCommand{Kind: ModPrompt, Text: "go"})
	if err != nil {
		t.Fatalf("enqueue: %v", err)
	}
	second, err := svc.EnqueueModCommand("s1", ModCommand{Kind: ModAbort})
	if err != nil {
		t.Fatalf("enqueue: %v", err)
	}
	if first == second {
		t.Fatalf("two commands share the id %q", first)
	}
	got := pollMod(t, svc.ws, "s1")
	want := []ModCommand{{ID: first, Kind: ModPrompt, Text: "go"}, {ID: second, Kind: ModAbort}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("poll = %v, want %v", got, want)
	}
}

// A parked poll answers as soon as a command is queued, not when its wait runs
// out.
func TestModParkedPollWakesOnEnqueue(t *testing.T) {
	svc := newModService(t, events.New(), time.Minute)
	runModSession(svc, "s1")
	got := pollInBackground(t, svc.ws, "s1")

	id, err := svc.EnqueueModCommand("s1", ModCommand{Kind: ModAbort})
	if err != nil {
		t.Fatalf("enqueue: %v", err)
	}
	select {
	case cmds := <-got:
		if want := []ModCommand{{ID: id, Kind: ModAbort}}; !reflect.DeepEqual(cmds, want) {
			t.Fatalf("poll = %v, want %v", cmds, want)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("the parked poll was never woken")
	}
}

// A client that left takes nothing with it: the command it would have been
// handed waits for the next poll.
func TestModCancelledPollLeavesTheQueue(t *testing.T) {
	svc := newModService(t, events.New(), time.Minute)
	runModSession(svc, "s1")
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		defer close(done)
		if _, err := fetchMod(ctx, svc.ws, "s1"); !errors.Is(err, context.Canceled) {
			t.Errorf("cancelled poll: err = %v, want context.Canceled", err)
		}
	}()
	waitFor(t, func() bool { return parkedPolls(svc.ws, "s1") == 1 }, "the poll to park")
	cancel()
	<-done
	waitFor(t, func() bool { return parkedPolls(svc.ws, "s1") == 0 }, "the server to see the client leave")

	id, err := svc.EnqueueModCommand("s1", ModCommand{Kind: ModCompact})
	if err != nil {
		t.Fatalf("enqueue: %v", err)
	}
	want := []ModCommand{{ID: id, Kind: ModCompact}}
	if got := pollMod(t, svc.ws, "s1"); !reflect.DeepEqual(got, want) {
		t.Fatalf("poll = %v, want %v", got, want)
	}
}

// Closing the session releases its parked poll empty-handed and detaches it.
func TestModCloseReleasesTheParkedPoll(t *testing.T) {
	svc := newModService(t, events.New(), time.Minute)
	runModSession(svc, "s1")
	got := pollInBackground(t, svc.ws, "s1")

	if err := svc.Close("s1"); err != nil {
		t.Fatalf("close: %v", err)
	}
	select {
	case cmds := <-got:
		if len(cmds) != 0 {
			t.Fatalf("released poll = %v, want []", cmds)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("closing the session left its poll parked")
	}
	if _, err := svc.EnqueueModCommand("s1", ModCommand{Kind: ModAbort}); !errors.Is(err, errModDetached) {
		t.Fatalf("enqueue after close: err = %v, want errModDetached", err)
	}
}

func TestEnqueueModCommandRefuses(t *testing.T) {
	svc := newModService(t, events.New(), 10*time.Millisecond)
	runModSession(svc, "attached")
	pollMod(t, svc.ws, "attached")
	tests := []struct {
		name    string
		session string
		cmd     ModCommand
		detach  bool
	}{
		{"a session that never polled", "never", ModCommand{Kind: ModAbort}, true},
		{"an unknown kind", "attached", ModCommand{Kind: "type"}, false},
		{"an empty prompt", "attached", ModCommand{Kind: ModPrompt, Text: "  "}, false},
		{"text on an abort", "attached", ModCommand{Kind: ModAbort, Text: "x"}, false},
		{"a model on a prompt", "attached", ModCommand{Kind: ModPrompt, Text: "go", Model: "m"}, false},
		{"an effort on a model", "attached", ModCommand{Kind: ModModel, Effort: "high"}, false},
		{"instructions on an effort", "attached", ModCommand{Kind: ModEffort, Instructions: "x"}, false},
		{"text on a compact", "attached", ModCommand{Kind: ModCompact, Text: "x"}, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := svc.EnqueueModCommand(tc.session, tc.cmd)
			if err == nil {
				t.Fatal("enqueue accepted it")
			}
			if errors.Is(err, errModDetached) != tc.detach {
				t.Fatalf("err = %v, detached want %v", err, tc.detach)
			}
		})
	}
}

// A mod that stopped polling (disabled mid-session, or its process gone without
// lich hearing) is detached once its re-poll grace has run out.
func TestEnqueueModCommandRefusesAStaleMod(t *testing.T) {
	svc := newModService(t, events.New(), 10*time.Millisecond)
	runModSession(svc, "s1")
	pollMod(t, svc.ws, "s1")
	later := time.Now().Add(time.Minute)
	svc.ws.mods.mu.Lock()
	svc.ws.mods.now = func() time.Time { return later }
	svc.ws.mods.mu.Unlock()
	if _, err := svc.EnqueueModCommand("s1", ModCommand{Kind: ModAbort}); !errors.Is(err, errModDetached) {
		t.Fatalf("err = %v, want errModDetached", err)
	}
}

// A command the mod never collected before it went quiet is not handed to the
// mod that polls again later: by then it would land on an unrelated turn.
func TestModCommandOutlivingTheAttachWindowIsDropped(t *testing.T) {
	svc := newModService(t, events.New(), 10*time.Millisecond)
	runModSession(svc, "s1")
	pollMod(t, svc.ws, "s1")
	if _, err := svc.EnqueueModCommand("s1", ModCommand{Kind: ModPrompt, Text: "go"}); err != nil {
		t.Fatalf("enqueue: %v", err)
	}
	later := time.Now().Add(time.Hour)
	svc.ws.mods.mu.Lock()
	svc.ws.mods.now = func() time.Time { return later }
	svc.ws.mods.mu.Unlock()
	if got := pollMod(t, svc.ws, "s1"); len(got) != 0 {
		t.Fatalf("a poll an hour later = %v, want []", got)
	}
}

// A poll whose client is already gone when it reaches the queue drains nothing:
// the command stays for the next poll instead of vanishing unwritten.
func TestModPollFromAGoneClientDrainsNothing(t *testing.T) {
	svc := newModService(t, events.New(), 10*time.Millisecond)
	runModSession(svc, "s1")
	pollMod(t, svc.ws, "s1")
	id, err := svc.EnqueueModCommand("s1", ModCommand{Kind: ModAbort})
	if err != nil {
		t.Fatalf("enqueue: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	req := httptest.NewRequestWithContext(ctx, http.MethodGet, modCommandsURL(svc.ws, "s1"), nil)
	svc.ws.modCommands(httptest.NewRecorder(), req)

	want := []ModCommand{{ID: id, Kind: ModAbort}}
	if got := pollMod(t, svc.ws, "s1"); !reflect.DeepEqual(got, want) {
		t.Fatalf("poll = %v, want %v", got, want)
	}
}

// The mod lives until Close kills its process, and the contract has it re-poll
// at once. That last poll must not leave the closed session attached.
func TestModPollWhileClosingDoesNotReattach(t *testing.T) {
	svc := newModService(t, events.New(), 10*time.Millisecond)
	p := runModSession(svc, "s1")
	pollMod(t, svc.ws, "s1")
	p.onClose = func() { pollMod(t, svc.ws, "s1") }

	if err := svc.Close("s1"); err != nil {
		t.Fatalf("close: %v", err)
	}
	if _, err := svc.EnqueueModCommand("s1", ModCommand{Kind: ModAbort}); !errors.Is(err, errModDetached) {
		t.Fatalf("enqueue after close: err = %v, want errModDetached", err)
	}
	if n := attachedMods(svc.ws); n != 0 {
		t.Fatalf("the closed session left %d queue entries behind", n)
	}
}

// A poll already on the wire when the session closed still lands. It must not
// make the closed session take commands nobody will collect.
func TestModPollLandingAfterCloseTakesNoCommands(t *testing.T) {
	svc := newModService(t, events.New(), 10*time.Millisecond)
	runModSession(svc, "s1")
	pollMod(t, svc.ws, "s1")
	if err := svc.Close("s1"); err != nil {
		t.Fatalf("close: %v", err)
	}
	pollMod(t, svc.ws, "s1")
	if _, err := svc.EnqueueModCommand("s1", ModCommand{Kind: ModAbort}); !errors.Is(err, errModDetached) {
		t.Fatalf("enqueue after close: err = %v, want errModDetached", err)
	}
}

func TestEnqueueModCommandWithoutTransport(t *testing.T) {
	svc := &Service{wsErr: errors.New("listen failed")}
	if _, err := svc.EnqueueModCommand("s1", ModCommand{Kind: ModAbort}); err == nil {
		t.Fatal("enqueue succeeded with no transport")
	}
}

// Only an applied abort ends a turn: a failure ends nothing, and no other kind
// is lich's to act on.
func TestModAckCallsTheAbortSinkOnlyForAnAppliedAbort(t *testing.T) {
	tests := []struct {
		name string
		body string
		want int64
	}{
		{"applied abort", `{"session_id":"s1","id":"m1","kind":"abort","ok":true}`, 1},
		{"failed abort", `{"session_id":"s1","id":"m1","kind":"abort","ok":false,"error":"no turn"}`, 0},
		{"applied prompt", `{"session_id":"s1","id":"m1","kind":"prompt","ok":true}`, 0},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tr := newNilTransport(t)
			var calls atomic.Int64
			tr.setModAborted(func(id string) {
				if id == "s1" {
					calls.Add(1)
				}
			})
			postModAck(t, tr, tc.body)
			if got := calls.Load(); got != tc.want {
				t.Fatalf("abort sink called %d times, want %d", got, tc.want)
			}
		})
	}
}

// The end of the wire for the gap this contract closes: Claude Code ends an
// aborted turn without Stop, so the ack is what tells the window it is over.
func TestModAbortAckEndsTheOpenTurn(t *testing.T) {
	hub, rec := newProbeHub(t)
	svc := newModService(t, hub, modPollWait)
	postHook(t, svc, "s1", statusBusy)
	postModAck(t, svc.ws, `{"session_id":"s1","id":"m1","kind":"abort","ok":true}`)
	postModAck(t, svc.ws, `{"session_id":"quiet","id":"m2","kind":"abort","ok":true}`)

	hub.Emit(probeReadyEvent, nil)
	waitFor(t, func() bool { return slices.Contains(rec.snapshot(), probeReadyEvent) },
		"the window to be told about the aborted turn")
	want := []string{statusBusy, statusInterrupted}
	if got := rec.statesOf("s1"); strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("the window was told %v, want %v", got, want)
	}
	if got := rec.statesOf("quiet"); len(got) != 0 {
		t.Fatalf("an abort with no open turn told the window %v", got)
	}
}

// TestModCommandsMatchFixture pins the lich-to-mod direction: the mod reads
// docs/hooks/fixtures/mod-commands.json, so every shape lich sends must encode
// to exactly what that file says.
func TestModCommandsMatchFixture(t *testing.T) {
	cmds := []ModCommand{
		{ID: "m1", Kind: ModPrompt, Text: "run the tests"},
		{ID: "m2", Kind: ModAbort},
		{ID: "m3", Kind: ModModel, Model: "claude-opus-4-1"},
		{ID: "m4", Kind: ModModel},
		{ID: "m5", Kind: ModEffort, Effort: "high"},
		{ID: "m6", Kind: ModEffort},
		{ID: "m7", Kind: ModCompact, Instructions: "keep the test plan"},
		{ID: "m8", Kind: ModCompact},
	}
	encoded, err := json.Marshal(cmds)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	golden, err := os.ReadFile(filepath.Join(hookFixtureDir, "mod-commands.json"))
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	var got, want []map[string]any
	if err := json.Unmarshal(encoded, &got); err != nil {
		t.Fatalf("decode encoded: %v", err)
	}
	if err := json.Unmarshal(golden, &want); err != nil {
		t.Fatalf("decode fixture: %v", err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("lich sends %v, the fixture says %v", got, want)
	}
}
