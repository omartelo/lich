// Package appupdate checks GitHub for a newer lich release and, on a Windows
// install no package manager owns, downloads and runs the release's installer.
// Everywhere else lich belongs to a package manager (a Linux package, Homebrew,
// Scoop), so this package only reports the update: the UI drives the install
// through that manager instead (see install.sh and the /restart endpoint), or
// points at the release page where it can name none.
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
	"sync"
	"time"

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
	// brewPackage is the tap-qualified name Homebrew users update through; brew
	// owns its copy, so lich never swaps it itself.
	brewPackage = "omartelo/tap/lich"
	// installScript is the deb/rpm/other-distro update path: install.sh detects
	// the distro, installs the matching package, and POSTs /restart itself.
	installScript = "curl -fsSL https://raw.githubusercontent.com/" + repo + "/main/install.sh | sh"
	// restartChain relaunches lich after an install that, unlike install.sh, does
	// not know how. It POSTs the same /restart endpoint using the
	// LICH_PORT/LICH_TOKEN every lich PTY session exports, so the token is never
	// rendered into the pasted text — it stays a shell env reference expanded at
	// run time, not a literal baked into scrollback.
	restartChain = ` && curl -fsS --max-time 5 -X POST "http://127.0.0.1:$LICH_PORT/restart?token=$LICH_TOKEN"`
	// scoopPackage is the manifest name a Scoop install updates through; Scoop
	// owns its copy under apps\lich\current, so lich never swaps it itself.
	scoopPackage = "lich"
	// restartChainPwsh is restartChain for the PowerShell a Windows session runs:
	// `;` because Windows PowerShell 5.1 has no `&&`, and the env references in
	// its own syntax. Scoop keeps the old version's directory until `scoop
	// cleanup`, so the update lands while lich runs and the restart picks it up
	// through the repointed junction.
	restartChainPwsh = `; Invoke-RestMethod -Method Post "http://127.0.0.1:$env:LICH_PORT/restart?token=$env:LICH_TOKEN"`
	// defaultOSRelease is where the distro identity lives; a Service field points
	// tests elsewhere.
	defaultOSRelease = "/etc/os-release"

	httpTimeout = 5 * time.Second
	// downloadTimeout bounds the installer download. A client Timeout spans the
	// whole body read, so the metadata timeout above would cut a multi-MiB
	// asset mid-stream on any modest link; this one is a hang stop, not a pace.
	downloadTimeout = 5 * time.Minute
)

// Service reports lich's own update state and applies self-updates where the
// binary is writable.
type Service struct {
	mu      sync.Mutex
	install func(string) error
	// emit publishes Apply's progress to the app's event hub (ProgressEventName);
	// nil leaves the update working and silent.
	emit func(name string, data any)

	http *http.Client
	// download carries the release-asset GET; separate from http so the short
	// metadata timeout never applies to the body. Nil falls back to http
	// (tests build bare Services against local servers).
	download *http.Client
	version  string
	exePath  string
	// goos and goarch are the platform, fields so tests can drive the
	// installer path without running on that OS or CPU; they default to
	// runtime.GOOS/runtime.GOARCH.
	goos   string
	goarch string
	// latestURL is the release endpoint to poll; a field so tests can point it
	// at a local server. downloadBase / tagBase back the same seam for Apply.
	latestURL    string
	downloadBase string
	tagBase      string
	// osReleasePath is read to pick the Linux install command; a field so tests
	// drive the arch/non-arch branches off a fixture instead of the host's file.
	osReleasePath string
}

