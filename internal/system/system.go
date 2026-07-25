// Package system holds the few OS integrations the frontend needs that are
// not tied to any domain service — opening a URL in the user's default browser
// and opening a work-tree file in their editor.
package system

import (
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

type Service struct {
	// env is the resolved login-shell environment (see terminal.ResolveShellEnv),
	// the source of $VISUAL/$EDITOR — a GUI launch never sourced the user's rc.
	env []string
	// run launches a detached process; injected in tests, exec in production.
	run func(name string, args ...string) error
}

func New(env []string) *Service {
	return &Service{
		env: env,
		run: func(name string, args ...string) error {
			return exec.Command(name, args...).Start()
		},
	}
}

// OpenExternal opens an http(s) URL in the default browser. Scheme-gated so a
// crafted terminal escape can never turn a click into a file:// or custom
// scheme launch.
func (s *Service) OpenExternal(rawURL string) error {
	if err := ValidateExternalURL(rawURL); err != nil {
		return err
	}
	return s.run("xdg-open", rawURL)
}

// OpenInEditor decides how to open a work-tree file. rel is validated against
// traversal, then joined onto dir (mirroring project.ReadFile's guard). It
// prefers $VISUAL, then $EDITOR — resolved from the login shell, so a GUI launch
// still sees rc exports.
//
// When the editor is a terminal editor (vim, nvim, nano, …) it launches nothing
// and returns the shell command line to run in a lich terminal session: a
// detached launch would give it no controlling terminal. Otherwise it launches
// the GUI editor — or, with no editor set, the platform's default opener —
// detached, and returns "". The caller runs the returned command in a terminal
// only when it is non-empty.
func (s *Service) OpenInEditor(dir, rel string) (string, error) {
	if err := validateRel(rel); err != nil {
		return "", err
	}
	full := filepath.Join(dir, rel)
	editor := s.getenv("VISUAL")
	if editor == "" {
		editor = s.getenv("EDITOR")
	}
	if editor != "" && isTerminalEditor(editor) {
		return editor + " " + shellQuote(full), nil
	}
	if editor != "" {
		return "", s.runEditor(editor, full)
	}
	return "", s.openDefault(full)
}

// runEditor launches a GUI $EDITOR value that may carry flags ("code --wait"),
// appending the file as the final argument. An all-whitespace value degrades to
// the default opener rather than launching an empty command.
func (s *Service) runEditor(editor, full string) error {
	fields := strings.Fields(editor)
	if len(fields) == 0 {
		return s.openDefault(full)
	}
	args := append(fields[1:], full)
	return s.run(fields[0], args...)
}

// terminalEditors run inside the terminal; keyed by binary basename. A GUI
// launch gives them no controlling terminal, so lich runs them in a session.
// Ceiling: a fixed list — an unlisted terminal editor is treated as GUI and
// launched detached (and silently fails to open); add it here.
var terminalEditors = map[string]bool{
	"vi": true, "vim": true, "nvim": true, "neovim": true, "nano": true,
	"emacs": true, "emacsclient": true, "helix": true, "hx": true, "kak": true,
	"kakoune": true, "micro": true, "vis": true, "joe": true, "ne": true,
}

// isTerminalEditor reports whether the editor command (which may carry flags,
// e.g. "nvim -p") names a terminal editor, matched on the binary's basename.
func isTerminalEditor(editor string) bool {
	fields := strings.Fields(editor)
	if len(fields) == 0 {
		return false
	}
	return terminalEditors[filepath.Base(fields[0])]
}

// shellQuote wraps s in single quotes for a POSIX shell, escaping embedded
// single quotes — the command runs in the session's shell, so the file path (the
// caller-influenced part) must survive spaces and metacharacters. Assumes an
// sh/bash/zsh-style shell; cmd.exe (experimental Windows) quotes differently.
func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

// getenv reads a key from the resolved shell env, "" when absent.
func (s *Service) getenv(key string) string {
	prefix := key + "="
	for _, kv := range s.env {
		if strings.HasPrefix(kv, prefix) {
			return kv[len(prefix):]
		}
	}
	return ""
}

// validateRel rejects rooted paths and parent-directory escapes, so a joined
// path can never leave the work-tree root. Mirrors project.validateRelPath.
func validateRel(rel string) error {
	clean := filepath.Clean(rel)
	if isRooted(clean) || clean == ".." ||
		strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return fmt.Errorf("invalid repository path %q", rel)
	}
	return nil
}

// isRooted reports whether a cleaned path starts at a root. IsAbs alone is not
// enough on Windows, where it demands a volume name and so reads "/etc/passwd"
// as relative; a work-tree path is always relative, so a leading separator is
// refused too (os.IsPathSeparator counts "\" only where it is one).
func isRooted(clean string) bool {
	return filepath.IsAbs(clean) || os.IsPathSeparator(clean[0])
}

// ValidateExternalURL accepts absolute http/https URLs only.
func ValidateExternalURL(rawURL string) error {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return fmt.Errorf("invalid url: %w", err)
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return fmt.Errorf("refusing to open scheme %q", parsed.Scheme)
	}
	if parsed.Host == "" {
		return fmt.Errorf("refusing to open url without host")
	}
	return nil
}
