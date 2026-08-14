package project

import (
	"os"
	"path/filepath"
	"testing"
)

func writeSetup(t *testing.T, dir, script string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Join(dir, ".lich"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, ".lich", "setup-worktree.sh"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
}

// TestSetupScriptReadsTheRepoFile pins the contract the spawn path relies on:
// the script is .lich/setup-worktree.sh in the given checkout, trimmed, and a
// repository without one answers "" — which is what tells wrapSetup not to
// wrap at all.
func TestSetupScriptReadsTheRepoFile(t *testing.T) {
	dir := t.TempDir()
	if got := SetupScript(dir); got != "" {
		t.Errorf("missing file: got %q, want empty", got)
	}
	writeSetup(t, dir, "pnpm install\n")
	if got := SetupScript(dir); got != "pnpm install" {
		t.Errorf("got %q, want %q", got, "pnpm install")
	}
}

// A whitespace-only file must read as "no script": wrapping the spawn to run
// nothing would still make the session wait out a marker for an empty setup.
func TestSetupScriptTreatsBlankFileAsAbsent(t *testing.T) {
	dir := t.TempDir()
	writeSetup(t, dir, "\n  \n")
	if got := SetupScript(dir); got != "" {
		t.Errorf("blank file: got %q, want empty", got)
	}
}

// TestWorktreeSetupPrefersTheFile: with a file present the dialog shows it and
// no suggestion is offered — detection only speaks when nothing is configured.
func TestWorktreeSetupPrefersTheFile(t *testing.T) {
	svc := &Service{}
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "pnpm-lock.yaml"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	writeSetup(t, dir, "task bootstrap")

	got := svc.WorktreeSetup(dir)
	if got.Script != "task bootstrap" {
		t.Errorf("script: got %q, want %q", got.Script, "task bootstrap")
	}
	if got.Suggestion != "" || got.Detected != "" {
		t.Errorf("file present must silence detection, got suggestion %q (%q)", got.Suggestion, got.Detected)
	}
}

// TestWorktreeSetupDetection pins the probe table's visible behaviour: which
// root files speak, what they propose, and that multiple hits chain.
func TestWorktreeSetupDetection(t *testing.T) {
	svc := &Service{}
	tests := []struct {
		name       string
		files      []string
		suggestion string
		detected   string
	}{
		{"nothing", nil, "", ""},
		{"pnpm", []string{"pnpm-lock.yaml"}, "pnpm install", "pnpm-lock.yaml"},
		{"npm", []string{"package-lock.json"}, "npm install", "package-lock.json"},
		{"yarn", []string{"yarn.lock"}, "yarn install", "yarn.lock"},
		{"go", []string{"go.mod"}, "go mod download", "go.mod"},
		{"cargo", []string{"Cargo.lock"}, "cargo fetch", "Cargo.lock"},
		{
			"go backend with js frontend",
			[]string{"go.mod", "pnpm-lock.yaml"},
			"pnpm install && go mod download",
			"pnpm-lock.yaml, go.mod",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			for _, f := range tt.files {
				if err := os.WriteFile(filepath.Join(dir, f), nil, 0o644); err != nil {
					t.Fatal(err)
				}
			}
			got := svc.WorktreeSetup(dir)
			if got.Suggestion != tt.suggestion || got.Detected != tt.detected {
				t.Errorf("got (%q, %q), want (%q, %q)", got.Suggestion, got.Detected, tt.suggestion, tt.detected)
			}
		})
	}
}

// TestSaveWorktreeSetup covers the dialog's Use/Save: the file is created
// with its directory, read back by SetupScript, and an empty save removes it
// instead of pinning an empty setup.
func TestSaveWorktreeSetup(t *testing.T) {
	svc := &Service{}
	dir := t.TempDir()

	if err := svc.SaveWorktreeSetup(dir, "pnpm install"); err != nil {
		t.Fatal(err)
	}
	if got := SetupScript(dir); got != "pnpm install" {
		t.Errorf("after save: got %q, want %q", got, "pnpm install")
	}

	if err := svc.SaveWorktreeSetup(dir, "  "); err != nil {
		t.Fatal(err)
	}
	if got := SetupScript(dir); got != "" {
		t.Errorf("after empty save: got %q, want the file gone", got)
	}
	if _, err := os.Stat(filepath.Join(dir, ".lich", "setup-worktree.sh")); !os.IsNotExist(err) {
		t.Error("empty save must remove the file, not truncate it")
	}
	// Removing what is already absent is a no-op, not an error: Discard-and-
	// save-empty twice must not fail the dialog.
	if err := svc.SaveWorktreeSetup(dir, ""); err != nil {
		t.Errorf("empty save with no file: %v", err)
	}
}
