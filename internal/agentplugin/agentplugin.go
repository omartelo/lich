// Package agentplugin manages the lich companion plugin inside the provider
// CLIs that can run it: whether it is installed, whether a newer release
// exists, and installing or updating it. Six of the registry's seven ship the
// same plugin from the same repository; what differs per provider — the
// subcommands, where the installed version is read from, and whether there is a
// CLI at all — lives in claude.go, codex.go, antigravity.go, opencode.go,
// omp.go and crush.go.
//
// Two install shapes, decided by the harness rather than by lich:
//
//   - Claude Code and Codex are driven through their own plugin CLI, the
//     supported interface, so lich never edits their state files by hand.
//   - Antigravity, opencode, oh-my-pi and Crush have lich write the files the
//     release published: a customization directory under Antigravity's global
//     `config/plugins/`, a module into opencode's plugin directory, another into
//     oh-my-pi's extensions directory, and hook scripts plus a delimited block
//     of `hook add` lines into Crush's crushrc. Each carries the version it
//     installed — a marker line, or a field in a manifest lich owns outright —
//     which is the only record of what is installed where the harness keeps
//     none. Antigravity does have a plugin CLI; antigravity.go says why it is
//     not the install.
package agentplugin

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"slices"
	"strings"
	"time"

	"github.com/omartelo/lich/internal/ghrelease"
	"github.com/omartelo/lich/internal/providers"
	"github.com/omartelo/lich/internal/semver"
	"github.com/omartelo/lich/internal/winexec"
)

const (
	marketplaceRepo = "omartelo/lich-plugin"
	marketplaceName = "lich-plugin"
	pluginName      = "lich"
	// pluginKey is how both harnesses name the plugin: install target, update
	// target, and key in their installed-plugin state.
	pluginKey        = pluginName + "@" + marketplaceName
	latestReleaseURL = "https://api.github.com/repos/" + marketplaceRepo + "/releases/latest"
	// rawBaseURL serves the repository's files at a tag, for the harnesses lich
	// installs by writing files rather than by calling a CLI.
	rawBaseURL = "https://raw.githubusercontent.com/" + marketplaceRepo
	// markerName labels the lines lich writes into a harness's files, so an
	// update can find what a previous install left there.
	markerName = marketplaceName

	// cmdTimeout bounds a plugin mutation; a marketplace add clones the repo,
	// which Claude Code itself caps at 120s. readTimeout bounds the status
	// question asked at startup, which reads local state and must not hold the
	// prompt behind a CLI that never answers.
	cmdTimeout  = 130 * time.Second
	readTimeout = 10 * time.Second
	httpTimeout = 5 * time.Second
)

// supported is every provider that can run the plugin, in the order the UI
// lists them. A provider outside this list has no plugin to offer, so it never
// reaches a status or an install.
//
// Cursor CLI is the registry's seventh and is not here: nothing has been
// measured against a real run of it, and an install that registers hooks the
// CLI never fires is a card that reports nothing while claiming it is wired.
// Its CLI reads Claude Code's own hook spelling and loads a plugin directory
// carrying `.claude-plugin/plugin.json`, so the gap is closable —
// docs/ceilings.md carries what that costs today.
var supported = []string{
	providers.Claude, providers.Codex, providers.Antigravity,
	providers.OpenCode, providers.OMP, providers.Crush,
}

// BinResolver supplies the binary to shell out to for a provider. The store
// implements it; the empty project id asks for the global value, and an empty
// return means "the provider's default name on PATH".
type BinResolver interface {
	ProviderBin(providerID, projectID string) string
}

// Service reports and manages the lich plugin's install state per provider.
type Service struct {
	bins BinResolver
	http *http.Client
	// latestURL is the release endpoint to poll; a field so tests can point it
	// at a local server.
	latestURL string
	// rawBase serves the plugin repository's files at a tag; a field so tests
	// can point it at a local server.
	rawBase string
	// lookPath resolves a binary on PATH, injected so tests decide which
	// provider CLIs the machine has.
	lookPath func(string) (string, error)
	// lichBin names this lich for the registrations written into a harness's
	// config. Injected for the same reason lookPath is: a test has to be able to
	// say "this binary cannot be resolved", which is the branch that decides
	// whether a session is promised tools it does not have.
	lichBin func() string
}

