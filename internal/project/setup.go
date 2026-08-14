package project

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// setupScriptPath is the repo-relative path of the worktree setup script: the
// shell that runs in a new worktree's terminal before its provider starts. It
// lives in the repository rather than lich's store so the script is versioned
// and shared like the code it bootstraps; an untracked copy works too.
const setupScriptPath = ".lich/setup-worktree.sh"

// SetupScript returns the project's worktree setup script, or "" when the
// repository ships none. Always read from the main checkout, never from the
// worktree being created: a worktree opened on somebody else's branch (the PR
// flow) must not execute that branch's script.
func SetupScript(projectPath string) string {
	data, err := os.ReadFile(filepath.Join(projectPath, filepath.FromSlash(setupScriptPath)))
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(data))
}

// WorktreeSetup is what the New-worktree dialog shows about a project's setup:
// the script when the repository ships one, otherwise a suggestion detected
// from files at its root. Mirrored in frontend/src/lib/api-types.ts.
type WorktreeSetup struct {
	// Script is the setup file's content; empty when the repository has none.
	Script string `json:"script"`
	// Suggestion is a command proposed from Detected. It never runs on its
	// own — the dialog offers it and only an accepted offer is saved and run.
	Suggestion string `json:"suggestion,omitempty"`
	// Detected names the files behind Suggestion, for the dialog's copy.
	Detected string `json:"detected,omitempty"`
}

// WorktreeSetup reports the setup script the dialog should show for the
// project at path, falling back to a lockfile-detected suggestion.
func (s *Service) WorktreeSetup(path string) WorktreeSetup {
	if script := SetupScript(path); script != "" {
		return WorktreeSetup{Script: script}
	}
	suggestion, detected := detectSetup(path)
	return WorktreeSetup{Suggestion: suggestion, Detected: detected}
}

// SaveWorktreeSetup writes the setup script into the project checkout — the
// dialog's Use/Save actions. The file lands untracked; committing it is the
// user's call. Saving an empty script removes the file instead, returning the
// dialog to the suggestion state rather than pinning an empty setup.
func (s *Service) SaveWorktreeSetup(path, script string) error {
	target := filepath.Join(path, filepath.FromSlash(setupScriptPath))
	if strings.TrimSpace(script) == "" {
		if err := os.Remove(target); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("remove %s: %w", setupScriptPath, err)
		}
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return fmt.Errorf("create %s: %w", filepath.Dir(setupScriptPath), err)
	}
	if err := os.WriteFile(target, []byte(script+"\n"), 0o755); err != nil {
		return fmt.Errorf("write %s: %w", setupScriptPath, err)
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
