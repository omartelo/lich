package store

import "testing"

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

func TestClaudeBinResolution(t *testing.T) {
	svc := newTestStore(t)

	// Nothing configured: empty, so the terminal applies its own default.
	if got := svc.ClaudeBin("p1"); got != "" {
		t.Errorf("unconfigured = %q, want \"\"", got)
	}

	// Global only: every project resolves to it.
	_ = svc.SetSetting(claudeBinKey, globalScope, "global-claude")
	if got := svc.ClaudeBin("p1"); got != "global-claude" {
		t.Errorf("global fallback = %q, want global-claude", got)
	}

	// Project override wins over global for that project only.
	_ = svc.SetSetting(claudeBinKey, "p1", "p1-claude")
	if got := svc.ClaudeBin("p1"); got != "p1-claude" {
		t.Errorf("p1 override = %q, want p1-claude", got)
	}
	if got := svc.ClaudeBin("p2"); got != "global-claude" {
		t.Errorf("p2 = %q, want global-claude", got)
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

	// Claude routes through the legacy key, so ProviderBin and ClaudeBin agree.
	_ = svc.SetSetting(claudeBinKey, globalScope, "global-claude")
	if got := svc.ProviderBin("claude", "p2"); got != "global-claude" {
		t.Errorf("ProviderBin claude = %q, want global-claude", got)
	}
}
