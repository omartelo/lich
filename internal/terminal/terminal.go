// Package terminal spawns PTY-backed shell sessions and bridges their I/O to the
// frontend over the local WebSocket transport (transport.go), falling back to
// the /events channel. Sessions are keyed by an opaque session ID and run
// independently of the frontend, so navigating away from a project (or hiding
// its terminal) never kills its shell. A project may own several sessions.
package terminal

import (
	"encoding/base64"
	"log/slog"
	"net/http"
	"os"
	"slices"
	"strconv"
	"sync"
	"time"

	"github.com/omartelo/lich/internal/events"
	"github.com/omartelo/lich/internal/pricing"
)

// Event names. A terminal I/O event carries the session ID as a suffix (e.g.
// "terminal:data:home") so each frontend terminal subscribes only to its own
// stream instead of filtering a global broadcast. The session events below are
// global and carry the id in their payload: their consumers outlive any one
// card, and a per-session name can only reach a subscriber that exists when it
// is emitted.
const (
	// dataEventPrefix carries base64-encoded PTY output. Output is base64-encoded
	// because raw PTY bytes may split a multi-byte UTF-8 sequence mid-read, which
	// the JSON event bridge would otherwise corrupt.
	dataEventPrefix = "terminal:data:"
	// exitEventPrefix is emitted once when a session's shell process exits,
	// carrying the status it exited with (see exitEvent).
	exitEventPrefix = "terminal:exit:"
	// statusEventName carries a session's processing state ({id, state, tool,
	// detail, reason} — "busy"/"done"/"waiting"/"idle", plus the tool a pre-tool
	// report names and what a waiting one is blocked on), reported by the lich
	// hooks running inside the PTY (see transport.hook and
	// docs/hooks/session-state.md). "interrupted" is the one value lich raises
	// itself, for a turn the user stopped at the PTY. The frontend keeps it in
	// stores keyed by id (session-status-store.ts, session-tool-store.ts) rather
	// than in the card, which is only mounted while its project is active.
	statusEventName = "session-status"
	// titleEventName carries an auto-applied session label ({id, label}).
	titleEventName = "session-title"
	// touchedEventName carries the id of a session that likely changed files on
	// disk, nudging an immediate git-status refresh ahead of the steady poll.
	touchedEventName = "session-touched"
	// agentEventName carries which provider CLI is live inside a session's PTY
	// ({id, agent}) — reported through the session-start hook by whichever
	// provider ships it, so a shell card shows that provider's mark while its
	// CLI runs in it. An empty agent clears the mark; every PTY spawn emits that
	// clear so a respawned session never wears a dead agent's icon.
	agentEventName = "session-agent"
	// sandboxEventName carries whether a session's PTY runs confined ({id,
	// confined}), emitted by every spawn. The card marks a confined session, and
	// the answer is the spawn's own — it takes the provider's rung, the checkout
	// and a per-session override to reach, so the window is told rather than
	// asked to work it out again. Persisted with the row too (store.Session's
	// Sandbox), which is what a page reload hydrates from.
	sandboxEventName = "session-sandbox"
	// turnEventName carries the id of a session whose last finished turn has
	// just been filed ({id}) — emitted when the closing snapshot lands, not when
	// the turn's `done` is reported. The two are not the same moment: the
	// snapshot runs on a worker, so a panel refreshed off the state report reads
	// the record before it exists (see turnSnaps).
	turnEventName = "session-turn"
)

// statusEvent is the payload of statusEventName: the session whose processing
// state changed, the new state, and — on a pre-tool report alone — the tool it
// is about to run and what that tool acts on, or — on a waiting one alone — what
// it is blocked on. All three are the provider's own words, never translated
// here (docs/hooks/session-state.md tables them).
type statusEvent struct {
	ID     string `json:"id"`
	State  string `json:"state"`
	Tool   string `json:"tool,omitempty"`
	Detail string `json:"detail,omitempty"`
	Reason string `json:"reason,omitempty"`
}

