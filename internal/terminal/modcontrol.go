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
	ModPrompt  = "prompt"
	ModAbort   = "abort"
	ModModel   = "model"
	ModEffort  = "effort"
	ModCompact = "compact"
)

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

var errModDetached = errors.New(
	"control channel not connected: the lich-plugin mod is not running in this session")

// ModCommand is one instruction for a session's mod. Model and Effort absent
// mean "drop the override", which the mod reads as null.
type ModCommand struct {
	ID           string `json:"id"`
	Kind         string `json:"kind"`
	Text         string `json:"text,omitempty"`
	Model        string `json:"model,omitempty"`
	Effort       string `json:"effort,omitempty"`
	Instructions string `json:"instructions,omitempty"`
}

func knownModKind(kind string) bool {
	switch kind {
	case ModPrompt, ModAbort, ModModel, ModEffort, ModCompact:
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
	next     uint64
	wait     time.Duration
	now      func() time.Time
}

type queuedModCommand struct {
	ModCommand
	queued time.Time
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

// enqueue queues cmd for the session's mod and returns the id its ack will
// carry. A session whose mod has not polled within the attach window has no
// mod to collect it, so it is refused rather than left to rot.
func (q *modQueue) enqueue(id string, cmd ModCommand) (string, error) {
	q.mu.Lock()
	defer q.mu.Unlock()
	seen, ok := q.lastSeen[id]
	now := q.now()
	if !ok || now.Sub(seen) > q.attachWindow() {
		return "", errModDetached
	}
	q.next++
	cmd.ID = "m" + strconv.FormatUint(q.next, 10)
	q.pending[id] = append(q.pending[id], queuedModCommand{ModCommand: cmd, queued: now})
	wakeLocked(q.waiters[id])
	return cmd.ID, nil
}

// forget drops a session's queue and releases its parked polls empty-handed.
func (q *modQueue) forget(id string) {
	q.mu.Lock()
	defer q.mu.Unlock()
	delete(q.pending, id)
	delete(q.lastSeen, id)
	wakeLocked(q.waiters[id])
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

// modAck receives the mod's ack. Only an applied abort is acted on: Claude Code
// ends an aborted turn without firing Stop, so the ack is the only word lich
// gets that the turn is over.
func (t *transport) modAck(w http.ResponseWriter, r *http.Request) {
	servePost(t, w, r, parseModAck, func(req modAckRequest) error {
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

// parseModAck validates an ack body. The id is not checked against anything:
// lich keeps no table of what it handed out.
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
// when cmd is not a command the contract defines or carries a field of another
// kind, and with errModDetached when the session has no process or no mod
// polling, which is every session but a running Claude Code one with the
// lich-plugin mod.
func (s *Service) EnqueueModCommand(id string, cmd ModCommand) (string, error) {
	if s.ws == nil {
		return "", fmt.Errorf("control channel unavailable, the transport did not start: %w", s.wsErr)
	}
	if !knownModKind(cmd.Kind) {
		return "", fmt.Errorf("unknown mod command kind %q", cmd.Kind)
	}
	if ownModFields(cmd) != cmd {
		return "", fmt.Errorf("%s command carries a field that belongs to another kind", cmd.Kind)
	}
	if cmd.Kind == ModPrompt && strings.TrimSpace(cmd.Text) == "" {
		return "", errors.New("prompt command has no text")
	}
	// A poll from a process that is already gone can still land after Close
	// and look like an attached mod for the length of the attach window.
	if !s.Live(id) {
		return "", errModDetached
	}
	return s.ws.mods.enqueue(id, cmd)
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
	case ModCompact:
		own.Instructions = cmd.Instructions
	}
	return own
}
