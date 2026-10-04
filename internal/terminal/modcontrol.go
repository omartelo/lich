package terminal

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
)

// The command kinds a Claude Code mod applies for lich (docs/hooks/mod-control.md).
const (
	ModPrompt     = "prompt"
	ModAbort      = "abort"
	ModModel      = "model"
	ModEffort     = "effort"
	ModRunCommand = "command"
)

// What RunModCommand learned about a command by the time its wait ended.
const (
	ModWithdrawn = "withdrawn" // no poll collected it, so it was taken back and never runs
	ModDelivered = "delivered" // a poll carried it, and no ack came back yet
	ModAcked     = "acked"     // the mod acked it: OK and Error say how it went
	ModEnded     = "ended"     // the session exited or was closed first
)

const (
	// modUnknownKind is what a mod acks for a kind it does not know (the
	// contract's kind rule), which is how a mod older than modCommandRelease
	// answers a command.
	modUnknownKind = "unknown kind"
	// modCommandRelease is the first lich-plugin release whose mod runs a
	// command.
	modCommandRelease = "0.15.0"
	// modFirstClaudeCode is the first Claude Code that runs mods (the
	// contract's ceilings).
	modFirstClaudeCode = "2.1.280"
)

// defaultWritingCommands maps the slash commands that, run through a mod, save
// their value as the default for every new session (measured on Claude Code
// 2.1.288) to the kind that sets the same thing for this session only.
var defaultWritingCommands = map[string]string{"model": ModModel, "effort": ModEffort}

const (
	// modPollWait bounds how long GET /mod/commands holds an empty poll. The
	// handler bounds itself because http.Serve sets no timeouts, and the bound
	// has to stay under the mod's $.http.fetch timeout, a fixed 30s measured on
	// Claude Code 2.1.288 (see the contract's ceilings).
	modPollWait = 25 * time.Second
	// modRepollGrace is how long a live mod takes to poll again after a
	// response, its 1s error backoff included.
	modRepollGrace = 5 * time.Second
)

var errModNotRunning = errors.New(
	"the session is not running, so there is nothing to hand the command to: start it again, then retry")

var errModDetached = errors.New(
	"the session cannot take commands: no lich-plugin mod is polling from it. That needs Claude Code " +
		modFirstClaudeCode + " or later with mods on, lich-plugin " + modCommandRelease +
		" or later (update it, then restart the session), and the session's folder trusted " +
		"(Claude Code loads the mod only after that)")

// ModCommand is one instruction for a session's mod. Model and Effort absent
// mean "drop the override", which the mod reads as null.
type ModCommand struct {
	ID     string `json:"id"`
	Kind   string `json:"kind"`
	Text   string `json:"text,omitempty"`
	Model  string `json:"model,omitempty"`
	Effort string `json:"effort,omitempty"`
	Name   string `json:"name,omitempty"`
	Args   string `json:"args,omitempty"`
}

// ModOutcome is what became of one command: State is one of ModWithdrawn,
// ModDelivered, ModAcked and ModEnded.
type ModOutcome struct {
	ID    string
	State string
	OK    bool
	Error string
}

func knownModKind(kind string) bool {
	switch kind {
	case ModPrompt, ModAbort, ModModel, ModEffort, ModRunCommand:
		return true
	}
	return false
}

// modQueue holds each session's undelivered commands and the polls parked on
// them. mu is a leaf lock: the reap calls forget while holding Service.mu.
type modQueue struct {
	mu      sync.Mutex
	pending map[string][]queuedModCommand
	waiters map[string][]chan struct{}
	// lastSeen is when each session's mod last polled, which is what tells an
	// attached mod from a session that has none.
	lastSeen map[string]time.Time
	// acks is every queued command whose ack nobody has collected, by id.
	acks map[string]*modWaiter
	next uint64
	wait time.Duration
	now  func() time.Time
}

type queuedModCommand struct {
	ModCommand
	queued time.Time
}

// modWaiter is one command's place to receive its outcome. settled has room for
// the one outcome sent to it, so the sender never blocks.
type modWaiter struct {
	id, session string
	delivered   bool
	settled     chan ModOutcome
}

