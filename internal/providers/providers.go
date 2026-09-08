// Package providers is the registry of AI coding CLI harnesses lich can run in
// a session (Claude Code, Codex, Antigravity, opencode, oh-my-pi, Crush, Cursor
// CLI, Kiro CLI). A provider id doubles as the session kind that spawns it; the terminal
// resolves the id to a binary, and the settings store keys per-provider
// overrides on it. Detection reads that override and then scans PATH, mirroring
// internal/chromium's browser detection.
package providers

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
)

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
	Kiro        = "kiro"
)

// Provider is a known harness: a stable id, a display name, the executable
// names to look for on PATH (in preference order), and the page that documents
// installing that CLI.
//
// Docs is the page a user who has not got the provider is sent to, so it is the
// install instructions rather than the product's front door — and it is checked
// against the live web when it lands (docs/adding-a-provider.md), because a link
// offered by a "not found" row is worth less than nothing if it 404s.
type Provider struct {
	ID       string
	Name     string
	Binaries []string
	Docs     string
}

// Registry is every provider lich knows about, in display order. Claude Code is
// first: it is the default, and the plugin's home. Every one of them resumes a
// conversation by id (terminal.resumeArgs); all but Cursor CLI also have the
// companion plugin installed into them (agentplugin.supported) — Cursor runs it
// anyway, because it executes every Claude Code hook on the machine, which
// docs/ceilings.md names along with what that costs. What still differs per
// provider is spelled at each of those tables, and AcceptsMCPServer below is
// the one split this file owns.
var Registry = []Provider{
	{ID: Claude, Name: "Claude Code", Binaries: []string{"claude"}, Docs: "https://code.claude.com/docs/en/setup"},
	{ID: Codex, Name: "Codex", Binaries: []string{"codex"}, Docs: "https://learn.chatgpt.com/docs/codex/cli"},
	{ID: Antigravity, Name: "Antigravity", Binaries: []string{"agy"}, Docs: "https://antigravity.google/docs/cli/getting-started"},
	{ID: OpenCode, Name: "opencode", Binaries: []string{"opencode"}, Docs: "https://opencode.ai/docs/"},
	{ID: OMP, Name: "oh-my-pi", Binaries: []string{"omp"}, Docs: "https://github.com/can1357/oh-my-pi"},
	{ID: Crush, Name: "Crush", Binaries: []string{"crush"}, Docs: "https://github.com/charmbracelet/crush"},
	{ID: Cursor, Name: "Cursor CLI", Binaries: []string{"cursor-agent"}, Docs: "https://cursor.com/docs/cli/installation"},
	{ID: Kiro, Name: "Kiro CLI", Binaries: []string{"kiro-cli"}, Docs: "https://kiro.dev/docs/cli/installation/"},
}

// KiroAgentName is the agent profile lich installs into Kiro CLI and spawns it
// with. Kiro keeps its hooks inside an agent config and its built-in
// `kiro_default` cannot be shadowed by a file, so lich's four reports only reach
// a Kiro session through an agent of lich's own — written by
// internal/agentplugin, named on the command line by internal/terminal, and
// spelled once here so those two cannot drift apart.
const KiroAgentName = "lich"

// KiroAgentPath is the file that agent lives in, inside Kiro's global agents
// directory. That root hangs off the home alone — 2.21.0 honours no environment
// variable for it. Resolved here rather than in either caller so the install
// that writes the file (internal/agentplugin) and the spawn that decides whether
// to name it (internal/terminal) cannot drift to different paths.
func KiroAgentPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolve home directory: %w", err)
	}
	return filepath.Join(home, ".kiro", "agents", KiroAgentName+".json"), nil
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

// SupportsFork reports whether a provider can branch an existing conversation
// into a second one — the copy carries the history, the original is left
// untouched — so lich can offer to carry a session's conversation into a new
// checkout instead of only reopening it where it was.
//
// Three of the eight spell it themselves and lich forks through no other route:
// Claude Code's `--fork-session` rides its `--resume`, Codex swaps its `resume`
// subcommand for `fork`, and opencode's `--fork` rides its `--session`
// (measured on 2.1.261, 0.151.0 and 1.18.23). The other five keep no such verb
// — oh-my-pi, Crush, Cursor CLI, Kiro CLI and Antigravity offer resume alone —
// and lich will not forge one: the copy would have to be written into that
// harness's own private store (two SQLite schemas with triggers, a blob store
// keyed on the md5 of the checkout, two JSONL formats carrying the id and cwd
// inside them), none documented, each versioned by its own CLI, and the blast
// radius of getting one wrong is the user's real history. The card says so at
// the dead menu item rather than dropping it (SessionForkItem).
func SupportsFork(id string) bool {
	return id == Claude || id == Codex || id == OpenCode
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

// Where the binary a session would spawn was resolved from. A machine with an
// agent only lich knows about — reached through the binary setting rather than
// $PATH — is installed just as much as one that answers `which`, and the two are
// told apart so Settings can caption the row with which layer won.
const (
	// SourcePath: one of the provider's own binaries answered on $PATH.
	SourcePath = "path"
	// SourceSetting: the binary configured in Settings › Providers, which is the
	// one the spawn resolves first.
	SourceSetting = "setting"
)

// Detected reports a provider and whether a binary a session could spawn was
// found — on PATH, or at the path the user configured.
// Binary is the executable name a session spawns (DefaultBinary), which the
// settings screen needs even when nothing was found: a provider id is not its
// command — Antigravity's is `agy`.
// Docs is carried on every entry, installed or not: the row that has somewhere
// to send the user is exactly the one that found nothing.
// Source is SourcePath or SourceSetting, empty when nothing was found.
type Detected struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Binary    string `json:"binary"`
	Installed bool   `json:"installed"`
	Path      string `json:"path"`
	Source    string `json:"source"`
	Docs      string `json:"docs"`
}

