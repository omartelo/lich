// Package terminal spawns a PTY-backed shell and bridges its I/O to the frontend
// over Wails events.
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

// Event names bridging PTY I/O to the frontend.
const (
	// DataEvent carries base64-encoded PTY output. Output is base64-encoded
	// because raw PTY bytes may split a multi-byte UTF-8 sequence mid-read, which
	// the JSON event bridge would otherwise corrupt.
	DataEvent = "terminal:data"
	// ExitEvent is emitted once when the shell process exits.
	ExitEvent = "terminal:exit"
)

// Service spawns a PTY-backed shell and bridges its I/O to the frontend.
//
// ponytail: single session per app window. Add a map[id]*session keyed by a
// terminal ID if concurrent terminals (tabs/splits) are needed later.
type Service struct {
	mu   sync.Mutex
	ptmx *os.File
	cmd  *exec.Cmd
}

// defaultShell returns the user's login shell from $SHELL, falling back to a
// sane default when it is unset.
func defaultShell() string {
	if shell := os.Getenv("SHELL"); shell != "" {
		return shell
	}
	return "/bin/sh"
}

// Start spawns the user's shell attached to a new PTY sized to cols x rows and
// begins streaming its output to the frontend. Calling Start while a session is
// already running is a no-op.
func (s *Service) Start(cols, rows int) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.ptmx != nil {
		return nil
	}

	cmd := exec.Command(defaultShell())
	cmd.Env = append(os.Environ(), "TERM=xterm-256color")

	ptmx, err := pty.StartWithSize(cmd, &pty.Winsize{Rows: uint16(rows), Cols: uint16(cols)})
	if err != nil {
		return fmt.Errorf("failed to start pty: %w", err)
	}

	s.ptmx = ptmx
	s.cmd = cmd

	go s.stream(ptmx, cmd)
	return nil
}

// stream copies PTY output to the frontend until the PTY is closed, then reaps
// the process, clears the session and emits the exit event.
func (s *Service) stream(ptmx *os.File, cmd *exec.Cmd) {
	buf := make([]byte, 32*1024)
	for {
		n, err := ptmx.Read(buf)
		if n > 0 {
			encoded := base64.StdEncoding.EncodeToString(buf[:n])
			application.Get().Event.Emit(DataEvent, encoded)
		}
		if err != nil {
			break
		}
	}
	_ = cmd.Wait()

	s.mu.Lock()
	if s.ptmx == ptmx {
		s.ptmx = nil
		s.cmd = nil
	}
	s.mu.Unlock()

	application.Get().Event.Emit(ExitEvent)
}

// Write forwards keyboard input from the frontend to the PTY.
func (s *Service) Write(data string) error {
	s.mu.Lock()
	ptmx := s.ptmx
	s.mu.Unlock()
	if ptmx == nil {
		return nil
	}
	_, err := ptmx.Write([]byte(data))
	return err
}

// Resize updates the PTY window size when the frontend terminal is resized.
func (s *Service) Resize(cols, rows int) error {
	s.mu.Lock()
	ptmx := s.ptmx
	s.mu.Unlock()
	if ptmx == nil {
		return nil
	}
	return pty.Setsize(ptmx, &pty.Winsize{Rows: uint16(rows), Cols: uint16(cols)})
}

// Close terminates the running shell session, if any.
func (s *Service) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.ptmx == nil {
		return nil
	}
	err := s.ptmx.Close()
	s.ptmx = nil
	if s.cmd != nil && s.cmd.Process != nil {
		_ = s.cmd.Process.Kill()
	}
	s.cmd = nil
	return err
}
