package project

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// The repo-relative paths of the two scripts a checkout can ship. They live in
// the repository rather than lich's store so both are versioned and shared like
// the code they bootstrap; an untracked copy works too.
//
// setupScriptPath runs once in a new worktree's terminal before its provider
// starts. runScriptPath is the project's own app — the command a run card opens
// into, in a checkout that already has its dependencies.
const (
	setupScriptPath = ".lich/setup-worktree.sh"
	runScriptPath   = ".lich/run-worktree.sh"
)

// SetupScript returns the project's worktree setup script, or "" when the
// repository ships none. Always read from the main checkout, never from the
// worktree being created: a worktree opened on somebody else's branch (the PR
// flow) must not execute that branch's script.
func SetupScript(projectPath string) string {
	return readScript(projectPath, setupScriptPath)
}

// RunScript returns the project's run script, or "" when the repository ships
// none. Read from the main checkout for the same reason SetupScript is: the
// command belongs to the project, not to whichever branch a worktree holds.
func RunScript(projectPath string) string {
	return readScript(projectPath, runScriptPath)
}

// readScript reads one of the two scripts, answering "" for every failure — a
// script lich cannot read is a script it will not run either.
func readScript(projectPath, rel string) string {
	data, err := os.ReadFile(filepath.Join(projectPath, filepath.FromSlash(rel)))
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(data))
}

// WorktreeSetup is what the New-worktree dialog shows about a project's two
// scripts: the setup one when the repository ships it, otherwise a suggestion
// detected from files at its root, and the run one beside it. Mirrored in
// frontend/src/lib/api-types.ts.
type WorktreeSetup struct {
	// Script is the setup file's content; empty when the repository has none.
	Script string `json:"script"`
	// Run is the run file's content; empty when the repository has none. It has
	// no suggestion beside it — a lockfile names how a checkout installs, never
	// how the project starts.
	Run string `json:"run"`
	// Suggestion is a command proposed from Detected. It never runs on its
	// own — the dialog offers it and only an accepted offer is saved and run.
	Suggestion string `json:"suggestion,omitempty"`
	// Detected names the files behind Suggestion, for the dialog's copy.
	Detected string `json:"detected,omitempty"`
}

// WorktreeSetup reports the setup script the dialog should show for the
// project at path, falling back to a lockfile-detected suggestion.
func (s *Service) WorktreeSetup(path string) WorktreeSetup {
	info := WorktreeSetup{Script: SetupScript(path), Run: RunScript(path)}
	if info.Script == "" {
		info.Suggestion, info.Detected = detectSetup(path)
	}
	return info
}

// SaveWorktreeSetup writes the setup script into the project checkout — the
// dialog's Use/Save actions. The file lands untracked; committing it is the
// user's call. Saving an empty script removes the file instead, returning the
// dialog to the suggestion state rather than pinning an empty setup.
func (s *Service) SaveWorktreeSetup(path, script string) error {
	return writeScript(path, setupScriptPath, script)
}

// SaveWorktreeRun writes the run script into the project checkout, the same way
// SaveWorktreeSetup writes the setup one. Saving an empty script removes the
// file, which is also how a project stops offering a run card.
func (s *Service) SaveWorktreeRun(path, script string) error {
	return writeScript(path, runScriptPath, script)
}

// writeScript is the one write behind both, an empty script removing the file.
func writeScript(path, rel, script string) error {
	target := filepath.Join(path, filepath.FromSlash(rel))
	if strings.TrimSpace(script) == "" {
		if err := os.Remove(target); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("remove %s: %w", rel, err)
		}
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return fmt.Errorf("create %s: %w", filepath.Dir(rel), err)
	}
	if err := os.WriteFile(target, []byte(script+"\n"), 0o755); err != nil {
		return fmt.Errorf("write %s: %w", rel, err)
	}
	return nil
}

// setupProbes maps repository-root files to the bootstrap they imply, in
// suggestion order. Deliberately tiny: the common cases, not a package-manager
// census — anything else is one Edit away.
var setupProbes = []struct{ file, command string }{
	{"pnpm-lock.yaml", "pnpm install"},
	{"package-lock.json", "npm install"},
	{"yarn.lock", "yarn install"},
	{"go.mod", "go mod download"},
	{"Cargo.lock", "cargo fetch"},
}

// detectSetup proposes a setup command from files at the repository root.
// Multiple hits chain with && — a Go backend with a JS frontend wants both.
func detectSetup(path string) (suggestion, detected string) {
	var commands, files []string
	for _, probe := range setupProbes {
		if _, err := os.Stat(filepath.Join(path, probe.file)); err == nil {
			commands = append(commands, probe.command)
			files = append(files, probe.file)
		}
	}
	return strings.Join(commands, " && "), strings.Join(files, ", ")
}
