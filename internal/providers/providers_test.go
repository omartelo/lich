package providers

import (
	"errors"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestDefaultBinary(t *testing.T) {
	cases := map[string]string{
		Claude:      "claude",
		Codex:       "codex",
		Antigravity: "agy",
		OpenCode:    "opencode",
		OMP:         "omp",
		Crush:       "crush",
		Cursor:      "cursor-agent",
		"nope":      "",
		"":          "",
	}
	for id, want := range cases {
		if got := DefaultBinary(id); got != want {
			t.Errorf("DefaultBinary(%q) = %q, want %q", id, got, want)
		}
	}
}

func TestDetect(t *testing.T) {
	// Only claude and crush are "installed"; the fake resolves their binaries to
	// a path and reports the rest missing.
	installed := map[string]string{
		"claude": "/usr/bin/claude",
		"crush":  "/opt/bin/crush",
	}
	svc := &Service{
		lookPath: func(name string) (string, error) {
			if path, ok := installed[name]; ok {
				return path, nil
			}
			return "", exec.ErrNotFound
		},
	}

	got, err := svc.Detect()
	if err != nil {
		t.Fatalf("Detect: %v", err)
	}
	if len(got) != len(Registry) {
		t.Fatalf("Detect returned %d providers, want %d", len(got), len(Registry))
	}
	// Order matches Registry, and install state/path track the fake.
	if got[0].ID != Claude || !got[0].Installed || got[0].Path != "/usr/bin/claude" {
		t.Errorf("claude = %+v, want installed at /usr/bin/claude", got[0])
	}
	if got[1].ID != Codex || got[1].Installed {
		t.Errorf("codex = %+v, want not installed", got[1])
	}
	if got[5].ID != Crush || !got[5].Installed || got[5].Path != "/opt/bin/crush" {
		t.Errorf("crush = %+v, want installed at /opt/bin/crush", got[5])
	}
}

// TestDetectCarriesTheBinary pins the field the settings screen asks $PATH for.
// Antigravity is the case that matters: its id is not its command, so a screen
// falling back to the id verifies a binary nobody ships.
func TestDetectCarriesTheBinary(t *testing.T) {
	svc := &Service{lookPath: func(string) (string, error) { return "", exec.ErrNotFound }}
	got, err := svc.Detect()
	if err != nil {
		t.Fatalf("Detect: %v", err)
	}
	for _, d := range got {
		if d.Binary != DefaultBinary(d.ID) || d.Binary == "" {
			t.Errorf("%s binary = %q, want %q", d.ID, d.Binary, DefaultBinary(d.ID))
		}
	}
	if got[2].ID != Antigravity || got[2].Binary != "agy" {
		t.Errorf("antigravity = %+v, want binary agy", got[2])
	}
}

func TestDetectAllMissing(t *testing.T) {
	svc := &Service{lookPath: func(string) (string, error) { return "", errors.New("nope") }}
	got, err := svc.Detect()
	if err != nil {
		t.Fatalf("Detect: %v", err)
	}
	for _, d := range got {
		if d.Installed || d.Path != "" {
			t.Errorf("%s reported installed with no binary on PATH: %+v", d.ID, d)
		}
	}
}

// TestKnown proves the guard the session-start hook payload passes through:
// every registered id is accepted, and anything else — a provider lich has no
// entry for, the shell kind, an empty string — is not.
func TestKnown(t *testing.T) {
	for _, p := range Registry {
		if !Known(p.ID) {
			t.Errorf("Known(%q) = false, want true for a registered provider", p.ID)
		}
	}
	for _, id := range []string{"", "shell", "gemini", "Claude"} {
		if Known(id) {
			t.Errorf("Known(%q) = true, want false", id)
		}
	}
}

// TestEveryProviderDocumentsItsInstall pins the field the "not found on PATH"
// rows link to. A blank one there is a row that names a missing agent and offers
// nothing, which is the dead end the field exists to close — so provider number
// eight fails here until it brings its page.
func TestEveryProviderDocumentsItsInstall(t *testing.T) {
	for _, p := range Registry {
		if !strings.HasPrefix(p.Docs, "https://") {
			t.Errorf("%s docs = %q, want an https install page", p.ID, p.Docs)
		}
	}
}

// TestDetectCarriesTheDocs: the link travels on the detection result, installed
// or not — the row with somewhere to send the user is the one that found nothing.
func TestDetectCarriesTheDocs(t *testing.T) {
	svc := &Service{lookPath: func(string) (string, error) { return "", exec.ErrNotFound }}
	got, err := svc.Detect()
	if err != nil {
		t.Fatalf("Detect: %v", err)
	}
	for i, d := range got {
		if d.Docs != Registry[i].Docs || d.Docs == "" {
			t.Errorf("%s docs = %q, want %q", d.ID, d.Docs, Registry[i].Docs)
		}
	}
}

// TestSupportsFork pins the three providers that can branch a conversation and,
// more importantly, the five that cannot: a false turning true here is a fork
// flag reaching a CLI with no verb for it, which kills the spawn before the
// session exists. Every id is listed rather than looped over the Registry, so a
// provider added without an answer fails this test instead of inheriting one.
func TestSupportsFork(t *testing.T) {
	cases := map[string]bool{
		Claude:      true,
		Codex:       true,
		OpenCode:    true,
		Antigravity: false,
		OMP:         false,
		Crush:       false,
		Cursor:      false,
		Kiro:        false,
		"shell":     false,
		"nope":      false,
		"":          false,
	}
	for _, p := range Registry {
		if _, listed := cases[p.ID]; !listed {
			t.Errorf("provider %q has no fork answer — add one to this table", p.ID)
		}
	}
	for id, want := range cases {
		if got := SupportsFork(id); got != want {
			t.Errorf("SupportsFork(%q) = %v, want %v", id, got, want)
		}
	}
}

// TestCostSourceOfNamesEveryRung is the table `lich cost` and the footer both
// read the split off. Every registered provider is in it: a new one lands on a
// rung the moment lich can read its spend, and until somebody says which, the
// report would silently file its dollars as nobody's arithmetic.
func TestCostSourceOfNamesEveryRung(t *testing.T) {
	want := map[string]CostSource{
		Claude:      CostSourcePriced,
		Codex:       CostSourcePriced,
		OMP:         CostSourceReported,
		OpenCode:    CostSourceReported,
		Crush:       CostSourceReported,
		Antigravity: CostSourceNone,
		Cursor:      CostSourceNone,
		Kiro:        CostSourceNone,
	}
	for _, p := range Registry {
		rung, named := want[p.ID]
		if !named {
			t.Fatalf("%s is on no rung: name one for it, or the report files its money as nobody's", p.ID)
		}
		if got := CostSourceOf(p.ID); got != rung {
			t.Errorf("CostSourceOf(%q) = %q, want %q", p.ID, got, rung)
		}
	}
	// A session running the user's shell was spawned as no provider at all,
	// whatever CLI the user started inside it.
	if got := CostSourceOf("shell"); got != CostSourceNone {
		t.Errorf("CostSourceOf(shell) = %q, want no rung", got)
	}
}

// TestDetectCountsAConfiguredBinary is the trap this reading exists to close: a
// machine whose only agent is reached through the binary setting used to read as
// a machine with nothing on it, so every implicit new session opened a terminal.
// The three answers a setting can give are pinned together — accepted, rejected,
// absent — because it is the same branch deciding all three.
func TestDetectCountsAConfiguredBinary(t *testing.T) {
	wrapper := configuredPath(t, "claude-wrapper")
	configured := map[string]string{
		Claude: wrapper,
		Codex:  configuredPath(t, "gone"),
	}
	svc := &Service{
		// Nothing at all on PATH: the bare machine the bullet described.
		lookPath: func(name string) (string, error) {
			if name == wrapper {
				return name, nil
			}
			return "", exec.ErrNotFound
		},
		configuredBin: func(id string) string { return configured[id] },
	}

	got, err := svc.Detect()
	if err != nil {
		t.Fatalf("Detect: %v", err)
	}
	if !got[0].Installed || got[0].Path != wrapper {
		t.Errorf("claude = %+v, want installed at the configured path", got[0])
	}
	if got[0].Source != SourceSetting {
		t.Errorf("claude source = %q, want %q", got[0].Source, SourceSetting)
	}
	// A configured binary that does not resolve is still the spawn's answer, so
	// the provider is not installed — and it says nothing about where from.
	if got[1].Installed || got[1].Source != "" {
		t.Errorf("codex = %+v, want not installed with no source", got[1])
	}
	// Nothing configured and nothing on PATH is the machine it always was.
	if got[2].Installed || got[2].Source != "" {
		t.Errorf("antigravity = %+v, want not installed with no source", got[2])
	}
}

// TestDetectPrefersTheConfiguredBinary pins the precedence against the spawn's:
// terminal reads store.ProviderBin before falling back to the provider's own
// command, so a configured binary decides even when $PATH has one — reporting
// the $PATH hit would name a binary no session of this provider would run.
func TestDetectPrefersTheConfiguredBinary(t *testing.T) {
	missing := configuredPath(t, "gone")
	svc := &Service{
		lookPath: func(name string) (string, error) {
			if name == "claude" {
				return "/usr/bin/claude", nil
			}
			return "", exec.ErrNotFound
		},
		configuredBin: func(id string) string {
			if id == Claude {
				return missing
			}
			return ""
		},
	}

	got, err := svc.Detect()
	if err != nil {
		t.Fatalf("Detect: %v", err)
	}
	if got[0].Installed || got[0].Path != "" {
		t.Errorf("claude = %+v, want the broken override to answer, not $PATH", got[0])
	}
}

// TestDetectSourceIsPathWhenNothingIsConfigured keeps the new field honest for
// the machine that has changed nothing: a $PATH hit says so, and a Service with
// no settings reader behind it (`lich doctor`, `lich rage`) scans PATH alone.
func TestDetectSourceIsPathWhenNothingIsConfigured(t *testing.T) {
	svc := &Service{
		lookPath: func(name string) (string, error) {
			if name == "claude" {
				return "/usr/bin/claude", nil
			}
			return "", exec.ErrNotFound
		},
	}

	got, err := svc.Detect()
	if err != nil {
		t.Fatalf("Detect: %v", err)
	}
	if got[0].Source != SourcePath || got[0].Path != "/usr/bin/claude" {
		t.Errorf("claude = %+v, want %q at /usr/bin/claude", got[0], SourcePath)
	}
	if got[1].Source != "" {
		t.Errorf("codex source = %q, want empty when nothing was found", got[1].Source)
	}
}

// TestSetConfiguredBinIsWhatDetectReads proves the seam main.go wires, rather
// than the field a test can set directly: the settings store cannot be reached
// from this package, so a Detect that stopped calling the injected reader would
// pass every test above and ship the bug back.
func TestSetConfiguredBinIsWhatDetectReads(t *testing.T) {
	svc := &Service{lookPath: func(string) (string, error) { return "", exec.ErrNotFound }}
	if got, _ := svc.Detect(); got[0].Installed {
		t.Fatal("claude installed before any binary was configured")
	}

	wrapper := configuredPath(t, "claude")
	svc.SetConfiguredBin(func(id string) string {
		if id == Claude {
			return wrapper
		}
		return ""
	})
	svc.lookPath = func(name string) (string, error) { return name, nil }

	got, _ := svc.Detect()
	if !got[0].Installed || got[0].Source != SourceSetting {
		t.Errorf("claude = %+v, want installed from the setting", got[0])
	}
}

// configuredPath spells what a user types into the binary setting, absolute for
// the OS the test is running on. It matters: Verify answers CheckRelative for a
// path that names a location without naming it from the root, and a POSIX path
// is exactly that on Windows — no drive letter, so filepath.IsAbs says no. The
// file is never created; every test above resolves it through a fake lookPath.
func configuredPath(t *testing.T, name string) string {
	t.Helper()
	return filepath.Join(t.TempDir(), name)
}
