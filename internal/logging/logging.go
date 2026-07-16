// Package logging configures the process-wide slog logger: structured text
// with source file:line on every record, mirrored to stderr and a persistent
// file under the lich config dir. The file is the audit trail — it is what
// remains on Windows once the GUI-subsystem build drops the console, and what
// a bug report quotes. The session token is never logged: the file outlives
// the session that would have made it useful.
package logging

import (
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
)

// maxLogSize is the size at which the log is rotated on startup. One previous
// generation is kept (*.old); lich emits little, so two generations outlive
// any debugging session.
const maxLogSize = 5 << 20

// Init points slog's default logger at stderr plus <dir>/lich.log
// (lich-dev.log under LICH_DEV, mirroring the store's database split), with
// the level taken from LICH_LOG_LEVEL (debug|warn|error; default info). The
// returned closer owns the file. When the file cannot be opened, logging
// still works on stderr alone: the closer is nil and the error says why the
// persistent half is missing — the caller reports it and carries on.
func Init(dir string) (io.Closer, error) {
	file, err := openLogFile(dir)
	out := io.Writer(os.Stderr)
	var closer io.Closer
	if err == nil {
		// File first: it is the record of truth, and io.MultiWriter stops at
		// the first failing writer. The console mirror is best-effort — the
		// windowsgui build has no stderr at all, and its write failures must
		// never poison the file half.
		out = io.MultiWriter(file, bestEffort{os.Stderr})
		closer = file
	}
	slog.SetDefault(slog.New(slog.NewTextHandler(out, &slog.HandlerOptions{
		AddSource: true,
		Level:     parseLevel(os.Getenv("LICH_LOG_LEVEL")),
	})))
	return closer, err
}

// bestEffort forwards writes and swallows failures — for mirror destinations
// whose loss must not fail the write (a console that does not exist).
type bestEffort struct {
	w io.Writer
}

func (b bestEffort) Write(p []byte) (int, error) {
	_, _ = b.w.Write(p)
	return len(p), nil
}

// openLogFile opens the append-mode log file, first rotating the previous
// generation out of the way once it outgrew maxLogSize.
func openLogFile(dir string) (*os.File, error) {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, fmt.Errorf("create log dir: %w", err)
	}
	path := filepath.Join(dir, fileName(os.Getenv("LICH_DEV") != ""))
	if info, err := os.Stat(path); err == nil && info.Size() >= maxLogSize {
		if err := os.Rename(path, path+".old"); err != nil {
			return nil, fmt.Errorf("rotate log: %w", err)
		}
	}
	file, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return nil, fmt.Errorf("open log file: %w", err)
	}
	return file, nil
}

// fileName mirrors the store's database split: a `task dev` session logs
// beside the dev DB and never pollutes the daily driver's audit trail.
func fileName(dev bool) string {
	if dev {
		return "lich-dev.log"
	}
	return "lich.log"
}

// parseLevel maps LICH_LOG_LEVEL onto slog levels. Unknown or empty values
// fall back to Info: an audit log that silently went quiet would be worse
// than a slightly noisy one.
func parseLevel(s string) slog.Level {
	switch strings.ToLower(s) {
	case "debug":
		return slog.LevelDebug
	case "warn":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}
