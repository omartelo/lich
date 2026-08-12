package terminal

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"os"
	"runtime/pprof"
	"strings"
	"sync"
	"time"

	"github.com/coder/websocket"
	"github.com/omartelo/lich/internal/providers"
	"github.com/omartelo/lich/internal/restart"
)

// Terminal I/O rides one local WebSocket carrying binary frames, instead of
// one RPC POST per keystroke and one JSON event per output chunk — per-message
// overhead at interactive rates (~60 crossings/s while typing) is the hot
// path this avoids. When no client is connected (or a write fails) output
// falls back to the /events channel and input to the RPC, so the app works
// degraded but unbroken.
//
// Frame format, both directions: [1 byte id length][session id][payload].

const (
	// wsWriteTimeout bounds a send to the local client; a stalled write drops
	// the connection so output falls back to the /events channel.
	wsWriteTimeout = 5 * time.Second
	// wsReadLimit bounds one input frame; keystrokes and pastes are far
	// smaller, and the frontend chunks nothing above this.
	wsReadLimit = 1 << 20
	// tokenBytes is the size of the random connect token.
	tokenBytes = 16
	// hookBodyLimit bounds a status POST from the Claude Code hook; the payload
	// is a tiny JSON object, so anything larger is malformed or hostile.
	hookBodyLimit = 4 << 10
	// These are the only session states the hook may report: busy while Claude
	// is producing output, done when its turn finishes, waiting when Claude is
	// blocked on the user (permission prompt or idle input, from Notification),
	// and idle when the session ends (from SessionEnd) — idle clears the card's
	// indicator so a stale spinner/check does not linger past a session or a
	// /clear.
	statusBusy    = "busy"
	statusDone    = "done"
	statusWaiting = "waiting"
	statusIdle    = "idle"
)

// encodeFrame prefixes payload with the session id. The id must fit one byte
// of length.
func encodeFrame(id string, payload []byte) ([]byte, error) {
	if len(id) == 0 || len(id) > 255 {
		return nil, fmt.Errorf("session id length %d out of range", len(id))
	}
	buf := make([]byte, 1+len(id)+len(payload))
	buf[0] = byte(len(id))
	copy(buf[1:], id)
	copy(buf[1+len(id):], payload)
	return buf, nil
}

// decodeFrame splits a frame into session id and payload. The payload aliases
// buf; callers must not retain it past the next read.
func decodeFrame(buf []byte) (string, []byte, error) {
	if len(buf) < 1 {
		return "", nil, errors.New("empty frame")
	}
	n := int(buf[0])
	if n == 0 || len(buf) < 1+n {
		return "", nil, fmt.Errorf("frame id length %d exceeds frame size %d", n, len(buf))
	}
	return string(buf[1 : 1+n]), buf[1+n:], nil
}

// transport is the local WebSocket endpoint. One client (the webview) is
// expected; a new connection replaces the previous one.
type transport struct {
	mu          sync.Mutex
	conn        *websocket.Conn
	port        int
	token       string
	mux         *http.ServeMux
	input       func(id string, data []byte)
	status      func(req hookRequest)
	linkSession func(sessionID, providerSessionID, provider string) error
	setTitle    func(sessionID, title string) error
	touched     func(sessionID string)
	// restart, when set, relaunches lich and closes this window; POST /restart
	// triggers it (install.sh calls it after replacing the binary on disk).
	// Guarded by mu: set via setRestart after the server goroutine is already
	// running, and read by the request goroutine handling /restart.
	restart func() error
}