// take returns the session's queued commands, waiting up to q.wait for one when
// none is queued. A poll whose ctx ended drains nothing, so a client that went
// away does not take commands with it.
func (q *modQueue) take(ctx context.Context, id string) []ModCommand {
	if ctx.Err() != nil {
		return nil
	}
	q.mu.Lock()
	if q.lastSeen == nil {
		q.pending = map[string][]queuedModCommand{}
		q.waiters = map[string][]chan struct{}{}
		q.lastSeen = map[string]time.Time{}
		q.acks = map[string]*modWaiter{}
	}
	q.lastSeen[id] = q.now()
	if cmds := q.drainLocked(id); cmds != nil {
		q.mu.Unlock()
		return cmds
	}
	wake := make(chan struct{}, 1)
	q.waiters[id] = append(q.waiters[id], wake)
	wait := q.wait
	q.mu.Unlock()

	timer := time.NewTimer(wait)
	select {
	case <-wake:
	case <-ctx.Done():
	case <-timer.C:
	}
	timer.Stop()

	q.mu.Lock()
	defer q.mu.Unlock()
	q.unparkLocked(id, wake)
	if ctx.Err() != nil {
		return nil
	}
	return q.drainLocked(id)
}

// drainLocked empties the session's queue. A command older than the attach
// window was queued for a mod that then stopped polling, and the one polling
// now may be hours later: a stale prompt or abort would land on whatever turn
// is running by then.
func (q *modQueue) drainLocked(id string) []ModCommand {
	var cmds []ModCommand
	for _, c := range q.pending[id] {
		if q.now().Sub(c.queued) > q.attachWindow() {
			slog.Warn("mod: dropped a command its mod never collected", "session", id, "id", c.ID, "kind", c.Kind)
			continue
		}
		if w := q.acks[c.ID]; w != nil {
			w.delivered = true
		}
		cmds = append(cmds, c.ModCommand)
	}
	delete(q.pending, id)
	return cmds
}

// attachWindow is how long a mod counts as attached after its last poll: one
// wait plus the re-poll grace.
func (q *modQueue) attachWindow() time.Duration {
	return q.wait + modRepollGrace
}

func (q *modQueue) unparkLocked(id string, wake chan struct{}) {
	rest := q.waiters[id][:0]
	for _, w := range q.waiters[id] {
		if w != wake {
			rest = append(rest, w)
		}
	}
	if len(rest) == 0 {
		delete(q.waiters, id)
		return
	}
	q.waiters[id] = rest
}

// enqueue queues cmd for the session's mod and returns the waiter its ack
// settles. A session whose mod has not polled within the attach window has no
// mod to collect it, so it is refused rather than left to rot.
func (q *modQueue) enqueue(id string, cmd ModCommand) (*modWaiter, error) {
	q.mu.Lock()
	defer q.mu.Unlock()
	seen, ok := q.lastSeen[id]
	now := q.now()
	if !ok || now.Sub(seen) > q.attachWindow() {
		return nil, errModDetached
	}
	q.next++
	cmd.ID = "m" + strconv.FormatUint(q.next, 10)
	q.pending[id] = append(q.pending[id], queuedModCommand{ModCommand: cmd, queued: now})
	w := &modWaiter{id: cmd.ID, session: id, settled: make(chan ModOutcome, 1)}
	q.acks[cmd.ID] = w
	wakeLocked(q.waiters[id])
	return w, nil
}

// settle hands an ack to the wait on its command. An ack for an id nobody is
// waiting on, or from another session than the one the command went to, is
// left alone.
func (q *modQueue) settle(session, id string, ok bool, reason string) {
	q.mu.Lock()
	defer q.mu.Unlock()
	w := q.acks[id]
	if w == nil || w.session != session {
		return
	}
	delete(q.acks, id)
	w.settled <- ModOutcome{ID: id, State: ModAcked, OK: ok, Error: reason}
}

