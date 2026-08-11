package store

import (
	"testing"

	"github.com/omartelo/lich/internal/providers"
)

func TestSettingGlobalAndProjectScope(t *testing.T) {
	svc := newTestStore(t)

	if got, err := svc.GetSetting(claudeBinKey, globalScope); err != nil || got != "" {
		t.Fatalf("missing setting = (%q, %v), want (\"\", nil)", got, err)
	}

	if err := svc.SetSetting(claudeBinKey, globalScope, "/usr/bin/claude"); err != nil {
		t.Fatalf("SetSetting global: %v", err)
	}
	if err := svc.SetSetting(claudeBinKey, "p1", "/opt/claude-dev"); err != nil {
		t.Fatalf("SetSetting project: %v", err)
	}

	// Global and project rows coexist under the same key.
	if got, _ := svc.GetSetting(claudeBinKey, globalScope); got != "/usr/bin/claude" {
		t.Errorf("global = %q", got)
	}
	if got, _ := svc.GetSetting(claudeBinKey, "p1"); got != "/opt/claude-dev" {
		t.Errorf("project = %q", got)
	}

	// Upsert overwrites in place rather than duplicating.
	if err := svc.SetSetting(claudeBinKey, globalScope, "/usr/local/bin/claude"); err != nil {
		t.Fatalf("SetSetting overwrite: %v", err)
	}
	if got, _ := svc.GetSetting(claudeBinKey, globalScope); got != "/usr/local/bin/claude" {
		t.Errorf("overwritten global = %q", got)
	}
}

func TestClaudeProviderBinResolution(t *testing.T) {
	svc := newTestStore(t)

	// Nothing configured: empty, so the terminal applies its own default.
	if got := svc.ProviderBin(providers.Claude, "p1"); got != "" {
		t.Errorf("unconfigured = %q, want \"\"", got)
	}

	// Global only: every project resolves to it.
	_ = svc.SetSetting(claudeBinKey, globalScope, "global-claude")
	if got := svc.ProviderBin(providers.Claude, "p1"); got != "global-claude" {
		t.Errorf("global fallback = %q, want global-claude", got)
	}

	// Project override wins over global for that project only.
	_ = svc.SetSetting(claudeBinKey, "p1", "p1-claude")
	if got := svc.ProviderBin(providers.Claude, "p1"); got != "p1-claude" {
		t.Errorf("p1 override = %q, want p1-claude", got)
	}
	if got := svc.ProviderBin(providers.Claude, "p2"); got != "global-claude" {
		t.Errorf("p2 = %q, want global-claude", got)
	}
}

func TestGHAccountForPath(t *testing.T) {
	svc := newTestStore(t)
	if err := svc.AddProject("p1", "lich", "/repo/lich"); err != nil {
		t.Fatalf("AddProject: %v", err)
	}
	// A worktree lives outside the project directory; only its session row ties
	// the path back to the project.
	if err := svc.AddSession("p1", "s1", "fix", "claude", "/data/worktrees/p1/fix", 2); err != nil {
		t.Fatalf("AddSession: %v", err)
	}
	if err := svc.SetSetting(ghAccountKey, "p1", "octocat"); err != nil {
		t.Fatalf("SetSetting: %v", err)
	}

	tests := []struct {
		name string
		path string
		want string
	}{
		{"project directory", "/repo/lich", "octocat"},
		{"worktree of the project", "/data/worktrees/p1/fix", "octocat"},
		{"unknown path", "/elsewhere", ""},
		{"empty path", "", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := svc.GHAccountForPath(tt.path); got != tt.want {
				t.Errorf("GHAccountForPath(%q) = %q, want %q", tt.path, got, tt.want)
			}
		})
	}

	// A project with no account configured resolves to "", which runs gh as its
	// own active account.
	if err := svc.AddProject("p2", "other", "/repo/other"); err != nil {
		t.Fatalf("AddProject p2: %v", err)
	}
	if got := svc.GHAccountForPath("/repo/other"); got != "" {
		t.Errorf("unconfigured project = %q, want \"\"", got)
	}
}

