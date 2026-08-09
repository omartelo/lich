// Package agentplugin manages the lich companion plugin inside the provider
// CLIs that can run it (Claude Code, Codex): whether it is installed, whether a
// newer release exists, and installing or updating it. Both harnesses ship the
// same plugin from the same repository, and both are driven through their own
// CLI — the supported interface — so lich never edits a harness's plugin state
// files by hand. What differs per provider (the subcommands, and where the
// installed version is read from) lives in claude.go and codex.go.
package agentplugin

import (
	"context"
	"fmt"
	"net/http"
	"os/exec"
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

	// cmdTimeout bounds a plugin mutation; a marketplace add clones the repo,
	// which Claude Code itself caps at 120s. readTimeout bounds the status
	// question asked at startup, which reads local state and must not hold the
	// prompt behind a CLI that never answers.
	cmdTimeout  = 130 * time.Second
	readTimeout = 10 * time.Second
	httpTimeout = 5 * time.Second
)

// supported is every provider whose CLI can run the plugin, in the order the UI
// lists them. A provider outside this list has no plugin to offer, so it never
// reaches a status or an install.
var supported = []string{providers.Claude, providers.Codex}

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
	// lookPath resolves a binary on PATH, injected so tests decide which
	// provider CLIs the machine has.
	lookPath func(string) (string, error)
}

// New returns a service that shells out through bins.
func New(bins BinResolver) *Service {
	return &Service{
		bins:      bins,
		http:      &http.Client{Timeout: httpTimeout},
		latestURL: latestReleaseURL,
		lookPath:  exec.LookPath,
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

// Install adds the marketplace and installs the plugin into a provider's CLI.
func (s *Service) Install(provider string) error {
	switch provider {
	case providers.Claude:
		return s.claudeInstall()
	case providers.Codex:
		return s.codexInstall()
	}
	return fmt.Errorf("no lich plugin for provider %q", provider)
}

// Update pulls the latest released version into a provider's CLI. Both apply it
// on the next session (a restart is required, which the UI signals).
func (s *Service) Update(provider string) error {
	switch provider {
	case providers.Claude:
		return s.claudeUpdate()
	case providers.Codex:
		return s.codexUpdate()
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
