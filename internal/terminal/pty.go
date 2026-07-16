package terminal

// ptyHandle is a running session's PTY end, the seam between the service and
// the platform PTY API: Read streams the child's output, Write delivers
// input, Resize changes the window size and Close hangs up. Each OS provides
// startPTY plus an implementation of this interface (build tags select the
// file, the Go idiom for OS-specific code) — terminal.go never touches a
// platform PTY API directly.
type ptyHandle interface {
	Read(p []byte) (int, error)
	Write(p []byte) (int, error)
	Resize(cols, rows int) error
	Close() error
}