// newTransport starts the listener on a random loopback port. input receives
// decoded input frames (keyboard data for a session's PTY); status receives a
// session's processing state, and the tool it is about to run, as reported by
// the provider's hooks (see /hook); linkSession records the provider session id
// a PTY reports at start (see /session-start); setTitle applies an
// auto-generated session label (see /session-title); touched signals a session
// likely changed files on disk (see /session-touched).
func newTransport(
	input func(id string, data []byte),
	status func(req hookRequest),
	linkSession func(sessionID, providerSessionID, provider string) error,
	setTitle func(sessionID, title string) error,
	touched func(sessionID string),
) (*transport, error) {
	raw := make([]byte, tokenBytes)
	if _, err := rand.Read(raw); err != nil {
		return nil, fmt.Errorf("failed to generate transport token: %w", err)
	}
	// The port is random by default; LICH_LISTEN_PORT pins it. The Chromium
	// shell needs a stable port (main.go defaults it there): the page's origin
	// is host:port, and the frontend's localStorage (lich.* settings) only
	// survives restarts when the origin does. Distinct from LICH_PORT, which
	// is the per-session hook-contract variable pointing at THIS listener —
	// reusing it would make a lich launched from inside a lich session try to
	// bind its parent's port.
	addr := "127.0.0.1:0"
	if port := os.Getenv("LICH_LISTEN_PORT"); port != "" {
		addr = "127.0.0.1:" + port
	}
	listener, err := listen(addr)
	if err != nil {
		return nil, fmt.Errorf("failed to listen for transport on %s: %w", addr, err)
	}
	t := &transport{
		port:        listener.Addr().(*net.TCPAddr).Port,
		token:       hex.EncodeToString(raw),
		input:       input,
		status:      status,
		linkSession: linkSession,
		setTitle:    setTitle,
		touched:     touched,
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/ws", t.handle)
	mux.HandleFunc("/ping", t.ping)
	mux.HandleFunc("/debug/goroutines", t.goroutines)
	mux.HandleFunc("/hook", t.hook)
	mux.HandleFunc("/session-start", t.sessionStart)
	mux.HandleFunc("/session-title", t.sessionTitle)
	mux.HandleFunc("/session-touched", t.sessionTouched)
	mux.HandleFunc("/restart", t.restartApp)
	t.mux = mux
	// Server and listener live for the process lifetime, like the PTY sessions
	// they serve; add Shutdown if the app ever needs teardown. Serve returning
	// is therefore always abnormal — the frontend just lost RPC and terminals.
	go func() {
		if err := http.Serve(listener, mux); err != nil {
			slog.Error("transport listener stopped", "err", err)
		}
	}()
	return t, nil
}

// bindTimeout / restartBindTimeout / bindRetryInterval bound the wait any
// launch spends retrying the pinned port while a just-exited lich still holds
// it: closing the window ends the process, but on Windows its child processes
// (Chromium, the PTYs) can take a moment longer to release the port than the
// parent takes to exit, so even a plain relaunch can lose that race. A
// restarting lich (restart.WaitEnv set) gets the longer bound: install.sh
// hands off immediately after spawning it, before the predecessor it is
// replacing has even been told to close its window.
const (
	bindTimeout        = 2 * time.Second
	restartBindTimeout = 10 * time.Second
	bindRetryInterval  = 200 * time.Millisecond
)

// listen binds addr, retrying while the port stays taken. A retry that pays
// off is logged too: it never surfaces as a failure, so the log is the only
// trace this race is happening at all — the diagnosis a prior bind failure on
// this port needed and did not have (see CHANGELOG).
func listen(addr string) (net.Listener, error) {
	timeout := bindTimeout
	if os.Getenv(restart.WaitEnv) != "" {
		timeout = restartBindTimeout
	}
	start := time.Now()
	deadline := start.Add(timeout)
	attempts := 0
	for {
		attempts++
		l, err := net.Listen("tcp", addr)
		if err == nil {
			if attempts > 1 {
				slog.Info("bind succeeded after retry",
					"addr", addr, "attempts", attempts, "waited", time.Since(start).Round(time.Millisecond))
			}
			return l, nil
		}
		if time.Now().After(deadline) {
			return nil, fmt.Errorf("after %d attempts over %s: %w", attempts, time.Since(start).Round(time.Millisecond), err)
		}
		time.Sleep(bindRetryInterval)
	}
}

// restartApp triggers an in-place relaunch: it authenticates and answers before
// tearing down, since the restart closes this very window and process. The
// caller (install.sh) ignores the body — the status code is for logs and tests.
func (t *transport) restartApp(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if !t.authorized(r) {
		http.Error(w, "invalid token", http.StatusUnauthorized)
		return
	}
	t.mu.Lock()
	fn := t.restart
	t.mu.Unlock()
	if fn == nil {
		http.Error(w, "restart unavailable", http.StatusServiceUnavailable)
		return
	}
	w.WriteHeader(http.StatusNoContent)
	if f, ok := w.(http.Flusher); ok {
		f.Flush()
	}
	// Run after the response flushes: the successor is spawned and this window
	// is closed, which unwinds the process. A failure only reaches the log.
	go func() {
		if err := fn(); err != nil {
			slog.Error("restart failed", "err", err)
		}
	}()
}

// setRestart records the relaunch callback under the same lock that guards the
// transport's other mutable state (conn). The service's SetRestart calls it, and
// it runs after the server goroutine is already serving, so the lock is what
// establishes the happens-before with the /restart handler's read.
func (t *transport) setRestart(fn func() error) {
	t.mu.Lock()
	t.restart = fn
	t.mu.Unlock()
}

// ping is the liveness probe a second lich launch uses to tell "a live lich
// already holds my pinned port" from "the port is taken by something else": only
// lich serves this behind the token, so a 204 proves the recorded instance is
// alive and is lich (see internal/singleton). Token-gated like every endpoint
// but /'s static assets.
func (t *transport) ping(w http.ResponseWriter, r *http.Request) {
	if !t.authorized(r) {
		http.Error(w, "invalid token", http.StatusUnauthorized)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// goroutines dumps every goroutine's stack, blocked ones included. It answers
// the failure a log cannot describe: the window stops updating while the
// process is still alive, so nothing panicked, nothing exited, and nothing was
// ever written. `lich rage` fetches this into the bug report; an instance that
// holds its port and will not answer here is itself the finding. Token-gated
// like every other endpoint, and deliberately not net/http/pprof: this is one
// dump of one thing, not a profiling surface.
func (t *transport) goroutines(w http.ResponseWriter, r *http.Request) {
	if !t.authorized(r) {
		http.Error(w, "invalid token", http.StatusUnauthorized)
		return
	}
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	// debug level 2 is the readable form — the same layout a panic prints, so a
	// hang and a crash are read side by side.
	if err := pprof.Lookup("goroutine").WriteTo(w, 2); err != nil {
		slog.Warn("goroutine dump", "err", err)
	}
}

// mount adds a handler to the transport listener behind the same token check
// every endpoint uses. ServeMux registration is safe after Serve started, so
// callers wire extra surfaces (RPC, the events push socket) post-construction.
func (t *transport) mount(pattern string, handler http.Handler) {
	t.mux.Handle(pattern, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !t.authorized(r) {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		handler.ServeHTTP(w, r)
	}))
}

// mountPublic adds a handler without the token check — for the static
// frontend assets the Chromium shell loads before it knows the token (which
// rides the page URL). Loopback-only like everything else; the RPC, terminal
// and event surfaces stay token-gated.
func (t *transport) mountPublic(pattern string, handler http.Handler) {
	t.mux.Handle(pattern, handler)
}

// authorized reports whether the request carries the transport's connect token.
// The origin is not checked — in dev the page comes from the Vite server, not
// this listener — the random token is the auth.
func (t *transport) authorized(r *http.Request) bool {
	provided := r.URL.Query().Get("token")
	return subtle.ConstantTimeCompare([]byte(provided), []byte(t.token)) == 1
}

// servePost is the skeleton every hook endpoint shares: POST only, token
// authenticated, body bounded to hookBodyLimit, parsed, applied, 204. A parse
// failure is the caller's fault (400); an apply failure is ours (500). The hook
// client ignores the response either way — the status codes exist so a failure
// is visible to logs and tests rather than silently swallowed. It is a function
// rather than a method on transport because Go methods cannot be generic.
func servePost[T any](
	t *transport,
	w http.ResponseWriter,
	r *http.Request,
	parse func([]byte) (T, error),
	apply func(T) error,
) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if !t.authorized(r) {
		http.Error(w, "invalid token", http.StatusUnauthorized)
		return
	}
	body, err := io.ReadAll(io.LimitReader(r.Body, hookBodyLimit))
	if err != nil {
		http.Error(w, "failed to read body", http.StatusBadRequest)
		return
	}
	parsed, err := parse(body)
	if err != nil {
		// The hook client ignores responses, so the log is the only place a
		// bad payload ever surfaces.
		slog.Warn("hook: bad payload", "path", r.URL.Path, "err", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if err := apply(parsed); err != nil {
		slog.Warn("hook: apply failed", "path", r.URL.Path, "err", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// hookRequest is a status POST body. Tool and Detail are the optional pair a
// pre-tool report carries — the provider's own name for the tool it is about to
// run, and its own words for what that tool acts on. Both are absent from every
// other report.
type hookRequest struct {
	SessionID string `json:"session_id"`
	State     string `json:"state"`
	Tool      string `json:"tool"`
	Detail    string `json:"detail"`
}

// hookTextLimit bounds the tool name and detail a report may carry, in runes.
// Over-long values are truncated rather than rejected: they are decoration on a
// state that has to land either way, and losing the spinner because a command
// line grew is the worse failure.
const hookTextLimit = 120

// hook receives a session status POST from a provider's hook running inside a
// spawned PTY and forwards it to the frontend via the status callback.
func (t *transport) hook(w http.ResponseWriter, r *http.Request) {
	servePost(t, w, r, parseHookRequest, func(req hookRequest) error {
		if t.status != nil {
			t.status(req)
		}
		return nil
	})
}

// parseHookRequest validates a status POST body: a session id, one of the known
// states, and the optional tool pair. It never trusts the payload — an unknown
// state is rejected so a stray or hostile POST can't drive the UI into an
// undefined status, and the free text is trimmed and capped (see hookTextLimit).
func parseHookRequest(body []byte) (hookRequest, error) {
	var req hookRequest
	if err := json.Unmarshal(body, &req); err != nil {
		return hookRequest{}, fmt.Errorf("invalid hook body: %w", err)
	}
	if req.SessionID == "" {
		return hookRequest{}, errors.New("hook missing session_id")
	}
	if req.State != statusBusy && req.State != statusDone &&
		req.State != statusWaiting && req.State != statusIdle {
		return hookRequest{}, fmt.Errorf("hook has unknown state %q", req.State)
	}
	req.Tool = clampRunes(strings.TrimSpace(req.Tool), hookTextLimit)
	req.Detail = clampRunes(strings.TrimSpace(req.Detail), hookTextLimit)
	// A detail with no tool to qualify names nothing, so it is dropped rather
	// than shown on its own.
	if req.Tool == "" {
		req.Detail = ""
	}
	return req, nil
}

// clampRunes truncates s to at most max runes, counting runes rather than bytes
// so a cut never lands mid-character.
func clampRunes(s string, max int) string {
	runes := []rune(s)
	if len(runes) <= max {
		return s
	}
	return string(runes[:max])
}

// startRequest is a session-start POST body. LegacyClaudeSessionID accepts the
// pre-multi-provider field name plugin releases before v0.3.0 send; parsing
// folds it into ProviderSessionID, so nothing downstream sees two names. Drop it
// once the install gate can no longer meet an older plugin.
type startRequest struct {
	SessionID             string `json:"session_id"`
	ProviderSessionID     string `json:"provider_session_id"`
	Provider              string `json:"provider"`
	LegacyClaudeSessionID string `json:"claude_session_id"`
}

// sessionStart receives the session-start POST from a provider's hook running
// inside a spawned PTY and records the provider session id against the lich
// session via the linkSession callback.
func (t *transport) sessionStart(w http.ResponseWriter, r *http.Request) {
	servePost(t, w, r, parseSessionStart, func(req startRequest) error {
		if t.linkSession == nil {
			return nil
		}
		if err := t.linkSession(req.SessionID, req.ProviderSessionID, req.Provider); err != nil {
			return fmt.Errorf("failed to record session: %w", err)
		}
		return nil
	})
}

// parseSessionStart validates a session-start POST body: the lich session id and
// the non-empty provider session id it is reporting. Both must be present. The
// provider that reported is optional and defaults to Claude Code — the only one
// that reported before the field existed — and is rejected when it names a
// provider lich has no registry entry for, like an unknown state on /hook.
func parseSessionStart(body []byte) (startRequest, error) {
	var req startRequest
	if err := json.Unmarshal(body, &req); err != nil {
		return startRequest{}, fmt.Errorf("invalid session-start body: %w", err)
	}
	if req.ProviderSessionID == "" {
		req.ProviderSessionID = req.LegacyClaudeSessionID
	}
	if req.Provider == "" {
		req.Provider = providers.Claude
	}
	if req.SessionID == "" {
		return startRequest{}, errors.New("session-start missing session_id")
	}
	if req.ProviderSessionID == "" {
		return startRequest{}, errors.New("session-start missing provider_session_id")
	}
	if !providers.Known(req.Provider) {
		return startRequest{}, fmt.Errorf("session-start has unknown provider %q", req.Provider)
	}
	return req, nil
}

// titleRequest is an ai-title POST body.
type titleRequest struct {
	SessionID string `json:"session_id"`
	Title     string `json:"title"`
}

// sessionTitle receives the ai-title POST from the Claude Code hook and applies
// it as the session's label via the setTitle callback, which no-ops when the
// user has renamed the session.
func (t *transport) sessionTitle(w http.ResponseWriter, r *http.Request) {
	servePost(t, w, r, parseSessionTitle, func(req titleRequest) error {
		if t.setTitle == nil {
			return nil
		}
		if err := t.setTitle(req.SessionID, req.Title); err != nil {
			return fmt.Errorf("failed to set title: %w", err)
		}
		return nil
	})
}

// parseSessionTitle validates an ai-title POST body: the lich session id and a
// non-empty title (trimmed, since the hook extracts it from a transcript line).
func parseSessionTitle(body []byte) (titleRequest, error) {
	var req titleRequest
	if err := json.Unmarshal(body, &req); err != nil {
		return titleRequest{}, fmt.Errorf("invalid session-title body: %w", err)
	}
	if req.SessionID == "" {
		return titleRequest{}, errors.New("session-title missing session_id")
	}
	req.Title = strings.TrimSpace(req.Title)
	if req.Title == "" {
		return titleRequest{}, errors.New("session-title missing title")
	}
	return req, nil
}

// touchedRequest is a touched POST body.
type touchedRequest struct {
	SessionID string `json:"session_id"`
}

// sessionTouched receives a POST from the Claude Code hook when a session likely
// changed files on disk (a file-mutating tool ran) and forwards the session id
// via the touched callback, which nudges an immediate git-status refresh. It is
// a best-effort latency optimization over the frontend's steady poll, so a
// failure is harmless — the poll still catches the change.
func (t *transport) sessionTouched(w http.ResponseWriter, r *http.Request) {
	servePost(t, w, r, parseSessionTouched, func(req touchedRequest) error {
		if t.touched != nil {
			t.touched(req.SessionID)
		}
		return nil
	})
}

// parseSessionTouched validates a touched POST body: just a non-empty lich
// session id.
func parseSessionTouched(body []byte) (touchedRequest, error) {
	var req touchedRequest
	if err := json.Unmarshal(body, &req); err != nil {
		return touchedRequest{}, fmt.Errorf("invalid session-touched body: %w", err)
	}
	if req.SessionID == "" {
		return touchedRequest{}, errors.New("session-touched missing session_id")
	}
	return req, nil
}

// handle upgrades the single expected client. See authorized for the auth model.
func (t *transport) handle(w http.ResponseWriter, r *http.Request) {
	if !t.authorized(r) {
		http.Error(w, "invalid token", http.StatusUnauthorized)
		return
	}
	conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{InsecureSkipVerify: true})
	if err != nil {
		return
	}
	conn.SetReadLimit(wsReadLimit)

	t.mu.Lock()
	previous := t.conn
	t.conn = conn
	t.mu.Unlock()
	if previous != nil {
		// Not a graceful close: that waits for the replaced client's close
		// reply, on this goroutine, before the new client's read loop starts.
		// Nothing reads the status anyway — the page reconnects on any close.
		_ = previous.CloseNow()
	}
	go t.readLoop(conn)
}

// readLoop forwards input frames to the service until the connection dies.
func (t *transport) readLoop(conn *websocket.Conn) {
	ctx := context.Background()
	for {
		kind, data, err := conn.Read(ctx)
		if err != nil {
			t.drop(conn)
			return
		}
		if kind != websocket.MessageBinary {
			continue
		}
		id, payload, err := decodeFrame(data)
		if err != nil {
			continue
		}
		t.input(id, payload)
	}
}

// drop forgets conn if it is still the active client.
func (t *transport) drop(conn *websocket.Conn) {
	t.mu.Lock()
	if t.conn == conn {
		t.conn = nil
	}
	t.mu.Unlock()
	_ = conn.Close(websocket.StatusNormalClosure, "")
}

// send delivers one session's output frame to the connected client. It
// returns false — and drops the client on write failure — when the caller
// should fall back to the event bridge. One mutex serializes writes for every
// session; per-session queues only if a local loopback write ever measurably
// stalls.
func (t *transport) send(id string, data []byte) bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.conn == nil {
		return false
	}
	frame, err := encodeFrame(id, data)
	if err != nil {
		return false
	}
	ctx, cancel := context.WithTimeout(context.Background(), wsWriteTimeout)
	defer cancel()
	if err := t.conn.Write(ctx, websocket.MessageBinary, frame); err != nil {
		t.conn = nil
		return false
	}
	return true
}

// TransportInfo tells the frontend where the terminal I/O WebSocket lives. A
// zero Port means the transport failed to start and the event bridge is the
// only path.
type TransportInfo struct {
	Port  int    `json:"port"`
	Token string `json:"token"`
}

// TransportError reports why the transport is absent, nil when it started. It
// stays out of TransportInfo on purpose: the page has no use for an OS error,
// and the launch-failure log is the only caller.
func (s *Service) TransportError() error {
	return s.wsErr
}

// Transport returns the WebSocket endpoint for terminal I/O.
func (s *Service) Transport() TransportInfo {
	if s.ws == nil {
		return TransportInfo{}
	}
	return TransportInfo{Port: s.ws.port, Token: s.ws.token}
}