// New returns a service that reports version as the running build and polls
// GitHub for the latest release. install runs a downloaded Windows installer;
// emit is the app's event hub.
func New(version string, install func(string) error, emit func(name string, data any)) *Service {
	exe, _ := os.Executable() // "" if unresolved — canSelfApply then stays false.
	return &Service{
		install:       install,
		emit:          emit,
		http:          &http.Client{Timeout: httpTimeout},
		download:      &http.Client{Timeout: downloadTimeout},
		version:       version,
		exePath:       exe,
		goos:          runtime.GOOS,
		goarch:        runtime.GOARCH,
		latestURL:     latestReleaseURL,
		downloadBase:  releaseBase,
		tagBase:       releaseTagBase,
		osReleasePath: defaultOSRelease,
	}
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
// update itself through the installer. A failed network lookup leaves everything empty and
// reports no update — it must not block or break app startup.
func (s *Service) Status() Status {
	latest := s.latestVersion()
	st := Status{
		CurrentVersion:  s.version,
		LatestVersion:   latest,
		UpdateAvailable: semver.IsRelease(s.version) && latest != "" && semver.Less(s.version, latest),
		CanSelfApply:    canSelfApply(s.goos, s.goarch, s.exePath),
		InstallCommand:  s.installCommand(),
	}
	if latest != "" {
		st.ReleaseURL = s.tagBase + "v" + latest
	}
	return st
}

// Apply verifies the latest release's Windows installer against its SHA-256
// checksum and hands the install over to it; the installer closes and
// relaunches lich. Only valid where CanSelfApply is true.
func (s *Service) Apply() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !canSelfApply(s.goos, s.goarch, s.exePath) {
		return fmt.Errorf("self-apply not supported on this install")
	}
	latest := s.latestVersion()
	if latest == "" {
		return fmt.Errorf("could not resolve the latest release")
	}
	asset := "lich-v" + latest + "-windows-amd64-setup.exe"
	base := s.downloadBase + "v" + latest + "/"

	sum, err := s.fetchChecksum(base+"checksums.txt", asset)
	if err != nil {
		return err
	}
	resp, err := s.getAsset(base + asset)
	if err != nil {
		return fmt.Errorf("download %s: %w", asset, err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download %s: status %d", asset, resp.StatusCode)
	}
	return s.applyInstaller(s.progressReader(resp.Body, resp.ContentLength, phaseInstaller), sum)
}

// canSelfApply reports whether this install updates through the button: a
// Windows amd64 install (the only installer that ships) in a writable directory
// Scoop does not own. The installer is the route for a portable folder too:
// it waits for lich to exit before it replaces lich.exe and the window's
// libcef.dll, both locked while they run, and relaunches lich afterwards.
// macOS and Linux lich belong to a package manager, or to the user who
// unpacked them.
func canSelfApply(goos, goarch, exePath string) bool {
	if goos != "windows" || goarch != "amd64" || exePath == "" || scoopOwned(exePath) {
		return false
	}
	return dirWritable(filepath.Dir(exePath))
}

// scoopOwned reports whether exePath is a Scoop install: <root>\apps\lich\
// <version or current>\lich.exe, whichever way the junction was resolved. The
// directory is writable, so without this check it would go to the Inno Setup
// installer, which would register a second lich in "Installed apps" and write into the
// version directory Scoop tracks.
func scoopOwned(exePath string) bool {
	segments := strings.Split(filepath.ToSlash(exePath), "/")
	n := len(segments)
	return n >= 4 && strings.EqualFold(segments[n-4], "apps") && strings.EqualFold(segments[n-3], scoopPackage)
}

// bundled reports whether exePath is the executable inside lich's macOS .app —
// what the Homebrew cask installs into /Applications, so it updates through
// Homebrew.
func bundled(exePath string) bool {
	return strings.Contains(filepath.ToSlash(exePath), ".app/Contents/MacOS/")
}

// installCommand is the shell command the UI pastes to update this install, or
// "" where no package manager owns it (the UI then offers the release page).
// A Scoop install is package-manager owned like Arch's, and its restart is
// spelled for PowerShell.
// Arch goes through its AUR helper plus an explicit restart — yay knows nothing
// about lich's /restart — while every other distro uses install.sh, which
// restarts itself. A Homebrew install is package-manager owned like Arch's, on
// whichever platform it runs, and macOS ships as a cask.
func (s *Service) installCommand() string {
	if bundled(s.exePath) {
		return "brew upgrade --cask " + brewPackage + restartChain
	}
	if scoopOwned(s.exePath) {
		return "scoop update " + scoopPackage + restartChainPwsh
	}
	if s.goos != "linux" || !packaged(s.exePath) {
		return ""
	}
	data, _ := os.ReadFile(s.osReleasePath) // missing/unreadable → not arch → install.sh
	if isArch(string(data)) {
		// Assumes yay, the common AUR helper; a paru user edits the one word,
		// since the command is pasted for review and never auto-run.
		return "yay -S " + aurPackage + restartChain
	}
	return installScript
}

// packaged reports whether exePath is where the deb, rpm and AUR packages put
// lich, with the window in ../lib/lich/shell. The tarball keeps it beside the
// binary instead, and a package installed over a tarball would leave /restart
// relaunching the tarball's binary, still at the old version.
func packaged(exePath string) bool {
	_, err := os.Stat(filepath.Join(filepath.Dir(exePath), "..", "lib", "lich", "shell", "lich-shell"))
	return err == nil
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
	data, err := io.ReadAll(io.LimitReader(resp.Body, ghrelease.BodyLimit))
	if err != nil {
		return nil, fmt.Errorf("read checksums: %w", err)
	}
	sum := parseChecksum(data, asset)
	if sum == "" {
		return nil, fmt.Errorf("no checksum for %s", asset)
	}
	decoded, err := hex.DecodeString(sum)
	if err != nil {
		return nil, fmt.Errorf("decode checksums: %w", err)
	}
	return decoded, nil
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

// get issues a metadata GET (JSON, checksums) on the short-timeout client.
func (s *Service) get(url string) (*http.Response, error) {
	return ghrelease.Get(s.http, url)
}

// getAsset issues the binary download on the long-timeout client.
func (s *Service) getAsset(url string) (*http.Response, error) {
	if s.download != nil {
		return ghrelease.Get(s.download, url)
	}
	return ghrelease.Get(s.http, url)
}