// titleEvent is the payload of titleEventName: the session whose label changed
// and its new label.
type titleEvent struct {
	ID    string `json:"id"`
	Label string `json:"label"`
}

// touchedEvent is the payload of touchedEventName: the session whose files
// likely changed.
type touchedEvent struct {
	ID string `json:"id"`
}

// exitEvent is the payload of exitEventPrefix: the status the session's PTY
// child exited with. Code is absent when the child left none — killed by a
// signal, or a wait that failed — so the window can say the session ended
// without reporting a clean exit nobody observed.
type exitEvent struct {
	Code *int `json:"code,omitempty"`
}

// exitPayload turns what Wait reaped into that payload. Every negative code is
// noExitStatus, not only the constant: a process cannot exit negative, so a
// negative is the seam saying it has nothing to report.
func exitPayload(code int) exitEvent {
	if code < 0 {
		return exitEvent{}
	}
	return exitEvent{Code: &code}
}

// agentEvent is the payload of agentEventName: the session and the provider
// CLI now live in its PTY ("" when none is).
type agentEvent struct {
	ID    string `json:"id"`
	Agent string `json:"agent"`
}

// sandboxEvent is the payload of sandboxEventName: the session and whether its
// PTY is confined.
type sandboxEvent struct {
	ID       string `json:"id"`
	Confined bool   `json:"confined"`
}

// turnEvent is the payload of turnEventName: the session whose last-turn record
// just changed. It carries no diff — the panel asks for one only while it is on
// screen, and a turn's diff is far larger than an event ought to be.
type turnEvent struct {
	ID string `json:"id"`
}

// session is a single running PTY-backed shell. done closes when the session
// is reaped (by stream or Close — whichever removes it from the map), stopping
// its cwd watcher. replay holds a capped tail of the PTY's output so a
// reconnecting frontend can reseed its scrollback after a page reload. outbox
// carries the coalesced output to the frontend off the PTY read path.
type session struct {
	pty    ptyHandle
	out    *coalescer
	done   chan struct{}
	replay *replayBuffer
	outbox *outbox
	// settingUp is true while the project's worktree setup script still owns
	// this PTY, before the provider it execs into. Cleared by the marker the
	// wrapper prints (see setupDone). Guarded by the service's mu.
	settingUp bool
	// setupTail is the end of the previous chunk, held only while settingUp so
	// the marker still matches when a PTY read splits it. Guarded by the
	// service's mu.
	setupTail string
	// lastOut is when this PTY last produced output, and ready records that it
	// has since gone quiet once — that its program finished drawing itself and
	// is waiting on input. Both guarded by the service's mu.
	lastOut time.Time
	ready   bool
	// draftAt is when the user last typed something at this prompt without
	// sending it, and zero when there is nothing of theirs on the line.
	// escPending is the beginning of an escape sequence a PTY write split, held
	// until the rest of it arrives. pasting is whether the bytes arriving now
	// are inside a bracketed paste, which is what keeps a pasted Ctrl+C or
	// Escape from reading as the user interrupting the turn. All three guarded
	// by the service's mu. See draft.go.
	draftAt    time.Time
	escPending []byte
	pasting    bool
	// confined records whether this PTY was spawned inside the sandbox, so Start
	// can report it once the spawn is out of the lock.
	confined bool
}