// await waits for w's outcome until ctx ends, and then says how far the command
// got. An ack that lands while the wait is ending still counts. A command no
// poll collected is withdrawn: its caller has been told how it went, and a
// command left queued past the wait would run unannounced, or be dropped
// silently at the next poll once it outlived the attach window.
func (q *modQueue) await(ctx context.Context, w *modWaiter) ModOutcome {
	select {
	case out := <-w.settled:
		return out
	case <-ctx.Done():
	}
	q.mu.Lock()
	defer q.mu.Unlock()
	select {
	case out := <-w.settled:
		return out
	default:
	}
	delete(q.acks, w.id)
	if w.delivered {
		return ModOutcome{ID: w.id, State: ModDelivered}
	}
	q.withdrawLocked(w)
	return ModOutcome{ID: w.id, State: ModWithdrawn}
}

func (q *modQueue) withdrawLocked(w *modWaiter) {
	rest := q.pending[w.session][:0]
	for _, c := range q.pending[w.session] {
		if c.ID != w.id {
			rest = append(rest, c)
		}
	}
	if len(rest) == 0 {
		delete(q.pending, w.session)
		return
	}
	q.pending[w.session] = rest
}

// forget drops a session's queue, releases its parked polls empty-handed and
// ends the waits on its commands.
func (q *modQueue) forget(id string) {
	q.mu.Lock()
	defer q.mu.Unlock()
	delete(q.pending, id)
	delete(q.lastSeen, id)
	wakeLocked(q.waiters[id])
	for cmdID, w := range q.acks {
		if w.session == id {
			delete(q.acks, cmdID)
			w.settled <- ModOutcome{ID: cmdID, State: ModEnded}
		}
	}
}

func wakeLocked(waiters []chan struct{}) {
	for _, w := range waiters {
		select {
		case w <- struct{}{}:
		default:
		}
	}
}

// setModAborted wires the callback a successful abort ack runs. Wired after
// construction like setRestart.
func (t *transport) setModAborted(fn func(id string)) {
	t.mu.Lock()
	t.modAborted = fn
	t.mu.Unlock()
}

// modCommands is the mod's long poll. It is not servePost: it is a GET that
// answers with a body, and it holds the request open.
func (t *transport) modCommands(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if !t.authorized(r) {
		http.Error(w, "invalid token", http.StatusUnauthorized)
		return
	}
	session := r.URL.Query().Get("session_id")
	if session == "" {
		http.Error(w, "session_id is required", http.StatusBadRequest)
		return
	}
	t.plugins.note(session, r.Header.Get(pluginVersionHeader))
	cmds := t.mods.take(r.Context(), session)
	if cmds == nil {
		cmds = []ModCommand{}
	}
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(cmds); err != nil {
		slog.Warn("mod: commands lost on the way to the mod", "session", session, "count", len(cmds), "err", err)
	}
}

// modAckRequest is a mod's report on one command it was handed.
type modAckRequest struct {
	SessionID string `json:"session_id"`
	ID        string `json:"id"`
	Kind      string `json:"kind"`
	OK        bool   `json:"ok"`
	Error     string `json:"error,omitempty"`
}

// modAck receives the mod's ack. Every ack settles a wait on its command; only
// an applied abort is acted on beyond that: Claude Code ends an aborted turn
// without firing Stop, so the ack is the only word lich gets that the turn is
// over.
func (t *transport) modAck(w http.ResponseWriter, r *http.Request) {
	servePost(t, w, r, parseModAck, func(req modAckRequest) error {
		t.mods.settle(req.SessionID, req.ID, req.OK, req.Error)
		if !req.OK {
			slog.Warn("mod: command failed", "session", req.SessionID, "id", req.ID,
				"kind", req.Kind, "error", req.Error)
			return nil
		}
		if req.Kind != ModAbort {
			return nil
		}
		t.mu.Lock()
		fn := t.modAborted
		t.mu.Unlock()
		if fn != nil {
			fn(req.SessionID)
		}
		return nil
	})
}

// parseModAck validates an ack body. The id is not checked: an ack nobody is
// waiting on is accepted like any other.
func parseModAck(body []byte) (modAckRequest, error) {
	var req modAckRequest
	if err := json.Unmarshal(body, &req); err != nil {
		return modAckRequest{}, fmt.Errorf("invalid mod ack body: %w", err)
	}
	if req.SessionID == "" {
		return modAckRequest{}, errors.New("mod ack missing session_id")
	}
	if req.ID == "" {
		return modAckRequest{}, errors.New("mod ack missing id")
	}
	if !knownModKind(req.Kind) {
		return modAckRequest{}, fmt.Errorf("mod ack has unknown kind %q", req.Kind)
	}
	req.Error = clampRunes(strings.TrimSpace(req.Error), hookTextLimit)
	return req, nil
}

