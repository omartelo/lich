package agentplugin

import (
	"fmt"
	"os"
	"path/filepath"
)

// The opencode side. opencode has no plugin CLI and no marketplace: a plugin
// there is a JavaScript module its server imports from a directory, so
// installing one is writing the file and uninstalling one is deleting it.
//
// lich writes the module the release published, with a marker line naming the
// version above it — that line is the only record of what is installed, since
// opencode keeps no plugin state to ask.

const (
	// opencodeSource is the module's path inside the plugin repository, and
	// opencodeFile the name it takes in opencode's plugin directory. The
	// extension is load-bearing: opencode globs `{plugin,plugins}/*.{ts,js}`, so
	// a `.mjs` would sit there unread.
	opencodeSource = "opencode/lich.js"
	opencodeFile   = "lich.js"

	// jsComment is how the marker line hides from the JavaScript that follows it.
	jsComment = "//"
)

func (s *Service) opencodeInstall() error {
	version, err := s.releaseVersion()
	if err != nil {
		return err
	}
	data, err := s.fetchFile(version, opencodeSource)
	if err != nil {
		return err
	}
	dir, err := s.opencodePluginDir()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create %s: %w", dir, err)
	}
	body := fmt.Sprintf("%s %s v%s — installed by lich, replace with the file from %s\n%s",
		jsComment, markerName, version, marketplaceRepo, data)
	return writeFile(filepath.Join(dir, opencodeFile), []byte(body), 0o644)
}

// opencodeInstalledVersion reads the version off the marker line lich wrote, or
// ("", false) when the module is absent, unreadable, or was not written by lich
// — a hand-installed copy carries no marker, and reporting a version for it
// would claim an install lich cannot update.
func (s *Service) opencodeInstalledVersion() (string, bool) {
	dir, err := s.opencodePluginDir()
	if err != nil {
		return "", false
	}
	data, err := os.ReadFile(filepath.Join(dir, opencodeFile))
	if err != nil {
		return "", false
	}
	return versionOf(data, jsComment)
}

// opencodePluginDir resolves opencode's global plugin directory. opencode reads
// it through the xdg-basedir convention — $XDG_CONFIG_HOME, else ~/.config —
// which it applies on every platform rather than deferring to the OS-native
// config location, so os.UserConfigDir would point elsewhere on macOS and
// Windows and the module would land where nothing loads it.
func (s *Service) opencodePluginDir() (string, error) {
	base := os.Getenv("XDG_CONFIG_HOME")
	if base == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("resolve home directory: %w", err)
		}
		base = filepath.Join(home, ".config")
	}
	return filepath.Join(base, "opencode", "plugin"), nil
}
