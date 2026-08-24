// Package providers is the registry of AI coding CLI harnesses lich can run in
// a session (Claude Code, Codex, Antigravity, opencode, oh-my-pi, Crush, Cursor
// CLI). A provider id doubles as the session kind that spawns it; the terminal
// resolves the id to a binary, and the settings store keys per-provider
// overrides on it. Detection is a PATH scan, mirroring internal/chromium's
// browser detection.
package providers

import "os/exec"

// Provider ids. Each id is also the session kind (store column + terminal.Start)
// that runs the provider. Kept in sync with frontend/src/lib/session/sessions.ts.
const (
	Claude      = "claude"
	Codex       = "codex"
	Antigravity = "antigravity"
	OpenCode    = "opencode"
	OMP         = "omp"
	Crush       = "crush"
	Cursor      = "cursor"
)

// Provider is a known harness: a stable id, a display name, and the executable
// names to look for on PATH (in preference order).
type Provider struct {
	ID       string
	Name     string
	Binaries []string
}

// Registry is every provider lich knows about, in display order. Claude Code is
// first: it is the default, and the plugin's home. Every one of them resumes a
// conversation by id (terminal.resumeArgs); all but Cursor CLI also run the
// companion plugin (agentplugin.supported), which is why a Cursor session
// reports no state and is never offered a resume — docs/ceilings.md names it.
// What still differs per provider is spelled at each of those tables, and
// AcceptsMCPServer below is the one split this file owns.
var Registry = []Provider{
	{ID: Claude, Name: "Claude Code", Binaries: []string{"claude"}},
	{ID: Codex, Name: "Codex", Binaries: []string{"codex"}},
	{ID: Antigravity, Name: "Antigravity", Binaries: []string{"agy"}},
	{ID: OpenCode, Name: "opencode", Binaries: []string{"opencode"}},
	{ID: OMP, Name: "oh-my-pi", Binaries: []string{"omp"}},
	{ID: Crush, Name: "Crush", Binaries: []string{"crush"}},
	{ID: Cursor, Name: "Cursor CLI", Binaries: []string{"cursor-agent"}},
}

// Known reports whether id names a registered provider. It guards a provider id
// that arrives from outside lich — a hook payload — before it is used as one.
func Known(id string) bool {
	for _, p := range Registry {
		if p.ID == id {
			return true
		}
	}
	return false
}

// AcceptsMCPServer reports whether a provider can be handed an MCP server on
// its own command line, so lich can register one for a session it spawns
// without editing config that belongs to the user or to their repository.
//
// Claude Code takes `--mcp-config` with a JSON string; Codex takes `-c`
// overrides for its `mcp_servers` table. Antigravity, opencode, oh-my-pi, Crush
// and Cursor CLI have no such flag — Antigravity keeps MCP behind an `agy mcp`
// subcommand, Crush's whole flag list is cwd, data-dir, session and debug, and
// Cursor's `mcp` subcommand only lists, enables and disables what is already in
// `~/.cursor/mcp.json` (measured on 2026.08.11) — so registering for those means
// writing config that outlives the spawn, which is what the plugin install does
// for the three that can take it. Their sessions reach the other sessions
// through the `lich` command line instead (docs/cli.md).
func AcceptsMCPServer(id string) bool {
	return id == Claude || id == Codex
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
// Binary is the executable name a session spawns (DefaultBinary), which the
// settings screen needs even when nothing was found: a provider id is not its
// command — Antigravity's is `agy`.
type Detected struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Binary    string `json:"binary"`
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
		d := Detected{ID: p.ID, Name: p.Name, Binary: DefaultBinary(p.ID)}
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
