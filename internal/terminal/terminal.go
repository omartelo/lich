// Package terminal spawns PTY-backed shell sessions and bridges their I/O to the
// frontend over Wails events. Sessions are keyed by an opaque session ID and run
// independently of the frontend, so navigating away from a project (or hiding
// its terminal) never kills its shell. A project may own several sessions.
package terminal

import (
	"encoding/base64"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"sync"

	"github.com/creack/pty"
	"github.com/wailsapp/wails/v3/pkg/application"
)

// Event name prefixes. The concrete event carries the session ID as a suffix
// (e.g. "terminal:data:home") so each frontend terminal subscribes only to its
// own stream instead of filtering a global broadcast.
const (
	// dataEventPrefix carries base64-encoded PTY output. Output is base64-encoded
	// because raw PTY bytes may split a multi-byte UTF-8 sequence mid-read, which
	// the JSON event bridge would otherwise corrupt.
	dataEventPrefix = "terminal:data:"
	// exitEventPrefix is emitted once when a session's shell process exits.
	exitEventPrefix = "terminal:exit:"
)

// session is a single running PTY-backed shell.
type session struct {
	ptmx *os.File
	cmd  *exec.Cmd
	out  *coalescer
}

// BinResolver supplies the Claude Code binary path to spawn for a project. An
// empty return spawns the default binary. The store implements it, reading the
// per-project override or the global setting.
type BinResolver interface {
	ClaudeBin(projectID string) string
}

// Service manages PTY-backed shell sessions keyed by session ID.
type Service struct {
	mu       sync.Mutex
	sessions map[string]*session
	bins     BinResolver
	// env is the environment every spawned session inherits: the launch
	// environment cleaned of AppImage runtime leakage (see childEnv), plus TERM.
	env []string
	// ws is the local WebSocket transport for terminal I/O (see transport.go);
	// nil when it failed to start, leaving the Wails event bridge as the path.
	ws *transport
}

// New returns a ready-to-use terminal service that resolves the binary to spawn
// through bins. env is the process environment to derive session environments
// from — callers pass a snapshot taken before any os.Setenv tweaks (main.go
// forces GDK_BACKEND on Linux) so those never leak into spawned shells.
func New(bins BinResolver, env []string) *Service {
	s := &Service{
		sessions: make(map[string]*session),
		bins:     bins,
		env:      append(childEnv(env), "TERM=xterm-256color"),
	}
	ws, err := newTransport(func(id string, data []byte) { _ = s.writeBytes(id, data) })
	if err == nil {
		s.ws = ws
	}
	return s
}

// defaultBin is the Claude Code binary spawned when the user has not configured
// a custom path.
const defaultBin = "claude"

// defaultShell is spawned for "shell" sessions when $SHELL is unset.
const defaultShell = "/bin/sh"

// KindShell marks a session that runs the user's shell instead of Claude Code.
const KindShell = "shell"

// readBufSize is the chunk size read from a session's PTY per iteration.
const readBufSize = 32 * 1024

// resolveBin returns the configured binary, or the default when it is empty.
func resolveBin(bin string) string {
	if bin == "" {
		return defaultBin
	}
	return bin
}

// resolveCommand picks the binary a session runs: the user's shell for "shell"
// sessions, otherwise the configured Claude Code binary.
func resolveCommand(kind, bin, shellEnv string) string {
	if kind == KindShell {
		if shellEnv == "" {
			return defaultShell
		}
		return shellEnv
	}
	return resolveBin(bin)
}

// appImageVars are injected by the AppImage runtime, AppImageLauncher or our
// AppRun (build/linux/appimage/fix-appimage.sh) and must not leak into the
// shell: ARGV0 (the .AppImage's invocation name) makes mise/asdf-style shims
// misread the shell as an invalid shim, the WEBKIT_ pair would run any
// WebKitGTK app launched from the terminal without its sandbox, and the rest
// are runtime internals a child never needs.
var appImageVars = map[string]bool{
	"ARGV0":              true,
	"APPIMAGE":           true,
	"APPDIR":             true,
	"OWD":                true,
	"TARGET_APPIMAGE":    true,
	"REDIRECT_APPIMAGE":  true,
	"DESKTOPINTEGRATION": true,
	"WEBKIT_DISABLE_SANDBOX_THIS_IS_DANGEROUS": true,
	"WEBKIT_DISABLE_DMABUF_RENDERER":           true,
}

