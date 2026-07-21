// Package appupdate checks GitHub for a newer lich release and, on the install
// channels that own their binary (Windows and macOS), downloads and swaps it in
// place. On Linux the binary belongs to the system package manager, so this
// package only reports the update — the UI drives the install through the
// package manager instead (see install.sh and the /restart endpoint).
package appupdate

import (
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"time"

	"github.com/minio/selfupdate"
	"github.com/omartelo/lich/internal/ghrelease"
	"github.com/omartelo/lich/internal/semver"
)

const (
	repo             = "omartelo/lich"
	latestReleaseURL = "https://api.github.com/repos/" + repo + "/releases/latest"
	releaseBase      = "https://github.com/" + repo + "/releases/download/"
	releaseTagBase   = "https://github.com/" + repo + "/releases/tag/"

	// aurPackage is the AUR name Arch users update through their helper.
	aurPackage = "lich-bin"
	// installScript is the deb/rpm/other-distro update path: install.sh detects
	// the distro, installs the matching package, and POSTs /restart itself.
	installScript = "curl -fsSL https://raw.githubusercontent.com/" + repo + "/main/install.sh | sh"
	// restartChain relaunches lich after an install that, unlike install.sh, does
	// not know how. It POSTs the same /restart endpoint using the
	// LICH_PORT/LICH_TOKEN every lich PTY session exports, so the token is never
	// rendered into the pasted text — it stays a shell env reference expanded at
	// run time, not a literal baked into scrollback.
	restartChain = ` && curl -fsS --max-time 5 -X POST "http://127.0.0.1:$LICH_PORT/restart?token=$LICH_TOKEN"`
	// defaultOSRelease is where the distro identity lives; a Service field points
	// tests elsewhere.
	defaultOSRelease = "/etc/os-release"

	httpTimeout = 5 * time.Second
	// bodyLimit caps the JSON/checksums reads; assetLimit caps the binary
	// download (lich is a ~10-20 MiB static binary — 256 MiB is slack, not a
	// target).
	bodyLimit  = 1 << 20
	assetLimit = 256 << 20
)

// Service reports lich's own update state and applies self-updates where the
// binary is writable.
type Service struct {
	http    *http.Client
	version string
	exePath string
	// goos is the platform, a field so tests can drive the self-apply path
	// without running on that OS; defaults to runtime.GOOS.
	goos string
	// latestURL is the release endpoint to poll; a field so tests can point it
	// at a local server. downloadBase / tagBase back the same seam for Apply.
	latestURL    string
	downloadBase string
	tagBase      string
	// osReleasePath is read to pick the Linux install command; a field so tests
	// drive the arch/non-arch branches off a fixture instead of the host's file.
	osReleasePath string
	// applyBinary swaps the running binary for the downloaded one. A field so a
	// test drives Apply's download+verify orchestration without the real swap
	// (which would replace the test binary); defaults to selfupdateApply.
	applyBinary func(r io.Reader, checksum []byte) error
}

// New returns a service that reports version as the running build and polls
// GitHub for the latest release.
func New(version string) *Service {
	exe, _ := os.Executable() // "" if unresolved — canSelfApply then stays false.
	return &Service{
		http:          &http.Client{Timeout: httpTimeout},
		version:       version,
		exePath:       exe,
		goos:          runtime.GOOS,
		latestURL:     latestReleaseURL,
		downloadBase:  releaseBase,
		tagBase:       releaseTagBase,
		osReleasePath: defaultOSRelease,
		applyBinary:   selfupdateApply,
	}
}

// selfupdateApply verifies the stream against checksum and atomically replaces
// the running binary, rolling back on failure — including the Windows
// locked-exe rename dance. It is the one boundary Apply cannot unit-test.
func selfupdateApply(r io.Reader, checksum []byte) error {
	return selfupdate.Apply(r, selfupdate.Options{Checksum: checksum})
}

// Status is lich's install/update state, reported to the frontend.
type Status struct {
	CurrentVersion  string `json:"currentVersion"`
	LatestVersion   string `json:"latestVersion"`
	UpdateAvailable bool   `json:"updateAvailable"`
	CanSelfApply    bool   `json:"canSelfApply"`
	ReleaseURL      string `json:"releaseUrl"`
	// InstallCommand is the shell command the UI pastes (never auto-runs) to
	// update a package-manager-owned install; empty on the self-apply platforms.
	InstallCommand string `json:"installCommand"`
}

// Status reports whether a newer release exists and whether this install can
// swap its own binary. A failed network lookup leaves everything empty and
// reports no update — it must not block or break app startup.
func (s *Service) Status() Status {
	latest := s.latestVersion()
	st := Status{
		CurrentVersion:  s.version,
		LatestVersion:   latest,
		UpdateAvailable: semver.IsRelease(s.version) && latest != "" && semver.Less(s.version, latest),
		CanSelfApply:    canSelfApply(s.goos, s.exePath),
		InstallCommand:  s.installCommand(),
	}
	if latest != "" {
		st.ReleaseURL = s.tagBase + "v" + latest
	}
	return st
}

