package terminal

import (
	"path/filepath"
	"strings"
)

// ptySpec describes the child process a session's PTY runs. It carries
// everything the platform needs to spawn: ConPTY has no exec.Cmd (CreateProcess
// wants one command-line string plus explicit dir/env — see go#62708), so the
// seam speaks in these primitives and each OS builds its own process from them.
type ptySpec struct {
	bin        string
	args       []string
	dir        string
	env        []string
	cols, rows int
}

// ptyHandle is a running session's PTY end, the seam between the service and
// the platform PTY API: Read streams the child's output, Write delivers
// input, Resize changes the window size, Pid identifies the child (0 when
// unknown), Wait reaps the exited child and Close hangs up and terminates it.
// Each OS provides startPTY(spec) plus an implementation of this interface
// (build tags select the file, the Go idiom for OS-specific code) —
// terminal.go never touches a platform PTY API directly.
type ptyHandle interface {
	Read(p []byte) (int, error)
	Write(p []byte) (int, error)
	Resize(cols, rows int) error
	Pid() int
	Wait() error
	Close() error
}

// wrapArgv builds the argv for an already-resolved binary path. npm ships
// Claude Code as claude.cmd, and Windows' CreateProcess runs neither .cmd nor
// .bat directly — only cmd.exe can — so a script binary is prefixed with
// `cmd.exe /c`; native executables pass through unchanged. It lives here, not
// in the Windows PTY file, so the decision stays pure and testable off-Windows
// (filepath.Ext is lexical); the OS boundaries — LookPath and
// ComposeCommandLine — stay in pty_windows.go.
func wrapArgv(path string, args []string) []string {
	argv := append([]string{path}, args...)
	switch strings.ToLower(filepath.Ext(path)) {
	case ".cmd", ".bat":
		argv = append([]string{"cmd.exe", "/c"}, argv...)
	}
	return argv
}
