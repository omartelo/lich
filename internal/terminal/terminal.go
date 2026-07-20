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
	"strconv"
	"strings"
	"sync"

	"github.com/omartelo/lich/internal/events"
	"github.com/omartelo/lich/internal/providers"
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
	// exitEventPrefix is emitted once when a session's shell process exits.
	exitEventPrefix = "terminal:exit:"
	// statusEventName carries a session's Claude Code processing state
	// ({id, state} — "busy"/"done"/"waiting"/"idle"), reported by the lich hook
	// running inside the PTY (see transport.hook and docs/hooks/session-state.md).
	// The frontend keeps it in a store keyed by id (session-status-store.ts)
	// rather than in the card, which is only mounted while its project is active.
	statusEventName = "session-status"
	// titleEventName carries an auto-applied session label ({id, label}).
	titleEventName = "session-title"
	// touchedEventName carries the id of a session that likely changed files on
	// disk, nudging an immediate git-status refresh ahead of the steady poll.
	touchedEventName = "session-touched"
)

// statusEvent is the payload of statusEventName: the session whose Claude Code
// processing state changed, and the new state.
type statusEvent struct {
	ID    string `json:"id"`
	State string `json:"state"`
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

// session is a single running PTY-backed shell. done closes when the session
// is reaped (by stream or Close — whichever removes it from the map), stopping
// its cwd watcher.
type session struct {
	pty  ptyHandle
	out  *coalescer
	done chan struct{}
}

// Store is the persistence the terminal service depends on: the binary to spawn
// for a provider in a project (empty return spawns the provider's default), and
// where to record the Claude session id a PTY reports through the SessionStart
// hook. The store implements both.
type Store interface {
	ProviderBin(providerID, projectID string) string
	SetClaudeSession(sessionID, claudeSessionID string) error
	SetSessionTitle(sessionID, title string) (bool, error)
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
		env:      append(childEnv(env), "TERM=xterm-256color"),
	}
	ws, err := newTransport(
		func(id string, data []byte) {
			if err := s.writeBytes(id, data); err != nil {
				slog.Warn("terminal: input write failed", "session", id, "err", err)
			}
		},
		func(id, state string) {
			hub.Emit(statusEventName, statusEvent{ID: id, State: state})
		},
		store.SetClaudeSession,
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
	if err == nil {
		s.ws = ws
	}
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

// SetRestart wires the POST /restart endpoint to fn, the in-place relaunch the
// update flow triggers after replacing the binary. No-op when the transport
// failed to start — /restart then simply reports unavailable.
func (s *Service) SetRestart(fn func() error) {
	if s.ws == nil {
		return
	}
	s.ws.setRestart(fn)
}

// sessionEnv is the environment for one PTY: the shared base plus the loopback
// coordinates a Claude Code hook needs to report this session's status back to
// lich. LICH_SESSION_ID is per-session, so this returns a fresh slice rather
// than aliasing (and appending to) the shared s.env. When the transport failed
// to start there is nowhere to report, so the base env is used unchanged — a
// hook spawned in this PTY sees no LICH_PORT and no-ops.
func (s *Service) sessionEnv(id string) []string {
	if s.ws == nil {
		return s.env
	}
	env := make([]string, len(s.env), len(s.env)+3)
	copy(env, s.env)
	return append(env,
		"LICH_PORT="+strconv.Itoa(s.ws.port),
		"LICH_TOKEN="+s.ws.token,
		"LICH_SESSION_ID="+id,
	)
}

// defaultBin is the binary spawned when the session's kind is not a known
// provider and no custom path is set — a safety net; real sessions always carry
// a registered provider kind.
const defaultBin = providers.Claude

// KindShell marks a session that runs the user's shell instead of a provider.
const KindShell = "shell"

// readBufSize is the chunk size read from a session's PTY per iteration.
const readBufSize = 32 * 1024

// resolveBin returns the configured binary, or the provider's default when it is
// empty (falling back to defaultBin for an unknown kind).
func resolveBin(kind, bin string) string {
	if bin != "" {
		return bin
	}
	if def := providers.DefaultBinary(kind); def != "" {
		return def
	}
	return defaultBin
}

// resumeFlag is the Claude Code flag that reopens an existing session by id.
const resumeFlag = "--resume"

// resumeArgs returns the arguments that reopen a Claude session, or nil when the
// session must start fresh. Resume is Claude-specific: "--resume" is Claude
// Code's flag, so a shell or any other provider never grows it (the frontend
// only ever passes a resume id for a claude session, but a stray one must not
// reach codex/opencode/crush either).
func resumeArgs(kind, resume string) []string {
	if kind != providers.Claude || resume == "" {
		return nil
	}
	return []string{resumeFlag, resume}
}

// resolveCommand picks the binary a session runs: the user's shell for "shell"
// sessions, otherwise the provider binary for the session's kind.
func resolveCommand(kind, bin, shellEnv string) string {
	if kind == KindShell {
		if shellEnv == "" {
			return defaultShell
		}
		return shellEnv
	}
	return resolveBin(kind, bin)
}

// appImageVars are injected by the AppImage runtime or AppImageLauncher and
// must not leak into the shell: ARGV0 (the .AppImage's invocation name) makes
// mise/asdf-style shims misread the shell as an invalid shim, and the rest
// are runtime internals a child never needs.
var appImageVars = map[string]bool{
	"ARGV0":              true,
	"APPIMAGE":           true,
	"APPDIR":             true,
	"OWD":                true,
	"TARGET_APPIMAGE":    true,
	"REDIRECT_APPIMAGE":  true,
	"DESKTOPINTEGRATION": true,
}

// childEnv returns env cleaned of everything the AppImage runtime injected, so
// spawned shells inherit the environment lich itself was launched with. Outside
// an AppImage (no APPDIR — the deb/rpm/dev case) env is returned unchanged.
// Inside one, appImageVars are dropped and every value is scrubbed of
// colon-separated path entries under the AppImage mount — our AppRun adds
// none, but wrappers like AppImageLauncher may prepend the mount to path
// lists, and a mount path in a child's PATH dies with the parent. Entries the
// user set survive verbatim, so a pre-existing LD_LIBRARY_PATH keeps working.
func childEnv(env []string) []string {
	appdir := ""
	for _, kv := range env {
		if v, ok := strings.CutPrefix(kv, "APPDIR="); ok {
			appdir = v
			break
		}
	}
	if appdir == "" {
		return env
	}
	out := make([]string, 0, len(env))
	for _, kv := range env {
		key, value, _ := strings.Cut(kv, "=")
		if appImageVars[key] {
			continue
		}
		scrubbed, changed := scrubPathList(value, appdir)
		switch {
		case changed && scrubbed == "":
			// The variable only pointed into the mount (AppRun's "${VAR:-}"
			// expansion can also leave a dangling empty entry): drop it.
		case changed:
			out = append(out, key+"="+scrubbed)
		default:
			out = append(out, kv)
		}
	}
	return out
}

// scrubPathList drops colon-separated entries of value that live under dir.
// changed reports whether anything was dropped; values without such entries are
// returned untouched, so non-path variables are never rewritten. When only
// empty entries remain the returned value is "".
func scrubPathList(value, dir string) (string, bool) {
	if !strings.Contains(value, dir) {
		return value, false
	}
	var kept []string
	changed, nonEmpty := false, false
	for entry := range strings.SplitSeq(value, ":") {
		if entry == dir || strings.HasPrefix(entry, dir+"/") {
			changed = true
			continue
		}
		if entry != "" {
			nonEmpty = true
		}
		kept = append(kept, entry)
	}
	if !changed {
		return value, false
	}
	if !nonEmpty {
		return "", true
	}
	return strings.Join(kept, ":"), true
}

// Start spawns the binary for session id under project projectID — the user's
// shell when kind is "shell", otherwise the provider binary for that kind
// resolved from the project's settings (falling back to the provider's default
// on $PATH) — attached to a new PTY sized to cols x rows and rooted at cwd, then
// streams its output to the frontend. An empty cwd defaults to the user's home directory. Starting a
// session that is already running is a no-op.
//
// A non-empty resume is a Claude session id to reopen (`--resume`), which the
// frontend passes after the user accepted the prompt to continue the session
// this card ran before the last restart. An id Claude no longer knows fails in
// the PTY like any other bad invocation — the user sees Claude's own error.
func (s *Service) Start(id, projectID, cwd, kind, resume string, cols, rows int) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, running := s.sessions[id]; running {
		return nil
	}

	if cwd == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return fmt.Errorf("failed to resolve home directory: %w", err)
		}
		cwd = home
	}

	p, err := startPTY(ptySpec{
		bin:  resolveCommand(kind, s.store.ProviderBin(kind, projectID), userShell()),
		args: resumeArgs(kind, resume),
		dir:  cwd,
		env:  s.sessionEnv(id),
		cols: cols,
		rows: rows,
	})
	if err != nil {
		return fmt.Errorf("failed to start pty for %q: %w", id, err)
	}

	out := newCoalescer(func(data []byte) {
		if s.ws != nil && s.ws.send(id, data) {
			return
		}
		encoded := base64.StdEncoding.EncodeToString(data)
		s.hub.Emit(dataEventPrefix+id, encoded)
	}, visibleFlushInterval, hiddenFlushInterval)
	done := make(chan struct{})
	s.sessions[id] = &session{pty: p, out: out, done: done}
	go s.stream(id, p, out)
	// The start directory is reported unconditionally so a respawn overwrites
	// whatever cwd the previous PTY left in the frontend's store.
	s.hub.Emit(cwdEventName, cwdEvent{ID: id, Cwd: cwd})
	go watchCwd(id, p.Pid(), cwd, done, s.hub)
	return nil
}

