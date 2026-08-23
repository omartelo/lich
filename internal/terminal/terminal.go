// Package terminal spawns PTY-backed shell sessions and bridges their I/O to the
// frontend over the local WebSocket transport (transport.go), falling back to
// the /events channel. Sessions are keyed by an opaque session ID and run
// independently of the frontend, so navigating away from a project (or hiding
// its terminal) never kills its shell. A project may own several sessions.
package terminal

import (
	"encoding/base64"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/omartelo/lich/internal/events"
	"github.com/omartelo/lich/internal/pricing"
	"github.com/omartelo/lich/internal/project"
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
	// turns is which sessions have a turn open right now, which is what tells a
	// `waiting` report that a human is blocking from one that only says the
	// session is sitting at its prompt (see turnLog). It carries its own lock:
	// nothing else about a report needs mu, and a hook must never queue behind a
	// PTY spawn.
	turns turnLog
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
	ws, err := newTransport(
		func(id string, data []byte) {
			if s.noteInput(id, data) {
				s.noteInterrupt(id)
			}
			if err := s.writeBytes(id, data); err != nil {
				slog.Warn("terminal: input write failed", "session", id, "err", err)
			}
		},
		func(req hookRequest) {
			// The window is told what the report means, not what it said: a
			// `waiting` outside a turn is a session idle at its prompt, and the
			// card would draw it as a human being blocked (see turnLog).
			if s.turns.report(req.SessionID, req.State) {
				hub.Emit(statusEventName, statusEvent{
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
			// Every non-idle state is a point where a fresh assistant usage line
			// may have landed — a tool call mid-turn (busy), a prompt (waiting),
			// or the turn's end (done) — so refresh the context window then, and
			// off-thread so a stalled emit never blocks the hook's response. Skip
			// idle (SessionEnd): the session is ending, nothing new to read.
			if req.State != statusIdle {
				go s.emitUsage(req.SessionID)
			}
		},
		func(sessionID, providerSessionID, provider string) error {
			if err := store.SetProviderSession(sessionID, providerSessionID); err != nil {
				return err
			}
			// A SessionStart report is proof that provider's CLI is running in
			// this PTY — whatever the card's kind, its icon can wear that
			// provider's mark now.
			hub.Emit(agentEventName, agentEvent{ID: sessionID, Agent: provider})
			return nil
		},
		func(id, title string) error {
			applied, err := store.SetSessionTitle(id, title)
			if err != nil {
				return err
			}
			if applied {
				hub.Emit(titleEventName, titleEvent{ID: id, Label: title})
			}
			return nil
		},
		func(id string) {
			hub.Emit(touchedEventName, touchedEvent{ID: id})
		},
	)
	s.ws, s.wsErr = ws, err
	return s
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

// readBufSize is the chunk size read from a session's PTY per iteration.
const readBufSize = 32 * 1024

// Start spawns the binary for session id under project projectID — the user's
// shell when kind is "shell", otherwise the provider binary for that kind
// resolved from the project's settings (falling back to the provider's default
// on $PATH) — attached to a new PTY sized to cols x rows and rooted at cwd, then
// streams its output to the frontend. An empty cwd defaults to the user's home
// directory. Starting a session that is already running is a no-op.
//
// A non-empty resume is a provider conversation id to reopen, spelled for the
// session's kind by resumeArgs, which the frontend passes after the user
// accepted the prompt to continue the session this card ran before the last
// restart. The prompt is only raised for a conversation ResumeAvailable still
// finds, so an id the provider no longer knows normally never reaches here; one
// that slips through (pruned between the check and the spawn) fails in the PTY
// like any other bad invocation — the user sees the provider's own error.
//
// name is what the session answers to in its provider's peer roster (Claude
// Code's `/list-agents`), passed by the frontend so the roster names the card
// the user sees. Only Claude Code has a roster; every other kind ignores it.
//
// setup is passed once, by the flow that just created this session's worktree:
// it runs the project's worktree setup script (.lich/setup-worktree.sh, see
// project.SetupScript) in the PTY before the provider, so a fresh checkout
// installs its dependencies in view. A respawn or resume never sets it. The
// script runs in the session's own environment, so it reads the same
// LICH_WORKTREE_PORT the provider will.
func (s *Service) Start(id, projectID, cwd, kind, resume, name string, setup bool, cols, rows int) error {
	sess, cwd, err := s.spawnSession(id, projectID, cwd, kind, resume, name, setup, cols, rows)
	if err != nil || sess == nil {
		return err
	}
	// Emitted outside s.mu: Emit blocks on a stalled /events client, which
	// would freeze every session's I/O. Both are unconditional so a respawn
	// overwrites whatever the previous PTY left in the frontend's stores.
	s.hub.Emit(cwdEventName, cwdEvent{ID: id, Cwd: cwd})
	s.hub.Emit(agentEventName, agentEvent{ID: id, Agent: ""})
	s.hub.Emit(sandboxEventName, sandboxEvent{ID: id, Confined: sess.confined})
	go watchCwd(id, sess.pty.Pid(), cwd, sess.done, s.hub)
	return nil
}

// spawnSession is the locked half of Start: dedupe, PTY spawn and session-map
// registration. A nil session with a nil error means id was already running.
// The returned cwd is the effective start directory (the input, or the
// resolved home when it was empty).
func (s *Service) spawnSession(id, projectID, cwd, kind, resume, name string, setup bool, cols, rows int) (*session, string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, running := s.sessions[id]; running {
		return nil, "", nil
	}
	// Whatever the previous provider left open under this id died with its PTY,
	// and the first report of the new one has to be read against its own turn.
	s.turns.forget(id)
	cols, rows = s.sizeFor(cols, rows)

	if cwd == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, "", fmt.Errorf("failed to resolve home directory: %w", err)
		}
		cwd = home
	}

	// The MCP registration is withheld without a transport for the same reason
	// the hook coordinates are: the server it points at would have nothing to
	// talk to, so the session would carry tools that can only fail.
	mcpBin := ""
	if s.ws != nil {
		mcpBin = lichBin()
	}
	skipPermissions := s.store.SkipPermissions(kind, projectID, cwd)
	// The model is read from the row rather than passed in, so it survives every
	// spawn this session ever gets: the window's first view, a respawn after a
	// reload, and the resume of a parked worktree session all arrive here.
	spec := ptySpec{
		bin:  resolveCommand(kind, s.store.ProviderBin(kind, projectID), userShell()),
		args: providerArgs(kind, name, resume, s.store.SessionModel(id), mcpBin, skipPermissions),
		dir:  cwd,
		env:  s.sessionEnv(id, projectID, cwd),
		cols: cols,
		rows: rows,
	}
	// Before wrapSetup, so a fresh worktree terminal carrying both runs the
	// project's setup script first, then the entrypoint, then the shell.
	spec = wrapEntrypoint(spec, kind, s.store.SessionEntrypoint(id), runtime.GOOS)
	settingUp := false
	if setup {
		spec, settingUp = wrapSetup(spec, project.SetupScript(s.store.ProjectPath(projectID)), runtime.GOOS)
	}
	// Outermost, so the setup script and the entrypoint are confined with the
	// session they run in front of.
	inSandbox := confined(s.store, id, kind, projectID, cwd)
	creds := s.sandboxCredentials(projectID, cwd, inSandbox)
	spec = wrapSandbox(spec, kind, userHome(), sessionDropDir(s.dropDir, id, inSandbox), inSandbox, creds)
	p, err := startPTY(spec)
	if err != nil {
		return nil, "", fmt.Errorf("failed to start pty for %q: %w", id, err)
	}

	box := newOutbox(func(data []byte) {
		if s.ws != nil && s.ws.send(id, data) {
			return
		}
		encoded := base64.StdEncoding.EncodeToString(data)
		s.hub.Emit(dataEventPrefix+id, encoded)
	}, outboxDepth)
	out := newCoalescer(box.push, visibleFlushInterval, hiddenFlushInterval)
	sess := &session{
		pty:       p,
		out:       out,
		done:      make(chan struct{}),
		replay:    newReplayBuffer(replayCapBytes),
		outbox:    box,
		settingUp: settingUp,
		// Timed from the spawn, so a program that never writes anything still
		// becomes ready once: quiet is the signal, and silence from the start
		// is quiet too.
		lastOut:  time.Now(),
		confined: inSandbox,
	}
	s.sessions[id] = sess
	go s.stream(id, sess)
	return sess, cwd, nil
}

// stream copies PTY output to the frontend until the PTY is closed, then reaps
// the process, drops the session and emits its exit event. Output goes through
// the session's coalescer, which batches it on a short cadence while the
// terminal is visible and a long one while it is hidden, and then through its
// outbox, which delivers it off this goroutine.
func (s *Service) stream(id string, sess *session) {
	p := sess.pty
	buf := make([]byte, readBufSize)
	for {
		n, err := p.Read(buf)
		if n > 0 {
			s.noteOutput(sess, buf[:n])
			sess.replay.append(buf[:n])
			sess.out.Write(buf[:n])
		}
		if err != nil {
			break
		}
	}
	code, _ := p.Wait()
	// Release the PTY handle: on a natural child exit nobody else closes it
	// (Close only reaps sessions still in the map), and after a user-driven
	// Close this is the second one — a no-op each seam has to make safe, since
	// the handles are already gone (see windowsPTY, where they are reissued).
	_ = p.Close()
	// Flush any batched output and wait for it to be delivered before the exit
	// event, so the frontend always sees the final bytes ahead of the exit
	// banner.
	sess.out.Close()
	sess.outbox.close()

	s.mu.Lock()
	reaped := false
	if current, ok := s.sessions[id]; ok && current.pty == p {
		delete(s.sessions, id)
		close(current.done)
		reaped = true
	}
	s.mu.Unlock()

	// Only the goroutine that actually evicted its own PTY owes an exit event:
	// a reap that lost the race to a respawn under the same id would otherwise
	// tell a live session it exited, and the frontend writes "[process exited]"
	// into that terminal. Emitted outside s.mu for the reason Start gives —
	// Emit blocks on a stalled /events client, and holding s.mu across it would
	// freeze every session's I/O.
	if reaped {
		s.hub.Emit(exitEventPrefix+id, exitPayload(code))
	}
}

// Write forwards keyboard input from the frontend to a session's PTY.
func (s *Service) Write(id, data string) error {
	return s.writeBytes(id, []byte(data))
}

// writeBytes delivers input bytes to a session's PTY; unknown sessions are a
// no-op. It is the shared sink for the RPC Write and the WebSocket
// transport's input frames.
//
// It loops until every byte is written. A single call is not enough:
// ConPTY's input pipe (pty_windows.go) is a plain Windows anonymous pipe, and
// WriteFile on one can legitimately return fewer bytes than given rather than
// blocking for the rest — the caller is documented to retry. A large paste
// landing exactly on that boundary lost its tail silently, closing bracket
// and all, so bracketed paste never closed and the Enter that followed
// landed inside it instead of submitting it. Unix's *os.File already loops
// internally, so this is a no-op extra check there.
func (s *Service) writeBytes(id string, data []byte) error {
	p := s.ptyOf(id)
	if p == nil {
		return nil
	}
	for len(data) > 0 {
		n, err := p.Write(data)
		if err != nil {
			return err
		}
		data = data[n:]
	}
	return nil
}

// SetVisible tells the session's output coalescer whether its terminal is on
// screen. Hidden sessions batch output (~250ms per event); flipping to visible
// flushes pending output immediately. Unknown sessions are a no-op.
func (s *Service) SetVisible(id string, visible bool) error {
	s.mu.Lock()
	sess, ok := s.sessions[id]
	s.mu.Unlock()
	if !ok {
		return nil
	}
	sess.out.SetVisible(visible)
	return nil
}

// Replay returns the capped tail of a session's PTY output, base64-encoded, so a
// reconnecting frontend can reseed its scrollback after a page reload discarded
// the page-side buffer. Empty for an unknown session — a brand-new one has no
// history yet. Base64 for the same reason the data event is: raw PTY bytes may
// split a multi-byte UTF-8 sequence across the JSON envelope.
func (s *Service) Replay(id string) (string, error) {
	s.mu.Lock()
	sess, ok := s.sessions[id]
	s.mu.Unlock()
	if !ok {
		return "", nil
	}
	return base64.StdEncoding.EncodeToString(sess.replay.snapshot()), nil
}

// Resize updates a session's PTY window size. The frontend only calls this for
// the visible terminal; a hidden terminal is resized on the next time it is
// shown.
func (s *Service) Resize(id string, cols, rows int) error {
	// Recorded even when the session it names is gone: it is the window's own
	// terminal being measured, and the next session spawned without one starts
	// at that size (see sizeFor).
	if cols > 0 && rows > 0 {
		s.mu.Lock()
		s.lastCols, s.lastRows = cols, rows
		s.mu.Unlock()
	}
	p := s.ptyOf(id)
	if p == nil {
		return nil
	}
	return p.Resize(cols, rows)
}

// noteOutput reads one chunk of a session's output for the two things lich has
// to know about it from outside: whether the worktree setup script is still the
// program on the other end of this PTY (see setupDone), and whether that
// program has ever stopped drawing (see Ready).
func (s *Service) noteOutput(sess *session, chunk []byte) {
	s.mu.Lock()
	defer s.mu.Unlock()
	// The quiet is recorded as it passes, never sampled when somebody finally
	// asks. A session settles seconds after it starts and is asked hours later,
	// by then mid-turn: sampling there reads a spinner redrawing every few
	// frames and calls a session that has been at its prompt all day unready.
	//
	// A setup script pauses too — between one package and the next — and that
	// quiet belongs to a program the provider has not replaced yet, so it is
	// not the provider's and does not count.
	if !sess.settingUp && !sess.lastOut.IsZero() && time.Since(sess.lastOut) >= readySettle {
		sess.ready = true
	}
	sess.lastOut = time.Now()
	if sess.settingUp && sess.setupEnded(chunk) {
		sess.settingUp = false
		// The provider starts drawing from here, so the quiet this waits for is
		// measured from the exec, not from the setup script's last line. The
		// clock goes with the flag: the exec itself takes a second or two — the
		// image is replaced, a runtime starts, a splash is composed — and that
		// silence is nobody's quiet. Measured from the script's last line it
		// would clear the settle on the provider's very first byte, mid-splash,
		// which is the write that lands on screen as literal paste markers.
		sess.ready = false
		sess.lastOut = time.Time{}
	}
}

// setupEnded reports whether chunk completes the wrapper's end marker, matching
// across the seam between two reads: the wrapper prints the marker in one write,
// but a PTY read can cut it anywhere, and neither half matches on its own — a
// session that missed it waits out every relay it is ever sent.
//
// Called under s.mu.
func (sess *session) setupEnded(chunk []byte) bool {
	joined := sess.setupTail + string(chunk)
	if strings.Contains(joined, setupDone) {
		sess.setupTail = ""
		return true
	}
	// One byte short of the marker is the most that can still be its beginning.
	if keep := len(setupDone) - 1; len(joined) > keep {
		joined = joined[len(joined)-keep:]
	}
	sess.setupTail = joined
	return false
}

// Ready reports whether a session can be given work — whether what reads this
// PTY is the provider rather than the project's setup script, and whether the
// prompt is free rather than half-filled by the person sitting at it. False for
// a session that is not running at all.
//
// Live is not this question. A session whose checkout is still installing its
// dependencies has a PTY, appears in the roster, and accepts writes that go
// straight into `pnpm install`'s stdin and are discarded before the provider
// ever starts. It looked delivered to everyone involved: the sender waited out
// its ticket on an agent that was never asked anything.
func (s *Service) Ready(id string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	sess, ok := s.sessions[id]
	if !ok || sess.settingUp {
		return false
	}
	// Unlike the rest of this, being ready is not a state a session reaches and
	// keeps: the user starts typing and the prompt is theirs again until they
	// send it (see draft.go).
	if sess.drafting(time.Now()) {
		return false
	}
	if sess.ready {
		return true
	}
	// A TUI that has stopped drawing is one that has taken the terminal and is
	// waiting on input. Before that, a message is written into a program still
	// setting the tty up, which discards what it finds there — the bracketed
	// paste then lands on screen as literal text, ahead of a prompt that never
	// received it.
	//
	// Quiet right now answers it too, for the session that has not produced a
	// byte since its last one; a quiet that passed earlier was recorded when it
	// happened (see noteOutput). Either way the session stays ready from then
	// on. A busy agent draws continuously, and a target mid-turn has always
	// been written to — its provider queues the input and answers a turn later.
	if !sess.lastOut.IsZero() && time.Since(sess.lastOut) >= readySettle {
		sess.ready = true
		return true
	}
	return false
}

// Close terminates a session's shell, if any.
func (s *Service) Close(id string) error {
	s.mu.Lock()
	sess, ok := s.sessions[id]
	if ok {
		delete(s.sessions, id)
		close(sess.done)
	}
	s.mu.Unlock()

	if !ok {
		return nil
	}
	return sess.pty.Close()
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
