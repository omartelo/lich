package terminal

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
// input, Resize changes the window size, Wait reaps the exited child and
// Close hangs up and terminates it. Each OS provides startPTY(spec) plus an
// implementation of this interface (build tags select the file, the Go idiom
// for OS-specific code) — terminal.go never touches a platform PTY API
// directly.
type ptyHandle interface {
	Read(p []byte) (int, error)
	Write(p []byte) (int, error)
	Resize(cols, rows int) error
	Wait() error
	Close() error
}