// stream copies PTY output to the frontend until the PTY is closed, then reaps
// the process, drops the session and emits its exit event. Output goes through
// the session's coalescer, which batches it on a short cadence while the
// terminal is visible and a long one while it is hidden.
func (s *Service) stream(id string, p ptyHandle, out *coalescer) {
	buf := make([]byte, readBufSize)
	for {
		n, err := p.Read(buf)
		if n > 0 {
			out.Write(buf[:n])
		}
		if err != nil {
			break
		}
	}
	_ = p.Wait()
	// Flush any batched output before the exit event so the frontend always
	// sees the final bytes ahead of the exit banner.
	out.Close()

	s.mu.Lock()
	if current, ok := s.sessions[id]; ok && current.pty == p {
		delete(s.sessions, id)
		close(current.done)
	}
	s.mu.Unlock()

	s.hub.Emit(exitEventPrefix+id, nil)
}

// Write forwards keyboard input from the frontend to a session's PTY.
func (s *Service) Write(id, data string) error {
	return s.writeBytes(id, []byte(data))
}

// writeBytes delivers input bytes to a session's PTY; unknown sessions are a
// no-op. It is the shared sink for the RPC Write and the WebSocket
// transport's input frames.
func (s *Service) writeBytes(id string, data []byte) error {
	p := s.ptyOf(id)
	if p == nil {
		return nil
	}
	_, err := p.Write(data)
	return err
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

// Resize updates a session's PTY window size. The frontend only calls this for
// the visible terminal; a hidden terminal is resized on the next time it is
// shown.
func (s *Service) Resize(id string, cols, rows int) error {
	p := s.ptyOf(id)
	if p == nil {
		return nil
	}
	return p.Resize(cols, rows)
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

// ptyOf returns the PTY for a session, or nil if it is not running.
func (s *Service) ptyOf(id string) ptyHandle {
	s.mu.Lock()
	defer s.mu.Unlock()
	if sess, ok := s.sessions[id]; ok {
		return sess.pty
	}
	return nil
}