// EnqueueModCommand queues cmd for session id's Claude Code mod and returns the
// id the mod's ack will carry (docs/hooks/mod-control.md). Delivery is at most
// once and the ack is the only receipt. It fails when the transport is down,
// when cmd is not a command the contract defines, carries a field of another
// kind or names a slash command lich refuses, with errModNotRunning when the
// session has no process, and with errModDetached when no mod is polling from
// it, which is every running session but a Claude Code one with the
// lich-plugin mod.
func (s *Service) EnqueueModCommand(id string, cmd ModCommand) (string, error) {
	w, err := s.queueModCommand(id, cmd)
	if err != nil {
		return "", err
	}
	return w.id, nil
}

// RunModCommand queues cmd like EnqueueModCommand and waits, until ctx ends,
// for the session's mod to ack it. A wait that ends first is not an error: the
// outcome says the command was delivered, and still goes through, or withdrawn
// before any poll collected it, and never runs.
func (s *Service) RunModCommand(ctx context.Context, id string, cmd ModCommand) (ModOutcome, error) {
	w, err := s.queueModCommand(id, cmd)
	if err != nil {
		return ModOutcome{}, err
	}
	out := s.ws.mods.await(ctx, w)
	if out.State == ModAcked && !out.OK && out.Error == modUnknownKind {
		out.Error = "this session's lich-plugin is too old for it: update lich-plugin to " +
			modCommandRelease + " or later, then restart the session"
	}
	return out, nil
}

func (s *Service) queueModCommand(id string, cmd ModCommand) (*modWaiter, error) {
	if s.ws == nil {
		return nil, fmt.Errorf("control channel unavailable, the transport did not start: %w", s.wsErr)
	}
	if !knownModKind(cmd.Kind) {
		return nil, fmt.Errorf("unknown mod command kind %q", cmd.Kind)
	}
	cmd, err := runnableCommand(cmd)
	if err != nil {
		return nil, err
	}
	if ownModFields(cmd) != cmd {
		return nil, fmt.Errorf("%s command carries a field that belongs to another kind", cmd.Kind)
	}
	if cmd.Kind == ModPrompt && strings.TrimSpace(cmd.Text) == "" {
		return nil, errors.New("prompt command has no text")
	}
	// A poll from a process that is already gone can still land after Close
	// and look like an attached mod for the length of the attach window.
	if !s.Live(id) {
		return nil, errModNotRunning
	}
	return s.ws.mods.enqueue(id, cmd)
}

// runnableCommand is cmd with a slash command's name as the mod passes it on,
// or the reason lich will not queue it. Every other kind passes unchanged. The
// refusal reads the first word with every leading slash gone, so no spelling
// of /model or /effort reaches Claude Code.
func runnableCommand(cmd ModCommand) (ModCommand, error) {
	if cmd.Kind != ModRunCommand {
		return cmd, nil
	}
	cmd.Name = strings.TrimPrefix(strings.TrimSpace(cmd.Name), "/")
	if cmd.Name == "" {
		return ModCommand{}, errors.New("a command needs the name of a slash command, and none was given")
	}
	first := strings.ToLower(strings.TrimLeft(strings.Fields(cmd.Name)[0], "/"))
	if kind, refused := defaultWritingCommands[first]; refused {
		return ModCommand{}, fmt.Errorf(
			"/%s is refused as a command: Claude Code saves what it sets as the default for every "+
				"new session. The %s action changes this session only", first, kind)
	}
	return cmd, nil
}

// ownModFields is cmd with only the fields its kind carries.
func ownModFields(cmd ModCommand) ModCommand {
	own := ModCommand{ID: cmd.ID, Kind: cmd.Kind}
	switch cmd.Kind {
	case ModPrompt:
		own.Text = cmd.Text
	case ModModel:
		own.Model = cmd.Model
	case ModEffort:
		own.Effort = cmd.Effort
	case ModRunCommand:
		own.Name, own.Args = cmd.Name, cmd.Args
	}
	return own
}
