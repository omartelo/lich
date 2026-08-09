package agentplugin

import (
	"bytes"
	"encoding/json"

	"github.com/omartelo/lich/internal/providers"
)

// The Codex side: `codex plugin ...` for every mutation, and `codex plugin list
// --json` for what is installed. Codex has no `plugin update`, so an update is
// an upgraded marketplace snapshot followed by the same install — which is
// idempotent there, unlike Claude Code's.

func (s *Service) codexInstall() error {
	if err := s.codexSyncMarketplace(); err != nil {
		return err
	}
	return s.run(providers.Codex, "plugin", "add", pluginKey)
}

func (s *Service) codexUpdate() error {
	if err := s.codexSyncMarketplace(); err != nil {
		return err
	}
	return s.run(providers.Codex, "plugin", "add", pluginKey)
}

// codexSyncMarketplace makes sure the marketplace is configured and its snapshot
// current. Unlike Claude Code's, a repeat `marketplace add` succeeds, so both
// calls run every time: the add is the first install's, the upgrade is what
// pulls a newer release into an existing snapshot. `plugin add` installs from
// the snapshot alone, so skipping the upgrade would reinstall the same version.
func (s *Service) codexSyncMarketplace() error {
	if err := s.run(providers.Codex, "plugin", "marketplace", "add", marketplaceRepo); err != nil {
		return err
	}
	return s.run(providers.Codex, "plugin", "marketplace", "upgrade", marketplaceName)
}

// codexInstalledVersion reads the plugin's installed version from Codex's own
// plugin listing, or ("", false) when the plugin is absent or the call fails.
func (s *Service) codexInstalledVersion() (string, bool) {
	out, err := s.read(providers.Codex, "plugin", "list", "--json")
	if err != nil {
		return "", false
	}
	return parseCodexPluginList([]byte(out), pluginKey)
}

// parseCodexPluginList pulls the plugin's version out of `codex plugin list
// --json`. The command prints a banner on some setups, so the JSON object is
// taken from the first brace on — anything before it is not part of the
// document.
func parseCodexPluginList(data []byte, key string) (string, bool) {
	start := bytes.IndexByte(data, '{')
	if start < 0 {
		return "", false
	}
	var doc struct {
		Installed []struct {
			PluginID  string `json:"pluginId"`
			Version   string `json:"version"`
			Installed bool   `json:"installed"`
		} `json:"installed"`
	}
	if err := json.Unmarshal(data[start:], &doc); err != nil {
		return "", false
	}
	for _, p := range doc.Installed {
		if p.PluginID == key && p.Installed && p.Version != "" {
			return p.Version, true
		}
	}
	return "", false
}