func TestProviderBinResolution(t *testing.T) {
	svc := newTestStore(t)

	// A non-claude provider is keyed by "provider.<id>.bin", independent of the
	// legacy claude.bin key, with the same project-over-global fallback.
	if got := svc.ProviderBin("codex", "p1"); got != "" {
		t.Errorf("unconfigured codex = %q, want \"\"", got)
	}
	_ = svc.SetSetting("provider.codex.bin", globalScope, "global-codex")
	if got := svc.ProviderBin("codex", "p1"); got != "global-codex" {
		t.Errorf("global codex = %q, want global-codex", got)
	}
	_ = svc.SetSetting("provider.codex.bin", "p1", "p1-codex")
	if got := svc.ProviderBin("codex", "p1"); got != "p1-codex" {
		t.Errorf("p1 codex override = %q, want p1-codex", got)
	}

	// Claude routes through the legacy key, so both scopes read the same value.
	_ = svc.SetSetting(claudeBinKey, globalScope, "global-claude")
	if got := svc.ProviderBin("claude", "p2"); got != "global-claude" {
		t.Errorf("ProviderBin claude = %q, want global-claude", got)
	}
}

// TestSkipPermissionsIsScopedByCheckout pins the two scopes apart: the flag a
// session in the project's own directory reads is not the one a session in a
// worktree reads, so turning it on for throwaway checkouts leaves the main
// working tree prompting.
func TestSkipPermissionsIsScopedByCheckout(t *testing.T) {
	svc := newTestStore(t)
	if err := svc.AddProject("p1", "alpha", "/repo/alpha"); err != nil {
		t.Fatalf("AddProject: %v", err)
	}

	// Nothing configured: both scopes prompt.
	if svc.SkipPermissions(providers.Claude, "p1", "/repo/alpha") {
		t.Error("unconfigured project directory skips permissions, want prompts")
	}
	if svc.SkipPermissions(providers.Claude, "p1", "/repo/alpha-feature") {
		t.Error("unconfigured worktree skips permissions, want prompts")
	}

	_ = svc.SetSetting("provider.claude.skip-permissions.worktree", globalScope, "true")
	if !svc.SkipPermissions(providers.Claude, "p1", "/repo/alpha-feature/") {
		t.Error("worktree does not skip permissions, want it on")
	}
	if svc.SkipPermissions(providers.Claude, "p1", "/repo/alpha/") {
		t.Error("project directory follows the worktree flag, want its own")
	}

	// A project lich cannot place answers with the main-checkout flag, never the
	// worktree one it just read as true.
	if svc.SkipPermissions(providers.Claude, "gone", "/repo/alpha-feature") {
		t.Error("unknown project reads the worktree flag, want the main-checkout one")
	}

	_ = svc.SetSetting("provider.claude.skip-permissions", globalScope, "true")
	if !svc.SkipPermissions(providers.Claude, "p1", "/repo/alpha") {
		t.Error("project directory does not skip permissions, want it on")
	}

	// Only the literal "true" arms it — any other stored value prompts.
	_ = svc.SetSetting("provider.claude.skip-permissions", globalScope, "1")
	if svc.SkipPermissions(providers.Claude, "p1", "/repo/alpha") {
		t.Error("value \"1\" skips permissions, want only \"true\" to")
	}
}

// TestWorktreeSetupIsProjectScoped pins the setup script the terminal service
// reads when it spawns the first session of a fresh worktree. Project-scoped
// only: a global value must never leak into a project that configured none,
// because the script it would run is another repository's.
func TestWorktreeSetupIsProjectScoped(t *testing.T) {
	svc := newTestStore(t)

	if got := svc.WorktreeSetup("p1"); got != "" {
		t.Errorf("unconfigured project = %q, want \"\"", got)
	}
	if err := svc.SetSetting(worktreeSetupKey, "p1", "pnpm install"); err != nil {
		t.Fatalf("SetSetting: %v", err)
	}
	if got := svc.WorktreeSetup("p1"); got != "pnpm install" {
		t.Errorf("p1 setup = %q, want \"pnpm install\"", got)
	}

	_ = svc.SetSetting(worktreeSetupKey, globalScope, "make bootstrap")
	if got := svc.WorktreeSetup("p2"); got != "" {
		t.Errorf("p2 setup = %q, want \"\" — a global value must not apply", got)
	}
}