// Store is the persistence the terminal service depends on: the binary to spawn
// for a provider in a project (empty return spawns the provider's default),
// whether a given session runs one of those rather than the provider's own,
// whether that spawn drops the provider's permission prompts and whether it runs
// confined, that project's own directory, the dev-server port reserved for each
// checkout, where to record the provider session id a PTY reports through its
// session-start hook, and the running cost accounting behind the footer readout
// (CostReadout gates it — off, none of the rest is called). The store implements
// them all.
type Store interface {
	ProviderBin(providerID, projectID string) string
	SessionCustomBin(sessionID string) bool
	SkipPermissions(providerID, projectID, cwd string) bool
	ProjectPath(projectID string) string
	WorktreePorts() map[string]int
	SetWorktreePort(path string, port int) error
	SetProviderSession(sessionID, providerSessionID string) error
	ProviderSession(sessionID string) (string, error)
	SessionModel(sessionID string) string
	SessionEntrypoint(sessionID string) string
	SessionSandbox(sessionID string) string
	SetSessionSandbox(sessionID, sandbox string) error
	SandboxDefault(providerID, projectID, cwd string) bool
	SandboxSSHAgent(projectID string) bool
	SandboxGHToken(projectID string) bool
	GHAccountForPath(path string) string
	SetSessionTitle(sessionID, title string) (bool, error)
	CostReadout() bool
	CostLedger(sessionID, transcriptID string) (int64, string, float64, error)
	SaveCostLedger(sessionID, transcriptID string, offset int64, lastMessage string, cost float64) error
	SessionCost(sessionID string) (float64, error)
	AddHandsOn(sessionID string, seconds int64) error
	HandsOn(sessionID string) (int64, error)
}

// Service manages PTY-backed shell sessions keyed by session ID.
type Service struct {
	mu       sync.Mutex
	sessions map[string]*session
	store    Store
	// hub pushes app events to the window over /events; see internal/events.
	hub *events.Hub
	// env is the environment every spawned session inherits: the launch
	// environment cleaned of AppImage runtime leakage (see childEnv), plus TERM.
	env []string
	// ws is the local WebSocket transport for terminal I/O (see transport.go);
	// nil when it failed to start, leaving /events and the RPC as the path.
	ws *transport
	// wsErr is why ws is nil. Kept because a failed bind of the pinned port is
	// how the app fails to launch at all, and the OS error is the only thing
	// telling a port somebody else holds apart from a port the OS refuses to
	// hand over — Windows answers WSAEACCES for a port inside an excluded
	// range, with nothing listening on it.
	wsErr error
	// prices resolves what a session's tokens cost. Nil disables the cost
	// readout outright — the state a test that builds a bare Service is in.
	prices rateSource
	// onState, when set, receives every session-state report. The relay watches
	// it to notice a target that worked through a request and ended its turn
	// without answering (internal/relay). Guarded by mu: wired after the
	// transport is already serving.
	onState func(id, state string)
	// spawns is how each live session's PTY was started — what it runs and where
	// — keyed by session id. Read by providerKind, which is what stops a
	// session-start report from repainting a card lich itself chose the provider
	// for, by the state report's own filter, and by the cost readout, which asks
	// where Crush keeps the database for this checkout.
	//
	// Deliberately not under mu, and that is the whole reason it is not simply a
	// field on session: spawnSession holds mu across the PTY spawn, so a report
	// that had to ask mu what a session runs would queue behind another
	// session's spawn — which is exactly what the note on turns below forbids.
	// Written once per spawn and read on every report, which is what sync.Map
	// is for.
	spawns sync.Map
	// turns is which sessions have a turn open right now, which is what tells a
	// `waiting` report that a human is blocking from one that only says the
	// session is sitting at its prompt (see turnLog). It carries its own lock:
	// nothing else about a report needs mu, and a hook must never queue behind a
	// PTY spawn.
	turns turnLog
	// hands is how long each session has been worked on, measured off the same
	// three signals the card is already drawing — a state report, a keystroke,
	// output while the agent is busy (see handsOn). It carries its own lock for
	// the reason turns does: a hook must never queue behind a PTY spawn.
	hands handsOn
	// snaps brackets each session's turn with a tree snapshot of its checkout,
	// which is what the Review panel's "Last turn" mode diffs (see turnSnaps).
	// It reads the same boundary turns does and carries its own lock for the
	// same reason.
	snaps turnSnaps
	// lastCols/lastRows is the last terminal size the window reported, and the
	// size a session spawned with none of its own is started at. See
	// sizeFor. Guarded by mu.
	lastCols, lastRows int
	// dropDir is where copies of dropped files live (internal/drop). Wired at
	// startup; empty leaves a confined session without the copies dropped into
	// it, never without a spawn. See SetDropDir.
	dropDir string
}

