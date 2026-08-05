package themes

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/omartelo/lich/internal/semver"
)

const (
	cloneTimeout     = 90 * time.Second
	remoteMaxLength  = 512
	maxThemesPerPack = 32
)

// scpLike matches git's shorthand remote (user@host:path), the form an ssh
// remote is usually copied in.
var scpLike = regexp.MustCompile(`^[a-zA-Z0-9._-]+@[a-zA-Z0-9._-]+:[^\s]+$`)

// GitInstallResult reports what installing a repository produced. Conflicts is
// non-empty only when the install was refused because those theme ids already
// exist locally; nothing was written in that case.
type GitInstallResult struct {
	Pack      string   `json:"pack"`
	Version   string   `json:"version"`
	Themes    []Theme  `json:"themes"`
	Conflicts []string `json:"conflicts"`
	UpToDate  bool     `json:"upToDate"`
}

// pack is a validated theme repository: its manifest plus every theme the
// clone carried, already stamped with their source.
type pack struct {
	manifest Manifest
	themes   []Theme
}

// InstallFromGit clones a theme repository and installs every theme in it. The
// install is all-or-nothing: one unreadable or invalid theme fails the whole
// repository instead of leaving half a pack behind.
func (s *Service) InstallFromGit(url string, overwrite bool) (GitInstallResult, error) {
	if s.initErr != nil {
		return GitInstallResult{}, s.initErr
	}
	installed, err := fetchPack(url)
	if err != nil {
		return GitInstallResult{}, err
	}
	return s.installPack(installed, overwrite)
}

// UpdateFromGit re-clones the repository a theme came from and installs it
// again when the manifest carries a newer version. The whole pack moves
// together — the repository, not the single theme, is what is versioned.
func (s *Service) UpdateFromGit(id string) (GitInstallResult, error) {
	if s.initErr != nil {
		return GitInstallResult{}, s.initErr
	}
	theme, err := s.read(id)
	if err != nil {
		return GitInstallResult{}, err
	}
	if theme.Source == nil {
		return GitInstallResult{}, fmt.Errorf("theme %q was imported as a single file and has no repository to update from", id)
	}
	latest, err := fetchPack(theme.Source.URL)
	if err != nil {
		return GitInstallResult{}, err
	}
	result := GitInstallResult{Pack: latest.manifest.Name, Version: latest.manifest.version()}
	if !semver.Less(theme.Source.Version, result.Version) {
		result.UpToDate = true
		return result, nil
	}
	return s.installPack(latest, true)
}

func (s *Service) installPack(installed pack, overwrite bool) (GitInstallResult, error) {
	result := GitInstallResult{
		Pack:    installed.manifest.Name,
		Version: installed.manifest.version(),
	}
	if !overwrite {
		for _, theme := range installed.themes {
			if _, err := os.Stat(s.path(theme.ID)); err == nil {
				result.Conflicts = append(result.Conflicts, theme.ID)
			}
		}
		if len(result.Conflicts) > 0 {
			return result, nil
		}
	}
	if err := os.MkdirAll(s.dir, 0o700); err != nil {
		return GitInstallResult{}, fmt.Errorf("create themes dir: %w", err)
	}
	for _, theme := range installed.themes {
		if _, err := s.install(theme, true); err != nil {
			return GitInstallResult{}, err
		}
	}
	result.Themes = installed.themes
	return result, nil
}

// fetchPack clones url into a temporary directory and reads the pack out of
// it. The clone is thrown away: an update is another clone, which costs one
// shallow fetch and saves reconciling a cache nobody watches.
func fetchPack(url string) (pack, error) {
	remote, err := validateRemote(url)
	if err != nil {
		return pack{}, err
	}
	tmp, err := os.MkdirTemp("", "lich-theme-repo-")
	if err != nil {
		return pack{}, fmt.Errorf("create temp dir: %w", err)
	}
	defer func() { _ = os.RemoveAll(tmp) }()
	dir := filepath.Join(tmp, "repo")
	ctx, cancel := context.WithTimeout(context.Background(), cloneTimeout)
	defer cancel()
	if err := clone(ctx, remote, dir); err != nil {
		return pack{}, err
	}
	return readPack(dir, remote)
}

