// Package terminal spawns PTY-backed shell sessions and bridges their I/O to the
// frontend over Wails events. Sessions are keyed by project ID and run
// independently of the frontend, so navigating away from a project (or hiding
// its terminal) never kills its shell.
package terminal

import (
	"encoding/base64"
	"fmt"
	"os"
	"os/exec"
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
}

// Service manages PTY-backed shell sessions keyed by project ID.
type Service struct {
	mu       sync.Mutex
	sessions map[string]*session
}

// New returns a ready-to-use terminal service.
func New() *Service {
	return &Service{sessions: make(map[string]*session)}
}

// defaultShell returns the user's login shell from $SHELL, falling back to a
// sane default when it is unset.
func defaultShell() string {
	if shell := os.Getenv("SHELL"); shell != "" {
		return shell
	}
	return "/bin/sh"
}

// Start spawns the user's shell for project id, attached to a new PTY sized to
// cols x rows and rooted at cwd, then streams its output to the frontend. An
// empty cwd defaults to the user's home directory. Starting a project that is
// already running is a no-op.
func (s *Service) Start(id, cwd string, cols, rows int) error {
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

	cmd := exec.Command(defaultShell())
	cmd.Dir = cwd
	cmd.Env = append(os.Environ(), "TERM=xterm-256color")

	ptmx, err := pty.StartWithSize(cmd, &pty.Winsize{Rows: uint16(rows), Cols: uint16(cols)})
	if err != nil {
		return fmt.Errorf("failed to start pty for %q: %w", id, err)
	}

	s.sessions[id] = &session{ptmx: ptmx, cmd: cmd}
	go s.stream(id, ptmx, cmd)
	return nil
}

// stream copies PTY output to the frontend until the PTY is closed, then reaps
// the process, drops the session and emits its exit event.
func (s *Service) stream(id string, ptmx *os.File, cmd *exec.Cmd) {
	buf := make([]byte, 32*1024)
	for {
		n, err := ptmx.Read(buf)
		if n > 0 {
			encoded := base64.StdEncoding.EncodeToString(buf[:n])
			application.Get().Event.Emit(dataEventPrefix+id, encoded)
		}
		if err != nil {
			break
		}
	}
	_ = cmd.Wait()

	s.mu.Lock()
	if current, ok := s.sessions[id]; ok && current.ptmx == ptmx {
		delete(s.sessions, id)
	}
	s.mu.Unlock()

	application.Get().Event.Emit(exitEventPrefix + id)
}

// Write forwards keyboard input from the frontend to a project's PTY.
func (s *Service) Write(id, data string) error {
	ptmx := s.ptmxOf(id)
	if ptmx == nil {
		return nil
	}
	_, err := ptmx.Write([]byte(data))
	return err
}

// Resize updates a project's PTY window size. The frontend only calls this for
// the visible terminal; a hidden terminal is resized on the next time it is
// shown.
func (s *Service) Resize(id string, cols, rows int) error {
	ptmx := s.ptmxOf(id)
	if ptmx == nil {
		return nil
	}
	return pty.Setsize(ptmx, &pty.Winsize{Rows: uint16(rows), Cols: uint16(cols)})
}

// Close terminates a project's shell session, if any.
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