// New returns a service that shells out through bins.
func New(bins BinResolver) *Service {
	return &Service{
		bins:      bins,
		http:      &http.Client{Timeout: httpTimeout},
		latestURL: latestReleaseURL,
		rawBase:   rawBaseURL,
		lookPath:  exec.LookPath,
		lichBin:   lichBinary,
	}
}

// Status is one provider's plugin state, reported to the frontend.
type Status struct {
	Provider string `json:"provider"`
	Name     string `json:"name"`
	// Available is whether the provider's CLI is on PATH at all. A provider the
	// machine does not have is still listed — with nothing to install — so the
	// UI can say why it is not on offer.
	Available        bool   `json:"available"`
	Installed        bool   `json:"installed"`
	InstalledVersion string `json:"installedVersion"`
	LatestVersion    string `json:"latestVersion"`
	UpdateAvailable  bool   `json:"updateAvailable"`
}

// Status reports, for every provider that can run the plugin, whether its CLI
// is present, whether the plugin is installed there and whether a newer release
// exists. The release lookup is one call for the whole list — both harnesses
// install the same repository — and a failed one leaves LatestVersion empty and
// never reports an update: this runs at startup and must not block it.
func (s *Service) Status() []Status {
	latest := s.latestVersion()
	out := make([]Status, 0, len(supported))
	for _, id := range supported {
		available := s.available(id)
		version := ""
		installed := false
		if available {
			version, installed = s.installedVersion(id)
		}
		out = append(out, computeStatus(id, available, installed, version, latest))
	}
	return out
}

// toolsMinVersion is the first plugin release that carries lich's own
// operations into a session — as tools of opencode's own, and as the MCP server
// registered in Crush's config. Claude Code and Codex never ask: they are told
// about the server on their own command line at spawn, whatever the plugin's
// version. oh-my-pi clears it by arriving later: its module only exists in
// releases well past this one, and its MCP registration is written by the same
// install.
//
// The version is checked rather than assumed because pointing a session at a
// tool its plugin predates is worse than naming the shell command, which works
// everywhere.
const toolsMinVersion = "0.9.0"

// HasTools reports whether a provider's sessions can call lich's own operations
// — whether a relayed message may name a tool rather than the command line.
//
// True for the two lich registers at spawn, and for the three that get them
// with the plugin, once the installed one is new enough.
func (s *Service) HasTools(provider string) bool {
	if providers.AcceptsMCPServer(provider) {
		return true
	}
	if !slices.Contains(supported, provider) || !s.available(provider) {
		return false
	}
	if registersServerAtInstall(provider) && s.lichBin() == "" {
		return false
	}
	version, installed := s.installedVersion(provider)
	return installed && !semver.Less(version, toolsMinVersion)
}

// registersServerAtInstall reports whether a provider's tools come from the MCP
// server the install writes into its config, rather than from the plugin itself.
//
// It is what stops HasTools promising tools the install could not register: all
// three write the lich binary's own path, and all three leave the registration
// out when it cannot be resolved (crushrcBlock, ompMCPDocument,
// antigravityRegisterMCP) — so a session there has the plugin's reports and no
// tool list, and must be told the command line instead. opencode is absent
// because its plugin defines the tools itself, with no binary to name.
func registersServerAtInstall(provider string) bool {
	return provider == providers.Crush || provider == providers.OMP ||
		provider == providers.Antigravity
}

// Installed reports whether the lich plugin is installed for a provider — that
// its sessions report what they are doing (docs/hooks/). The relay asks before
// reading a session's silence as an answer: a provider that never reports is
// silent whatever happens in it.
//
// It reads the provider's own plugin state — a file for Claude Code, opencode
// and oh-my-pi, a CLI call for Codex and Crush — so it is not for a hot path. The
// relay asks it once per errand, and HasTools once more before that; neither
// question is cached, so each one pays for itself.
func (s *Service) Installed(provider string) bool {
	if !slices.Contains(supported, provider) || !s.available(provider) {
		return false
	}
	_, installed := s.installedVersion(provider)
	return installed
}

// computeStatus is the pure decision: an update needs the plugin installed, a
// known latest, and that latest to be strictly newer than what is installed.
func computeStatus(id string, available, installed bool, installedVer, latestVer string) Status {
	return Status{
		Provider:         id,
		Name:             providerName(id),
		Available:        available,
		Installed:        installed,
		InstalledVersion: installedVer,
		LatestVersion:    latestVer,
		UpdateAvailable:  installed && latestVer != "" && semver.Less(installedVer, latestVer),
	}
}