// Apply downloads the latest release binary for this platform, verifies its
// SHA-256 against the release checksums, and atomically swaps it over the
// running executable. Only valid where CanSelfApply is true.
func (s *Service) Apply() error {
	if !canSelfApply(s.goos, s.exePath) {
		return fmt.Errorf("self-apply not supported on this install")
	}
	latest := s.latestVersion()
	if latest == "" {
		return fmt.Errorf("could not resolve the latest release")
	}
	asset := assetName(s.goos, runtime.GOARCH, latest)
	if asset == "" {
		return fmt.Errorf("no release asset for %s/%s", s.goos, runtime.GOARCH)
	}
	base := s.downloadBase + "v" + latest + "/"

	sum, err := s.fetchChecksum(base+"checksums.txt", asset)
	if err != nil {
		return err
	}
	resp, err := s.get(base + asset)
	if err != nil {
		return fmt.Errorf("download %s: %w", asset, err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download %s: status %d", asset, resp.StatusCode)
	}
	if err := s.applyBinary(io.LimitReader(resp.Body, assetLimit), sum); err != nil {
		return fmt.Errorf("apply update: %w", err)
	}
	return nil
}

// assetName is the release asset for a platform, or "" when none ships. Windows
// and macOS carry a self-apply binary; Linux is package-manager only, so it has
// no self-apply asset here even though a raw linux binary exists in the release.
func assetName(goos, goarch, version string) string {
	name := "lich-v" + version
	switch goos {
	case "windows":
		if goarch == "amd64" {
			return name + "-windows-amd64.exe"
		}
	case "darwin":
		if goarch == "arm64" || goarch == "amd64" {
			return name + "-darwin-" + goarch
		}
	}
	return ""
}

// canSelfApply reports whether this build can replace its own binary: a
// self-apply platform (Windows/macOS) with a known, writable executable
// directory. Linux always returns false — its binary is package-manager owned.
func canSelfApply(goos, exePath string) bool {
	if goos != "windows" && goos != "darwin" {
		return false
	}
	if exePath == "" {
		return false
	}
	return dirWritable(filepath.Dir(exePath))
}

// installCommand is the shell command the UI pastes to update this install, or
// "" on the self-apply platforms (they swap the binary through the button).
// Arch goes through its AUR helper plus an explicit restart — yay knows nothing
// about lich's /restart — while every other distro uses install.sh, which
// restarts itself.
func (s *Service) installCommand() string {
	if s.goos != "linux" {
		return ""
	}
	data, _ := os.ReadFile(s.osReleasePath) // missing/unreadable → not arch → install.sh
	if isArch(string(data)) {
		// ponytail: assumes yay, the common AUR helper; a paru user edits the one
		// word, since the command is pasted for review and never auto-run.
		return "yay -S " + aurPackage + restartChain
	}
	return installScript
}

// isArch reports whether os-release content describes Arch or an Arch
// derivative, mirroring install.sh's detect_family (ID first, then ID_LIKE so
// derivatives map to their parent).
func isArch(osRelease string) bool {
	return osReleaseField(osRelease, "ID") == "arch" ||
		slices.Contains(strings.Fields(osReleaseField(osRelease, "ID_LIKE")), "arch")
}

// osReleaseField returns the unquoted value of a KEY=value line in os-release
// content, or "" when the key is absent.
func osReleaseField(content, key string) string {
	for line := range strings.SplitSeq(content, "\n") {
		if rest, ok := strings.CutPrefix(strings.TrimSpace(line), key+"="); ok {
			return strings.Trim(rest, `"'`)
		}
	}
	return ""
}

// dirWritable reports whether a temp file can be created in dir — the real
// predicate for the atomic swap (selfupdate writes a temp file there, then
// renames it over the target).
func dirWritable(dir string) bool {
	f, err := os.CreateTemp(dir, ".lich-update-*")
	if err != nil {
		return false
	}
	name := f.Name()
	_ = f.Close()
	_ = os.Remove(name)
	return true
}

// fetchChecksum reads checksums.txt and returns the SHA-256 bytes for asset.
func (s *Service) fetchChecksum(url, asset string) ([]byte, error) {
	resp, err := s.get(url)
	if err != nil {
		return nil, fmt.Errorf("download checksums: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("download checksums: status %d", resp.StatusCode)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, bodyLimit))
	if err != nil {
		return nil, fmt.Errorf("read checksums: %w", err)
	}
	sum := parseChecksum(data, asset)
	if sum == "" {
		return nil, fmt.Errorf("no checksum for %s", asset)
	}
	return hex.DecodeString(sum)
}

// parseChecksum finds asset's hash in sha256sum-format lines ("<hex>  <name>").
func parseChecksum(data []byte, asset string) string {
	for line := range strings.SplitSeq(string(data), "\n") {
		fields := strings.Fields(line)
		if len(fields) >= 2 && strings.TrimPrefix(fields[1], "*") == asset {
			return fields[0]
		}
	}
	return ""
}

// latestVersion fetches the newest released version from GitHub, or "" on any
// failure — the caller treats an empty result as "no update known".
func (s *Service) latestVersion() string {
	return ghrelease.LatestTag(s.http, s.latestURL)
}

// get issues a GET with lich's identifying headers.
func (s *Service) get(url string) (*http.Response, error) {
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "lich")
	return s.http.Do(req)
}
