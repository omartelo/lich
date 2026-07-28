// Package system holds the few OS integrations the frontend needs that are
// not tied to any domain service — opening a URL in the user's default browser
// and opening a work-tree file in their editor.
package system

import (
	"fmt"
	"net/url"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/omartelo/lich/internal/relpath"
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
// scheme launch. The launcher itself is per-OS (see the open_* files).
func (s *Service) OpenExternal(rawURL string) error {
	if err := ValidateExternalURL(rawURL); err != nil {
		return err
	}
	return s.openURL(rawURL)
}

// OpenInEditor decides how to open a work-tree file. rel is validated against
// traversal (internal/relpath, the same guard project.ReadFile answers to), then
// joined onto dir. It prefers $VISUAL, then $EDITOR — resolved from the login
// shell, so a GUI launch still sees rc exports.
//
// When the editor is a terminal editor (vim, nvim, nano, …) it launches nothing
// and returns the shell command line to run in a lich terminal session: a
// detached launch would give it no controlling terminal. Otherwise it launches
// the GUI editor — or, with no editor set, the platform's default opener —
// detached, and returns "". The caller runs the returned command in a terminal
// only when it is non-empty.
func (s *Service) OpenInEditor(dir, rel string) (string, error) {
	if err := relpath.Validate(rel); err != nil {
		return "", err
	}
	full := filepath.Join(dir, rel)
	editor := s.getenv("VISUAL")
	if editor == "" {
		editor = s.getenv("EDITOR")
	}
	if editor != "" && isTerminalEditor(editor) {
		// The command runs in the session's shell, so the file path — the
		// caller-influenced part — must survive spaces and metacharacters. The
		// quoting rule is the shell's, and the session's shell is per-OS (see
		// the open_* files). A path that shell cannot express at all opens with
		// the default handler instead: a line that would run something else is
		// worse than not using the editor.
		quoted, ok := s.quoteForShell(full)
		if !ok {
			return "", s.openDefault(full)
		}
		return editor + " " + quoted, nil
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

// quoteCmdPath renders full as one argument of a cmd.exe command line, and
// reports false when cmd cannot express it at all. Double quotes cover the
// command separators (& | < >), the grouping parentheses, ^ and spaces — but
// cmd has no escape for three cases, so they are refused rather than mangled:
//
//   - a literal " ends the quoted run and nothing puts one back;
//   - % is expanded inside quotes too, so a defined variable's name between
//     percents silently becomes a different path;
//   - a trailing backslash escapes the closing quote for the target's own argv
//     parser. filepath.Join cleans that away before we get here, so this is the
//     belt to the caller's braces.
//
// Kept out of the build-tagged file so the rule is tested on any OS.
func quoteCmdPath(full string) (string, bool) {
	if strings.ContainsAny(full, `"%`) || strings.HasSuffix(full, `\`) {
		return "", false
	}
	return `"` + full + `"`, true
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
