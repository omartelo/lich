package agentplugin

import (
	"encoding/json"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/omartelo/lich/internal/providers"
)

// The Claude Code side: `claude plugin ...` for every mutation, and the CLI's
// own installed_plugins.json for what is installed.

func (s *Service) claudeInstall() error {
	if err := s.claudeSyncMarketplace(); err != nil {
		return err
	}
	return s.run(providers.Claude, "plugin", "install", pluginKey)
}

func (s *Service) claudeUpdate() error {
	if err := s.claudeSyncMarketplace(); err != nil {
		return err
	}
	return s.run(providers.Claude, "plugin", "update", pluginKey)
}

// claudeSyncMarketplace brings the local marketplace clone level with the
// remote: an add for the first install, an update for one already there.
//
// Both `plugin install` and `plugin update` refresh the marketplace themselves,
// but a failed refresh only downgrades them to a warning — they read the stale
// clone, report "already at the latest version" and exit 0. That is a success
// lich has no way to tell from a real one, so the refresh runs here first, where
// its failure is an error the user sees.
func (s *Service) claudeSyncMarketplace() error {
	// A repeat add errors ("already exists"), which is how an existing clone
	// announces itself; the update is the branch that then matters.
	err := s.run(providers.Claude, "plugin", "marketplace", "add", marketplaceRepo)
	if err == nil {
		return nil
	}
	slog.Debug("agentplugin: claude marketplace add", "err", err)
	return s.run(providers.Claude, "plugin", "marketplace", "update", marketplaceName)
}

// claudeInstalledVersion reads the plugin's installed version from Claude
// Code's plugin state, or ("", false) when absent or unreadable.
func claudeInstalledVersion() (string, bool) {
	dir := claudeConfigDir()
	if dir == "" {
		return "", false
	}
	data, err := os.ReadFile(filepath.Join(dir, "plugins", "installed_plugins.json"))
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
