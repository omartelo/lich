package chromium

import (
	"bytes"
	"errors"
	"io"
	"os/exec"
	"slices"
	"strings"
	"time"
)

// A healthy window already writes Fontconfig, GPU and D-Bus warnings, and
// lich.log is rotated at 5 MiB, so the window's stderr is never logged as it
// comes: only a launch that fails reports it, and only this much.
const (
	// tailLines is how much of the end of the stream a failed launch logs.
	tailLines = 20
	// lineBytes cuts a line that never ends, so a tail stays the size its line
	// count promises.
	lineBytes = 1024
	// whyLines bounds the lines that say why the window died: the FATAL lines
	// kept once they scroll out of the tail, and what Why hands a dialog.
	whyLines = 3
)

// stderrDrain bounds how long Wait holds on to the window's stderr after the
// window has exited: its subprocesses inherit the pipe, and Wait returns only
// once they let go of it. Measured, they did within 10 ms of both a clean exit
// and a FATAL one.
const stderrDrain = 2 * time.Second

// gap stands where a tail dropped lines between two it kept.
const gap = "..."

// stderrTail passes the window's stderr through to lich's own and keeps the
// end of it. Only exec's copying goroutine writes, and Lines is read after
// Wait has joined that goroutine, so there is no lock.
type stderrTail struct {
	mirror  io.Writer
	partial []byte
	// head holds the FATAL lines that scrolled out of lines, first ones first,
	// with a gap wherever lines between them were dropped.
	head  []string
	fatal int
	lines []string
}

func (t *stderrTail) Write(p []byte) (int, error) {
	// A terminal that is not there (a launcher start, the windowsgui build)
	// must not fail the copy: exec would report a window that closed cleanly
	// as one that failed.
	_, _ = t.mirror.Write(p)
	n := len(p)
	for {
		i := bytes.IndexByte(p, '\n')
		if i < 0 {
			t.buffer(p)
			return n, nil
		}
		t.buffer(p[:i])
		t.push(string(t.partial))
		t.partial = t.partial[:0]
		p = p[i+1:]
	}
}

func (t *stderrTail) buffer(b []byte) {
	room := max(lineBytes-len(t.partial), 0)
	t.partial = append(t.partial, b[:min(len(b), room)]...)
}

func (t *stderrTail) push(line string) {
	line = strings.TrimRight(line, "\r")
	if line == "" {
		return
	}
	t.lines = append(t.lines, line)
	if len(t.lines) <= tailLines {
		return
	}
	t.drop(t.lines[0])
	t.lines = t.lines[1:]
}

// drop keeps a FATAL line leaving the tail while there is room for one: the
// browser aborts on its first, so that one is the cause, and what follows it
// is the consequence (subprocesses dying, a crash dump) that would otherwise
// push it out.
func (t *stderrTail) drop(line string) {
	if isFatal(line) && t.fatal < whyLines {
		t.head = append(t.head, line)
		t.fatal++
		return
	}
	if len(t.head) > 0 && t.head[len(t.head)-1] == gap {
		return
	}
	t.head = append(t.head, gap)
}

// exit is the window's exit as launch reports it: nil for a window that closed
// cleanly, the exit and the tail otherwise. Wait reports a drain that ran out
// only after a clean exit, so ErrWaitDelay is a closed window with a subprocess
// that held the pipe past stderrDrain.
func (t *stderrTail) exit(err error) error {
	if err == nil || errors.Is(err, exec.ErrWaitDelay) {
		return nil
	}
	return &ExitError{Err: err, Stderr: t.Lines()}
}

// Lines is what the window wrote to stderr, as far as the tail kept it.
func (t *stderrTail) Lines() []string {
	lines := append(slices.Clone(t.head), t.lines...)
	if last := strings.TrimRight(string(t.partial), "\r"); last != "" {
		lines = append(lines, last)
	}
	return lines
}

// isFatal is a line Chromium logs before it aborts. A failed CHECK is logged
// at FATAL too; "Check failed" also catches one printed without the prefix.
func isFatal(line string) bool {
	return strings.Contains(line, ":FATAL:") || strings.Contains(line, "Check failed")
}

// ExitError is the window exiting with an error, with the end of what it wrote
// to stderr: a launcher start has no terminal, so without it the log holds the
// signal the window died of and never the reason.
type ExitError struct {
	Err    error
	Stderr []string
}

func (e *ExitError) Error() string {
	return strings.Join(append([]string{e.Err.Error()}, e.Stderr...), "\n")
}

func (e *ExitError) Unwrap() error { return e.Err }

// Why is err short enough for a dialog: an ExitError keeps only the lines that
// say why, its FATAL lines when it has any and its last lines otherwise.
func Why(err error) string {
	var exit *ExitError
	if !errors.As(err, &exit) {
		return err.Error()
	}
	why := slices.DeleteFunc(slices.Clone(exit.Stderr), func(line string) bool { return !isFatal(line) })
	if len(why) == 0 {
		why = exit.Stderr[max(len(exit.Stderr)-whyLines, 0):]
	}
	return strings.Join(append([]string{exit.Err.Error()}, why[:min(len(why), whyLines)]...), "\n")
}