// Install puts the plugin into a provider: through its plugin CLI where there
// is one, by writing the released files where there is not.
func (s *Service) Install(provider string) error {
	switch provider {
	case providers.Claude:
		return s.claudeInstall()
	case providers.Codex:
		return s.codexInstall()
	case providers.Antigravity:
		return s.antigravityInstall()
	case providers.OpenCode:
		return s.opencodeInstall()
	case providers.OMP:
		return s.ompInstall()
	case providers.Crush:
		return s.crushInstall()
	}
	return fmt.Errorf("no lich plugin for provider %q", provider)
}

// Update pulls the latest released version into a provider. Every one applies
// it on the next session (a restart is required, which the UI signals). For the
// file-shipped harnesses an update is the same write as an install — the files
// are replaced wholesale — so they share one path.
func (s *Service) Update(provider string) error {
	switch provider {
	case providers.Claude:
		return s.claudeUpdate()
	case providers.Codex:
		return s.codexUpdate()
	case providers.Antigravity:
		return s.antigravityInstall()
	case providers.OpenCode:
		return s.opencodeInstall()
	case providers.OMP:
		return s.ompInstall()
	case providers.Crush:
		return s.crushInstall()
	}
	return fmt.Errorf("no lich plugin for provider %q", provider)
}

// installedVersion reads the plugin's installed version from a provider's own
// plugin state, or ("", false) when absent or unreadable.
func (s *Service) installedVersion(provider string) (string, bool) {
	switch provider {
	case providers.Claude:
		return claudeInstalledVersion()
	case providers.Codex:
		return s.codexInstalledVersion()
	case providers.Antigravity:
		return s.antigravityInstalledVersion()
	case providers.OpenCode:
		return s.opencodeInstalledVersion()
	case providers.OMP:
		return s.ompInstalledVersion()
	case providers.Crush:
		return s.crushInstalledVersion()
	}
	return "", false
}

// available reports whether a provider's CLI can be found to shell out to.
func (s *Service) available(provider string) bool {
	_, err := s.lookPath(s.bin(provider))
	return err == nil
}

// bin is the executable to run for a provider: the configured override, else
// the provider's default name (resolved on PATH).
func (s *Service) bin(provider string) string {
	if bin := s.bins.ProviderBin(provider, ""); bin != "" {
		return bin
	}
	return providers.DefaultBinary(provider)
}

// providerName is a provider's display name, for messages the user reads.
func providerName(id string) string {
	for _, p := range providers.Registry {
		if p.ID == id {
			return p.Name
		}
	}
	return id
}

// run executes a mutation on a provider's CLI, returning its combined output on
// failure so the reason reaches the user instead of a bare exit code.
func (s *Service) run(provider string, args ...string) error {
	out, err := s.exec(provider, cmdTimeout, args...)
	if err != nil {
		return fmt.Errorf("%s %s: %w: %s",
			s.bin(provider), strings.Join(args, " "), err, strings.TrimSpace(out))
	}
	return nil
}

// read asks a provider's CLI a question and returns its combined output. Bounded
// far tighter than a mutation: this runs on the startup status check, where a CLI
// that hangs must cost a missing row, not a stalled prompt.
func (s *Service) read(provider string, args ...string) (string, error) {
	return s.exec(provider, readTimeout, args...)
}

// exec spawns a provider's CLI and returns its combined output.
func (s *Service) exec(provider string, timeout time.Duration, args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, s.bin(provider), args...)
	winexec.Hide(cmd)
	out, err := cmd.CombinedOutput()
	return string(out), err
}

// latestVersion fetches the newest released version from GitHub, or "" on any
// failure — the caller treats an empty result as "no update known".
func (s *Service) latestVersion() string {
	return ghrelease.LatestTag(s.http, s.latestURL)
}

// writeFile writes data to path through a temporary file in the same directory.
// Every path this package writes is one a harness loads and runs, and a crash or
// a full disk halfway through os.WriteFile leaves a truncated one that still
// loads — with lich's version marker on its first line saying it is whole.
func writeFile(path string, data []byte, perm os.FileMode) error {
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, perm); err != nil {
		return fmt.Errorf("write %s: %w", tmp, err)
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("write %s: %w", path, err)
	}
	return nil
}
