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
	if err := s.claudePinMarketplace(); err != nil {
		return err
	}
	return s.run(providers.Claude, "plugin", "install", pluginKey)
}

// claudePinMarketplace declares the marketplace at the tag of the newest release
// this lich is compatible with, so neither an install nor Claude Code's own
// marketplace refresh can move the plugin past it.
//
// The declaration is removed first because Claude Code refuses an add whose ref
// differs from the one already declared, and has no command to change a ref.
// The remove also uninstalls the plugin, which is why Update is this same path
// and not `plugin update`: the install right after puts it back at the pinned
// release, a downgrade included. A remove with nothing declared fails, which is
// the first install and not an error. All measured on Claude Code 2.1.270.
func (s *Service) claudePinMarketplace() error {
	version, err := s.releaseVersion()
	if err != nil {
		return err
	}
	if err := s.run(providers.Claude, "plugin", "marketplace", "remove", marketplaceName); err != nil {
		slog.Debug("agentplugin: claude marketplace remove", "err", err)
	}
	return s.run(providers.Claude, "plugin", "marketplace", "add", gitURL+"#v"+version)
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
