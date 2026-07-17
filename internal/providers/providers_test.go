package providers

import (
	"errors"
	"os/exec"
	"testing"
)

func TestDefaultBinary(t *testing.T) {
	cases := map[string]string{
		Claude:   "claude",
		Codex:    "codex",
		OpenCode: "opencode",
		Crush:    "crush",
		"nope":   "",
		"":       "",
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
	if got[3].ID != Crush || !got[3].Installed || got[3].Path != "/opt/bin/crush" {
		t.Errorf("crush = %+v, want installed at /opt/bin/crush", got[3])
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