// childEnv returns env cleaned of everything the AppImage runtime injected, so
// spawned shells inherit the environment lich itself was launched with. Outside
// an AppImage (no APPDIR — the deb/rpm/dev case) env is returned unchanged.
// Inside one, appImageVars are dropped and every value is scrubbed of
// colon-separated path entries under the AppImage mount — AppRun prepends the
// mount to LD_LIBRARY_PATH, PATH and XDG_DATA_DIRS, and its bundled Ubuntu libs
// break linkers and GTK apps run from the terminal. Entries the user set
// survive verbatim, so a pre-existing LD_LIBRARY_PATH keeps working.
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
// shell when kind is "shell", otherwise the Claude Code binary resolved from the
// project's settings (falling back to "claude" via $PATH) — attached to a new
// PTY sized to cols x rows and rooted at cwd, then streams its output to the
// frontend. An empty cwd defaults to the user's home directory. Starting a
// session that is already running is a no-op.
func (s *Service) Start(id, projectID, cwd, kind string, cols, rows int) error {
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

	cmd := exec.Command(resolveCommand(kind, s.bins.ClaudeBin(projectID), os.Getenv("SHELL")))
	cmd.Dir = cwd
	cmd.Env = s.env

	ptmx, err := pty.StartWithSize(cmd, &pty.Winsize{Rows: uint16(rows), Cols: uint16(cols)})
	if err != nil {
		return fmt.Errorf("failed to start pty for %q: %w", id, err)
	}

	out := newCoalescer(func(data []byte) {
		if s.ws != nil && s.ws.send(id, data) {
			return
		}
		encoded := base64.StdEncoding.EncodeToString(data)
		application.Get().Event.Emit(dataEventPrefix+id, encoded)
	}, visibleFlushInterval, hiddenFlushInterval)
	s.sessions[id] = &session{ptmx: ptmx, cmd: cmd, out: out}
	go s.stream(id, ptmx, cmd, out)
	return nil
}

// stream copies PTY output to the frontend until the PTY is closed, then reaps
// the process, drops the session and emits its exit event. Output goes through
// the session's coalescer, which batches it on a short cadence while the
// terminal is visible and a long one while it is hidden.
func (s *Service) stream(id string, ptmx *os.File, cmd *exec.Cmd, out *coalescer) {
	buf := make([]byte, readBufSize)
	for {
		n, err := ptmx.Read(buf)
		if n > 0 {
			out.Write(buf[:n])
		}
		if err != nil {
			break
		}
	}
	_ = cmd.Wait()
	// Flush any batched output before the exit event so the frontend always
	// sees the final bytes ahead of the exit banner.
	out.Close()

	s.mu.Lock()
	if current, ok := s.sessions[id]; ok && current.ptmx == ptmx {
		delete(s.sessions, id)
	}
	s.mu.Unlock()

	application.Get().Event.Emit(exitEventPrefix + id)
}

// Write forwards keyboard input from the frontend to a session's PTY.
func (s *Service) Write(id, data string) error {
	return s.writeBytes(id, []byte(data))
}

// writeBytes delivers input bytes to a session's PTY; unknown sessions are a
// no-op. It is the shared sink for the Wails binding and the WebSocket
// transport's input frames.
func (s *Service) writeBytes(id string, data []byte) error {
	ptmx := s.ptmxOf(id)
	if ptmx == nil {
		return nil
	}
	_, err := ptmx.Write(data)
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
	ptmx := s.ptmxOf(id)
	if ptmx == nil {
		return nil
	}
	return pty.Setsize(ptmx, &pty.Winsize{Rows: uint16(rows), Cols: uint16(cols)})
}

// Close terminates a session's shell, if any.
func (s *Service) Close(id string) error {
	s.mu.Lock()
	sess, ok := s.sessions[id]
	if ok {
		delete(s.sessions, id)
	}
	s.mu.Unlock()

	if !ok {
		return nil
	}
	err := sess.ptmx.Close()
	if sess.cmd.Process != nil {
		_ = sess.cmd.Process.Kill()
	}
	return err
}

// ptmxOf returns the PTY for a session, or nil if it is not running.
func (s *Service) ptmxOf(id string) *os.File {
	s.mu.Lock()
	defer s.mu.Unlock()
	if sess, ok := s.sessions[id]; ok {
		return sess.ptmx
	}
	return nil
}
