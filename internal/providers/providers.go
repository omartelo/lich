// Package providers is the registry of AI coding CLI harnesses lich can run in
// a session (Claude Code, Codex, opencode, Crush). A provider id doubles as the
// session kind that spawns it; the terminal resolves the id to a binary, and the
// settings store keys per-provider overrides on it. Detection is a PATH scan,
// mirroring internal/chromium's browser detection.
package providers

import "os/exec"

// Provider ids. Each id is also the session kind (store column + terminal.Start)
// that runs the provider. Kept in sync with frontend/src/lib/sessions.ts.
const (
	Claude   = "claude"
	Codex    = "codex"
	OpenCode = "opencode"
	Crush    = "crush"
)

// Provider is a known harness: a stable id, a display name, and the executable
// names to look for on PATH (in preference order).
type Provider struct {
	ID       string
	Name     string
	Binaries []string
}

// Registry is every provider lich knows about, in display order. Claude Code is
// first: it is the default and the only one wired for resume, the plugin and
// ai-titles — the rest just spawn their TUI in a PTY.
var Registry = []Provider{
	{ID: Claude, Name: "Claude Code", Binaries: []string{"claude"}},
	{ID: Codex, Name: "Codex", Binaries: []string{"codex"}},
	{ID: OpenCode, Name: "opencode", Binaries: []string{"opencode"}},
	{ID: Crush, Name: "Crush", Binaries: []string{"crush"}},
}

// DefaultBinary returns a provider's preferred executable name, or "" for an
// unknown id.
func DefaultBinary(id string) string {
	for _, p := range Registry {
		if p.ID == id && len(p.Binaries) > 0 {
			return p.Binaries[0]
		}
	}
	return ""
}

// Detected reports a provider and whether one of its binaries was found on PATH.
type Detected struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Installed bool   `json:"installed"`
	Path      string `json:"path"`
}

// Service detects installed providers. lookPath is injected so tests drive
// detection without touching the machine.
type Service struct {
	lookPath func(string) (string, error)
}

// New returns a Service that scans the real PATH.
func New() *Service {
	return &Service{lookPath: exec.LookPath}
}

// Detect returns every known provider with its install state, resolving the
// first candidate binary found on PATH. The list order matches Registry.
func (s *Service) Detect() ([]Detected, error) {
	out := make([]Detected, 0, len(Registry))
	for _, p := range Registry {
		d := Detected{ID: p.ID, Name: p.Name}
		for _, name := range p.Binaries {
			if path, err := s.lookPath(name); err == nil {
				d.Installed = true
				d.Path = path
				break
			}
		}
		out = append(out, d)
	}
	return out, nil
}