// SetDropDir names the directory holding the copies of dropped files
// (internal/drop). A confined session binds its own subdirectory of it at spawn
// — the copy is written by lich, outside the sandbox, so without the bind the
// path pasted at the prompt names a file the session cannot open.
//
// Startup wiring, called before anything spawns.
func (s *Service) SetDropDir(dir string) {
	s.dropDir = dir
}

// readySettle is how long a session's PTY must stay quiet before lich hands it
// work: long enough that a provider drawing its opening screen is not mistaken
// for one waiting at a prompt, short enough to sit inside the wait an agent
// gives a tool call. Measured against Claude Code, whose splash lands in
// several bursts.
const readySettle = 600 * time.Millisecond

// The size a session is started at when nothing has ever measured a terminal —
// no window has opened yet, so there is no real one to copy. A conventional
// terminal, which every TUI draws itself into.
const (
	fallbackCols = 80
	fallbackRows = 24
)

// sizeFor resolves the size to start a PTY at. A caller that measured a
// terminal passes it and gets it back; one that has no terminal to measure
// (internal/spawn, opening a session for an agent) passes zero and gets the
// last size the window reported.
//
// Copying that size is what keeps such a session readable. Its provider draws
// its whole conversation into whatever grid it is given, and the window replays
// those exact bytes into the terminal it builds when somebody finally opens the
// card. Started at a size the window does not have, the session is redrawn on
// the first view — at which point the TUI repaints and what it had already
// written is gone from the screen. The conversation survives in the provider,
// not on the screen the user came to read.
//
// Called under s.mu.
func (s *Service) sizeFor(cols, rows int) (int, int) {
	if cols > 0 && rows > 0 {
		s.lastCols, s.lastRows = cols, rows
		return cols, rows
	}
	if s.lastCols > 0 && s.lastRows > 0 {
		return s.lastCols, s.lastRows
	}
	return fallbackCols, fallbackRows
}

// New returns a ready-to-use terminal service that resolves the binary to spawn
// through store. env is the process environment to derive session environments
// from — callers pass a snapshot taken before any os.Setenv tweaks (main.go
// forces GDK_BACKEND on Linux) so those never leak into spawned shells.
// hub receives every app event the service pushes to the UI.
func New(store Store, env []string, hub *events.Hub) *Service {
	s := &Service{
		sessions: make(map[string]*session),
		store:    store,
		hub:      hub,
		// Clipped before the append: outside an AppImage childEnv hands back the
		// caller's own slice, and appending into its spare capacity would write
		// TERM into the array main still holds.
		env:    append(slices.Clip(childEnv(env)), "TERM=xterm-256color"),
		prices: pricing.New(),
	}
	// Wired here rather than emitted from the hook's own goroutine: the record
	// is filed on the snapshot worker, minutes-of-CPU later on a cold checkout,
	// and the panel has no other way to learn that the answer it already asked
	// for has changed.
	s.snaps.filed = func(id string) {
		hub.Emit(turnEventName, turnEvent{ID: id})
	}
	ws, err := newTransport(s.onInput, s.onHookState, s.onSessionStart, s.onTitle, s.onTouched)
	s.ws, s.wsErr = ws, err
	if ws != nil {
		// The transport is built before the service that owns the bridge, so the
		// path output takes when the socket cannot carry it is wired here.
		ws.setFallback(s.emitData)
	}
	return s
}

// onInput is the transport's keyboard callback. Only this path, never
// writeBytes, beats hands-on: what arrives here is the window's own input
// frames, which is a person at the keyboard. A relayed message another session
// pasted through Write is that session's work, not this one's.
func (s *Service) onInput(id string, data []byte) {
	s.beatHandsOn(id, 0)
	if s.noteInput(id, data) {
		s.noteInterrupt(id)
	}
	if err := s.writeBytes(id, data); err != nil {
		slog.Warn("terminal: input write failed", "session", id, "err", err)
	}
}

