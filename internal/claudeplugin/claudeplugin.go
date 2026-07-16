// Package claudeplugin manages the lich companion plugin inside Claude Code:
// whether it is installed, whether a newer release exists, and installing or
// updating it. All mutations go through the `claude` CLI — the supported
// interface — so lich never edits Claude Code's plugin state files by hand.
package claudeplugin

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/omartelo/lich/internal/winexec"
)

const (
	marketplaceRepo = "omartelo/lich-plugin"
	marketplaceName = "lich-plugin"
	pluginName      = "lich"
	// pluginKey is how Claude Code names the plugin everywhere: install target,
	// update target, and key in installed_plugins.json.
	pluginKey        = pluginName + "@" + marketplaceName
	latestReleaseURL = "https://api.github.com/repos/" + marketplaceRepo + "/releases/latest"

	// cmdTimeout bounds a `claude plugin` call; a marketplace add clones the
	// repo, which the CLI itself caps at 120s.
	cmdTimeout  = 130 * time.Second
	httpTimeout = 5 * time.Second
	bodyLimit   = 1 << 20

	// Release ranks order a pre-release below the release of the same version.
	preReleaseRank = 0
	releaseRank    = 1
)

// BinResolver supplies the Claude Code binary to shell out to. The store
// implements it; passing "" asks for the global binary.
type BinResolver interface {
	ClaudeBin(projectID string) string
}

// Service reports and manages the lich plugin's install state.
type Service struct {
	bins      BinResolver
	http      *http.Client
	configDir string
	// latestURL is the release endpoint to poll; a field so tests can point it
	// at a local server.
	latestURL string
}

// New returns a service that shells out through bins and reads Claude Code's
// plugin state from the resolved config directory.
func New(bins BinResolver) *Service {
	return &Service{
		bins:      bins,
		http:      &http.Client{Timeout: httpTimeout},
		configDir: claudeConfigDir(),
		latestURL: latestReleaseURL,
	}
}

// Status is the plugin's install/update state, reported to the frontend.
type Status struct {
	Installed        bool   `json:"installed"`
	InstalledVersion string `json:"installedVersion"`
	LatestVersion    string `json:"latestVersion"`
	UpdateAvailable  bool   `json:"updateAvailable"`
}

// Status reports whether the plugin is installed and whether a newer release
// exists. A failed network lookup leaves LatestVersion empty and never reports
// an update — it must not block or break app startup.
func (s *Service) Status() Status {
	version, installed := s.installedVersion()
	return computeStatus(installed, version, s.latestVersion())
}

// computeStatus is the pure decision: an update needs the plugin installed, a
// known latest, and that latest to be strictly newer than what is installed.
func computeStatus(installed bool, installedVer, latestVer string) Status {
	return Status{
		Installed:        installed,
		InstalledVersion: installedVer,
		LatestVersion:    latestVer,
		UpdateAvailable:  installed && latestVer != "" && semverLess(installedVer, latestVer),
	}
}

// Install adds the marketplace and installs the plugin at user scope.
func (s *Service) Install() error {
	// A repeat marketplace add errors ("already exists"); that is harmless —
	// the install below is what matters, so only its error is surfaced.
	if err := s.runClaude("plugin", "marketplace", "add", marketplaceRepo); err != nil {
		slog.Debug("claudeplugin: marketplace add", "err", err)
	}
	return s.runClaude("plugin", "install", pluginKey)
}

// Update pulls the latest released version. Claude Code applies it on the next
// session (a restart is required, which the UI signals).
func (s *Service) Update() error {
	return s.runClaude("plugin", "update", pluginKey)
}

func (s *Service) runClaude(args ...string) error {
	bin := s.bins.ClaudeBin("")
	if bin == "" {
		bin = "claude"
	}
	ctx, cancel := context.WithTimeout(context.Background(), cmdTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, bin, args...)
	winexec.Hide(cmd)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("claude %s: %w: %s", strings.Join(args, " "), err, strings.TrimSpace(string(out)))
	}
	return nil
}

