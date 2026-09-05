package providers

import (
	"errors"
	"os/exec"
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