// onHookState is the transport's state-report callback, one per hook a
// session's provider fires.
func (s *Service) onHookState(req hookRequest) {
	// Ahead of every filter below, because a beat is not about what the
	// card can draw: any hook naming a session is proof that session was
	// being worked, whatever endpoint it arrived on and whether or not
	// lich publishes it. That is why the other three callbacks beat too
	// — a provider whose hooks report no state has no other way to say
	// its agent is running. Cursor CLI's tool reports are dropped a line
	// down for want of anything that could end the spinner they would
	// start, and they are the only proof a Cursor turn is running at all
	// (see closableState).
	s.beatHandsOn(req.SessionID, 0)
	// Dropped where the harness cannot close it, before the turn log
	// ever sees it: a state nothing ends outlives what it describes.
	if !closableState(s.kindOf(req.SessionID), req.State) {
		return
	}
	// The window is told what the report means, not what it said: a
	// `waiting` outside a turn is a session idle at its prompt, and the
	// card would draw it as a human being blocked (see turnLog).
	if s.turns.report(req.SessionID, req.State) {
		s.hub.Emit(statusEventName, statusEvent{
			ID:     req.SessionID,
			State:  req.State,
			Tool:   req.Tool,
			Detail: req.Detail,
			Reason: req.Reason,
		})
	}
	// The relay reads the stream raw: it keeps its own turn accounting,
	// and a report held back from the window still moves an errand.
	if watch := s.stateWatcher(); watch != nil {
		watch(req.SessionID, req.State)
	}
	// The same boundary brackets the Review panel's "Last turn": `busy`
	// opens the window, `done` closes it. Queued, never taken here — a
	// snapshot on this goroutine would hold the hook's own response, and
	// with it the agent's next step (see turnSnaps).
	s.snaps.note(req.SessionID, req.State)
	// Every non-idle state is a point where a fresh assistant usage line
	// may have landed — a tool call mid-turn (busy), a prompt (waiting),
	// or the turn's end (done) — so refresh the context window then, and
	// off-thread so a stalled emit never blocks the hook's response. Skip
	// idle (SessionEnd): the session is ending, nothing new to read.
	if req.State != statusIdle {
		go s.emitUsage(req.SessionID)
	}
}

// onSessionStart is the transport's session-start callback: the provider's
// CLI reporting which conversation it runs.
func (s *Service) onSessionStart(sessionID, providerSessionID, provider string) error {
	// The beat above, on the one report Crush can make. Its only hook
	// event is PreToolUse and neither script it registers there reports
	// a state (docs/hooks/session-state.md), so this arrives once per
	// tool call — measured 2026.09.03 against Crush 0.88.0 — and is the
	// only proof a Crush turn is running.
	s.beatHandsOn(sessionID, 0)
	if err := s.store.SetProviderSession(sessionID, providerSessionID); err != nil {
		return err
	}
	// A SessionStart report is proof that *a* provider's CLI is running
	// in this PTY, and for a shell session that report is the only thing
	// that knows which — so its icon wears the reported mark.
	//
	// For a session lich spawned as a provider, the kind is the better
	// answer and it wins. A harness can run another harness's hooks:
	// Cursor CLI executes every Claude Code hook on the machine, the
	// user's own and each installed plugin's (measured on 2026.08.11,
	// `hookSource: claude-plugin`), so the lich plugin's own script
	// reports `claude` from inside a Cursor session and the card wore
	// Claude's mark one turn in. What a hook says is a claim; what lich
	// spawned is a fact.
	s.hub.Emit(agentEventName, agentEvent{ID: sessionID, Agent: s.providerKind(sessionID, provider)})
	return nil
}

// onTitle is the transport's ai-title callback.
func (s *Service) onTitle(id, title string) error {
	s.beatHandsOn(id, 0)
	applied, err := s.store.SetSessionTitle(id, title)
	if err != nil {
		return err
	}
	if applied {
		s.hub.Emit(titleEventName, titleEvent{ID: id, Label: title})
	}
	return nil
}

// onTouched is the transport's callback for a hook that names a session and
// reports nothing else.
func (s *Service) onTouched(id string) {
	s.beatHandsOn(id, 0)
	s.hub.Emit(touchedEventName, touchedEvent{ID: id})
}

