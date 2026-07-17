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
		http:         &http.Client{Timeout: httpTimeout},
		version:      version,
		exePath:      exe,
		goos:         runtime.GOOS,
		latestURL:    latestReleaseURL,
		downloadBase: releaseBase,
		tagBase:      releaseTagBase,
		applyBinary:  selfupdateApply,
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