// Service detects installed providers. lookPath is injected so tests drive
// detection without touching the machine.
type Service struct {
	lookPath func(string) (string, error)
	// mu guards refreshPath, wired after construction (SetPathRefresh) while the
	// window may already be calling in.
	mu sync.Mutex
	// refreshPath re-reads the login shell's environment and replaces the PATH
	// lich pinned at launch, everywhere lich resolves a binary from. Nil outside
	// the app — `lich doctor` builds a Service to read the machine, not to
	// change it — and RefreshPath is then the no-op that scans the same PATH
	// again.
	refreshPath func() error
	// configuredBin answers a provider's binary setting, the global one. Also
	// guarded by mu, and also nil outside the app: a Service built to read the
	// machine has no workspace database behind it, so it scans PATH alone.
	configuredBin func(id string) string
}

// New returns a Service that scans the real PATH.
func New() *Service {
	return &Service{lookPath: exec.LookPath}
}

// SetPathRefresh wires what a re-check re-reads the machine's PATH with. It is
// one resolution feeding several readers — the process PATH exec.LookPath goes
// through, the environment a new session inherits, the editor lookup — so
// main.go owns the applying and this package only asks for it.
func (s *Service) SetPathRefresh(fn func() error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.refreshPath = fn
}

// SetConfiguredBin wires where Detect reads a provider's binary setting from.
// The settings store cannot be reached from here — it imports this package to
// key those settings — so main.go hands the reader in, exactly as it does the
// PATH refresh.
func (s *Service) SetConfiguredBin(fn func(id string) string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.configuredBin = fn
}

// binSetting reads the configured binary for a provider, "" when nothing is
// wired or nothing is set.
func (s *Service) binSetting(id string) string {
	s.mu.Lock()
	read := s.configuredBin
	s.mu.Unlock()
	if read == nil {
		return ""
	}
	return read(id)
}

// RefreshPath re-reads the login shell's environment and re-pins what it
// resolved, so the Detect and Verify below answer from what is installed now
// rather than from what was installed when lich started. It is what lets a
// re-check recover a machine that installed an agent, git or gh into a
// directory the launch PATH did not carry, without a relaunch.
//
// It runs under the resolution's own bound (terminal.ReresolveShellEnv), and a
// shell that does not answer inside it leaves the pin exactly as it is and says
// so: re-scanning the old PATH would report the same absence as though it were
// news.
func (s *Service) RefreshPath() error {
	s.mu.Lock()
	refresh := s.refreshPath
	s.mu.Unlock()
	if refresh == nil {
		return nil
	}
	return refresh()
}

// Detect returns every known provider with its install state, resolving the
// binary exactly as a spawn would: the configured one first, then the first
// candidate found on PATH. The list order matches Registry.
//
// Reading the setting is what keeps a machine whose only agent lives at a path
// the user typed from reading as a bare machine — the state the empty screen
// answers with a terminal.
func (s *Service) Detect() ([]Detected, error) {
	out := make([]Detected, 0, len(Registry))
	for _, p := range Registry {
		d := Detected{ID: p.ID, Name: p.Name, Binary: DefaultBinary(p.ID), Docs: p.Docs}
		out = append(out, s.resolve(d, p.Binaries))
	}
	return out, nil
}

// resolve fills in where the provider's binary was found. A configured binary
// answers alone, whether or not it resolves: it is what the spawn will run, so
// falling through to PATH would report a binary no session of this provider
// would ever start.
func (s *Service) resolve(d Detected, binaries []string) Detected {
	if bin := s.binSetting(d.ID); bin != "" {
		if check := s.Verify(bin); check.Status == CheckOK {
			d.Installed, d.Path, d.Source = true, check.Path, SourceSetting
		}
		return d
	}
	for _, name := range binaries {
		if path, err := s.lookPath(name); err == nil {
			d.Installed, d.Path, d.Source = true, path, SourcePath
			break
		}
	}
	return d
}

// CostSource is whose arithmetic a session's dollars in the cost ledger are.
// Five providers file a figure there and they reach it two ways, which any
// report summing them has to be able to say: a dollar lich derived and a dollar
// Crush reported are the same number on screen and not the same claim.
type CostSource string

const (
	// CostSourceNone: no dollars at all. Antigravity and Cursor CLI file their
	// conversations where lich has no reader, and Kiro CLI meters spend in
	// credits, not money — and a session running the user's shell was never
	// spawned as a provider, so whatever it ran by hand is on neither rung.
	CostSourceNone CostSource = ""
	// CostSourcePriced: lich prices the conversation itself, from the token
	// counts the provider writes down (internal/pricing).
	CostSourcePriced CostSource = "priced"
	// CostSourceReported: the provider priced its own turns and lich files that
	// figure unchanged — each bills models no table here knows, so a second
	// opinion would only be a second, disagreeing number.
	CostSourceReported CostSource = "reported"
)

// CostSourceOf says which of the two a provider's spend is, and it is the one
// answer both the live readout and `lich cost` rely on. The rung is a fact of
// the provider — what it writes down decides who can price it — so it is
// answered from the id alone and never recorded per ledger row.
// docs/ceilings.md carries what each rung's figure leaves out.
func CostSourceOf(id string) CostSource {
	switch id {
	case Claude, Codex:
		return CostSourcePriced
	case OMP, OpenCode, Crush:
		return CostSourceReported
	}
	return CostSourceNone
}