// emitData puts one session's output on the /events bridge — where terminal
// output goes whenever the /ws transport cannot carry it, whether because no
// client is connected or because the socket refused the frame.
func (s *Service) emitData(id string, data []byte) {
	s.hub.Emit(dataEventPrefix+id, base64.StdEncoding.EncodeToString(data))
}

// Mount exposes an extra handler (the RPC dispatcher, the events push socket)
// on the transport listener, behind its token. No-op when the transport
// failed to start — those surfaces then simply don't exist, like /ws.
func (s *Service) Mount(pattern string, handler http.Handler) {
	if s.ws == nil {
		return
	}
	s.ws.mount(pattern, handler)
}

// MountPublic exposes a tokenless handler on the transport listener — the
// static frontend the Chromium shell loads before it knows the token.
func (s *Service) MountPublic(pattern string, handler http.Handler) {
	if s.ws == nil {
		return
	}
	s.ws.mountPublic(pattern, handler)
}

// noteInterrupt publishes the end of a turn the user stopped at the PTY: a lone
// Ctrl+C or Escape while lich has a turn open for that session (see noteInput
// and turnLog.interrupt). It is the fallback for the three providers that raise
// no event of their own when a turn is interrupted — Claude Code, Codex and
// oh-my-pi all skip the hook that would end it, so without this the card spins
// until some later turn finishes. It publishes "interrupted" rather than "done"
// because stopping a turn is not finishing one, and it never opens a turn, so
// nothing is invented for a session lich never heard start. Whatever the
// provider reports next overwrites it: an event from inside the session always
// outranks a guess made from its keystrokes.
func (s *Service) noteInterrupt(id string) {
	if !s.turns.interrupt(id) {
		return
	}
	s.hub.Emit(statusEventName, statusEvent{ID: id, State: statusInterrupted})
	// An interrupted turn is still a turn that ended, and it changed files like
	// any other. Closing the window here is what keeps the panel from holding
	// an older turn open until the next one finishes.
	s.snaps.closeTurn(id)
	// The relay keeps its own turn accounting off the same stream, and a turn
	// that ended has to read as ended there too — a queued delivery waits on the
	// target's prompt being free.
	if watch := s.stateWatcher(); watch != nil {
		watch(id, statusInterrupted)
	}
	// An interrupted turn spent tokens like any other, and it may be the last
	// one for a while: refresh the context readout off the transcript, off the
	// caller's thread the way the hook path does.
	go s.emitUsage(id)
}

// SetSessionState wires fn to every session-state report the hooks deliver.
// Only one watcher: the relay is the only thing that needs the stream, and the
// window gets the same reports over its own event channel.
func (s *Service) SetSessionState(fn func(id, state string)) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.onState = fn
}

// stateWatcher reads the watcher under the lock, so a report arriving while it
// is being wired sees one function or none, never a torn read.
func (s *Service) stateWatcher() func(id, state string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.onState
}

// SetRestart wires the POST /restart endpoint to fn, the in-place relaunch the
// update flow triggers after replacing the binary. No-op when the transport
// failed to start — /restart then simply reports unavailable.
func (s *Service) SetRestart(fn func() error) {
	if s.ws == nil {
		return
	}
	s.ws.setRestart(fn)
}

// sessionEnv is the environment for one PTY: the shared base, the project this
// session belongs to, the dev-server port its checkout owns, and the loopback
// coordinates a provider's hook needs to report this session's status back to
// lich. All of it is per-session, so this returns a fresh slice rather than
// aliasing (and appending to) the shared s.env.
//
// LICH_PROJECT_DIR and LICH_WORKTREE_PORT belong to the checkout rather than to
// the transport, so they are exported even when the transport failed to start.
// The rest are the transport's: with none there is nowhere to report, so a hook
// spawned in this PTY sees no LICH_PORT and no-ops, and the `lich` CLI it would
// have called has nothing to call.
func (s *Service) sessionEnv(id, projectID, cwd string) []string {
	env := make([]string, len(s.env), len(s.env)+6)
	copy(env, s.env)
	if dir := s.store.ProjectPath(projectID); dir != "" {
		env = append(env, "LICH_PROJECT_DIR="+dir)
	}
	if port := s.reserveWorktreePort(cwd); port > 0 {
		env = append(env, "LICH_WORKTREE_PORT="+strconv.Itoa(port))
	}
	if s.ws == nil {
		return env
	}
	env = append(env,
		"LICH_PORT="+strconv.Itoa(s.ws.port),
		"LICH_TOKEN="+s.ws.token,
		"LICH_SESSION_ID="+id,
	)
	if bin := lichBin(); bin != "" {
		env = append(env, "LICH_BIN="+bin)
	}
	return env
}