// installedVersion reads the plugin's installed version from Claude Code's
// plugin state, or ("", false) when absent or unreadable.
func (s *Service) installedVersion() (string, bool) {
	if s.configDir == "" {
		return "", false
	}
	data, err := os.ReadFile(filepath.Join(s.configDir, "plugins", "installed_plugins.json"))
	if err != nil {
		return "", false
	}
	return parseInstalledVersion(data, pluginKey)
}

// parseInstalledVersion pulls the plugin's version out of installed_plugins.json,
// preferring the user-scope install (how lich installs it) over any other.
func parseInstalledVersion(data []byte, key string) (string, bool) {
	var doc struct {
		Plugins map[string][]struct {
			Scope   string `json:"scope"`
			Version string `json:"version"`
		} `json:"plugins"`
	}
	if err := json.Unmarshal(data, &doc); err != nil {
		return "", false
	}
	entries := doc.Plugins[key]
	for _, e := range entries {
		if e.Scope == "user" && e.Version != "" {
			return e.Version, true
		}
	}
	for _, e := range entries {
		if e.Version != "" {
			return e.Version, true
		}
	}
	return "", false
}

// latestVersion fetches the newest released version from GitHub, or "" on any
// failure — the caller treats an empty result as "no update known".
func (s *Service) latestVersion() string {
	req, err := http.NewRequest(http.MethodGet, s.latestURL, nil)
	if err != nil {
		return ""
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "lich")
	resp, err := s.http.Do(req)
	if err != nil {
		return ""
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return ""
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, bodyLimit))
	if err != nil {
		return ""
	}
	return parseLatestTag(data)
}

// parseLatestTag reads the release tag and normalizes it to a bare semver.
func parseLatestTag(data []byte) string {
	var doc struct {
		TagName string `json:"tag_name"`
	}
	if err := json.Unmarshal(data, &doc); err != nil {
		return ""
	}
	return strings.TrimPrefix(doc.TagName, "v")
}

// semverLess reports whether version a is strictly older than b. Missing or
// non-numeric components sort as 0, and a pre-release sorts before the release
// it qualifies (SemVer §11: 1.0.0-rc1 < 1.0.0) — the plugin does ship rc tags,
// so an install made during an rc window must still see the stable as an update.
//
// Two different pre-releases of the same version compare equal (rc.1 vs rc.2 is
// not ordered): that would need identifier-wise comparison, and the update check
// only ever weighs an install against GitHub's "latest" release, which never
// resolves to a pre-release.
func semverLess(a, b string) bool {
	pa, pb := parseSemver(a), parseSemver(b)
	for i := range pa {
		if pa[i] != pb[i] {
			return pa[i] < pb[i]
		}
	}
	return false
}

// parseSemver splits v into a comparable tuple: MAJOR, MINOR, PATCH, and a
// release rank that keeps a pre-release below the release of the same version.
func parseSemver(v string) [4]int {
	v = strings.TrimPrefix(v, "v")
	// Build metadata carries no precedence (SemVer §10), so drop it before the
	// pre-release check — "+" may legally precede nothing else.
	if i := strings.IndexByte(v, '+'); i >= 0 {
		v = v[:i]
	}
	rank := releaseRank
	if i := strings.IndexByte(v, '-'); i >= 0 {
		v, rank = v[:i], preReleaseRank
	}
	out := [4]int{3: rank}
	for i, part := range strings.SplitN(v, ".", 3) {
		out[i], _ = strconv.Atoi(part)
	}
	return out
}

// claudeConfigDir resolves Claude Code's config directory: the CLAUDE_CONFIG_DIR
// override, else ~/.claude.
func claudeConfigDir() string {
	if d := os.Getenv("CLAUDE_CONFIG_DIR"); d != "" {
		return d
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".claude")
}