func clone(ctx context.Context, url, dir string) error {
	cmd := exec.CommandContext(ctx, "git",
		// ext:: resolves through a remote helper that runs a shell command.
		// validateRemote already rejects it; this makes the ban git's too.
		"-c", "protocol.ext.allow=never",
		"-c", "credential.helper=",
		"clone", "--depth", "1", "--single-branch", "--no-tags", "--", url, dir)
	// Without these a private repository stops on a credential prompt that has
	// no terminal to answer it, and the install hangs until the timeout.
	cmd.Env = append(os.Environ(),
		"GIT_TERMINAL_PROMPT=0",
		"GIT_ASKPASS=",
		"SSH_ASKPASS=",
		"GCM_INTERACTIVE=never",
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		if ctx.Err() != nil {
			return fmt.Errorf("clone %s: timed out after %s", url, cloneTimeout)
		}
		return fmt.Errorf("clone %s: %s", url, gitError(out, err))
	}
	return nil
}

// gitError reduces git's output to its last meaningful line, which is the one
// naming the failure ("repository not found", "could not read Username").
func gitError(out []byte, err error) string {
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	for i := len(lines) - 1; i >= 0; i-- {
		line := strings.TrimSpace(lines[i])
		if line != "" {
			return line
		}
	}
	return err.Error()
}

// readPack validates a cloned working tree: the manifest first, then every
// other JSON beside it as a theme.
func readPack(dir, url string) (pack, error) {
	raw, err := readThemeFile(filepath.Join(dir, manifestName))
	if err != nil {
		return pack{}, fmt.Errorf("repository has no readable %s: %w", manifestName, err)
	}
	var manifest Manifest
	if err := json.Unmarshal(raw, &manifest); err != nil {
		return pack{}, fmt.Errorf("parse %s: %w", manifestName, err)
	}
	if err := manifest.validate(); err != nil {
		return pack{}, err
	}
	files, err := themeFiles(dir)
	if err != nil {
		return pack{}, err
	}
	out := pack{manifest: manifest}
	seen := make(map[string]struct{}, len(files))
	for _, name := range files {
		theme, err := readPackTheme(filepath.Join(dir, name), url, manifest.version())
		if err != nil {
			return pack{}, fmt.Errorf("%s: %w", name, err)
		}
		if _, dup := seen[theme.ID]; dup {
			return pack{}, fmt.Errorf("%s: theme id %q is used by more than one file", name, theme.ID)
		}
		seen[theme.ID] = struct{}{}
		out.themes = append(out.themes, theme)
	}
	return out, nil
}

func readPackTheme(path, url, version string) (Theme, error) {
	data, err := readThemeFile(path)
	if err != nil {
		return Theme{}, err
	}
	var theme Theme
	if err := json.Unmarshal(data, &theme); err != nil {
		return Theme{}, fmt.Errorf("parse theme JSON: %w", err)
	}
	theme.Origin = OriginCustom
	theme.Source = &Source{URL: url, Version: version}
	if err := validateCustom(theme); err != nil {
		return Theme{}, err
	}
	return theme, nil
}

func themeFiles(dir string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("read repository: %w", err)
	}
	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || name == manifestName || filepath.Ext(name) != ".json" {
			continue
		}
		names = append(names, name)
	}
	if len(names) == 0 {
		return nil, fmt.Errorf("repository has no theme JSON next to %s", manifestName)
	}
	if len(names) > maxThemesPerPack {
		return nil, fmt.Errorf("repository holds more than %d themes", maxThemesPerPack)
	}
	sort.Strings(names)
	return names, nil
}

// validateRemote is this package's trust boundary. git resolves an ext:: URL
// through a remote helper that runs a shell command, and an argument starting
// with "-" is read as a flag, so a URL typed into the UI is checked against a
// transport allowlist before it ever reaches git.
func validateRemote(url string) (string, error) {
	remote := strings.TrimSpace(url)
	switch {
	case remote == "":
		return "", fmt.Errorf("repository URL is required")
	case len(remote) > remoteMaxLength:
		return "", fmt.Errorf("repository URL cannot exceed %d characters", remoteMaxLength)
	case strings.HasPrefix(remote, "-"):
		return "", fmt.Errorf("repository URL cannot start with %q", "-")
	case strings.HasPrefix(remote, "https://"),
		strings.HasPrefix(remote, "ssh://"),
		strings.HasPrefix(remote, "file://"),
		filepath.IsAbs(remote),
		scpLike.MatchString(remote):
		return remote, nil
	}
	return "", fmt.Errorf("repository URL must be https://, ssh://, file://, an absolute path or user@host:path")
}

func validateSource(source *Source) error {
	if source == nil {
		return nil
	}
	if _, err := validateRemote(source.URL); err != nil {
		return fmt.Errorf("theme source: %w", err)
	}
	if !semver.IsRelease(source.Version) {
		return fmt.Errorf("theme source version %q must be MAJOR.MINOR.PATCH", source.Version)
	}
	return nil
}