// lichBin is the path of the running lich binary, exported as LICH_BIN so a
// session can call the `lich` CLI back (internal/cli) — which is how one
// session reaches another. The path rather than the name: an installed lich is
// on $PATH and a `task dev` build is not, and on a machine running both, the
// name would resolve to whichever came first rather than to the lich this
// session belongs to. Empty when the path cannot be resolved; nothing is
// exported then and a caller falls back to $PATH.
var lichBin = sync.OnceValue(func() string {
	exe, err := os.Executable()
	if err != nil {
		slog.Warn("resolve executable — LICH_BIN unset in sessions", "err", err)
		return ""
	}
	return exe
})

// Close terminates a session's shell, if any.
func (s *Service) Close(id string) error {
	s.mu.Lock()
	sess, ok := s.sessions[id]
	if ok {
		delete(s.sessions, id)
		close(sess.done)
	}
	s.mu.Unlock()
	s.spawns.Delete(id)

	// Before the bail below, because a card is closed far more often than it is
	// running: its row is deleted either way, so nothing will ever ask what its
	// last turn changed — and its snapshot index would otherwise outlive it on
	// disk for the rest of the machine's life.
	s.snaps.forget(id)
	// Synchronously, and before the row that the window is about to delete goes:
	// a write that loses that race is dropped silently (store.AddHandsOn), and
	// this is the last chance a closed card gets to keep what it was worked.
	s.FlushHandsOn()
	s.hands.forget(id)
	if !ok {
		return nil
	}
	return sess.pty.Close()
}

// HandsOn is how long session id has been worked on, in whole seconds: the time
// it was reporting, being typed at or producing output for an open turn, minus
// every silence longer than handsOnIdleGap (see handsOn). Zero for a session
// nothing has been counted for yet, and up to handsOnFlush behind what has
// actually been measured — the readout is in minutes, so the debounce never
// shows.
func (s *Service) HandsOn(id string) (int64, error) {
	return s.store.HandsOn(id)
}

// FlushHandsOn writes what every session has been worked since the last write.
// Called off the debounce, when a card closes and once on the way out, so a
// session's hours outlive the process that counted them.
func (s *Service) FlushHandsOn() {
	for id, seconds := range s.hands.drain() {
		if err := s.store.AddHandsOn(id, seconds); err != nil {
			s.hands.restore(id, seconds)
			slog.Warn("terminal: save hands-on time", "session", id, "err", err)
		}
	}
}

// beatHandsOn records activity in session id, and writes the arrears off the
// caller's goroutine once the debounce is up — a hook must never wait on
// SQLite, and neither must the PTY reader.
func (s *Service) beatHandsOn(id string, minGap time.Duration) {
	if s.hands.beat(id, minGap) {
		go s.FlushHandsOn()
	}
}

// Live reports whether a session has a process running right now. It is the
// difference between a card and a PTY: a session the user has never opened in
// this page has a row in the store and nothing behind it to type at, so it can
// be listed but never addressed (internal/relay).
func (s *Service) Live(id string) bool {
	return s.ptyOf(id) != nil
}

// ptyOf returns the PTY for a session, or nil if it is not running.
func (s *Service) ptyOf(id string) ptyHandle {
	s.mu.Lock()
	defer s.mu.Unlock()
	if sess, ok := s.sessions[id]; ok {
		return sess.pty
	}
	return nil
}
