package agentplugin

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/omartelo/lich/internal/providers"
	"github.com/omartelo/lich/internal/relay"
)

// The Antigravity side. Antigravity does have a plugin CLI — `agy plugin
// install` — and lich does not use it, for one reason: it installs a directory,
// and the only remote form it takes clones that repository's default branch.
// lich installs a *release*: the version it wrote is what a card reports and
// what the next update compares against, and a clone of whatever is on main
// carries no version at all. So this is a file-shipped install like opencode's
// — lich writes the customization directory Antigravity discovers, and the
// files being there is the install.
//
// Three writes: the hook registration, the scripts it names, and a manifest.
// The registration's commands are relative (`hooks/report-state.sh`), which is
// what a hook run by Antigravity resolves against — it runs one through `sh -c`
// with the working directory set to the folder holding hooks.json, and sets no
// plugin-root variable of its own (both measured on 1.1.19). So the layout lich
// writes here is not a choice: it is the registration's own assumption.
//
// The whole directory belongs to lich, which is why the version can live in the
// manifest rather than in a marker line hidden in a comment.

const (
	// antigravityHooksFile is the registration, at the plugin repository's root
	// and under the same name in the installed directory. Neither name is
	// configurable: Antigravity discovers a plugin as `plugins/<name>/plugin.json`
	// with its `hooks.json` beside it.
	antigravityHooksFile = "hooks.json"

	// antigravityManifest is what marks the directory as a plugin rather than a
	// stray folder under `plugins/`.
	antigravityManifest = "plugin.json"

	// antigravityScriptDir is the directory the registration's relative commands
	// name, both in the repository and in the install.
	antigravityScriptDir = "hooks"
)

// antigravityScripts is every file the registration runs or reads, fetched from
// the release and written beside it. `detail.jq` is not a hook: the state and
// tool reports read it with `jq -r -f`, so it has to land with them.
var antigravityScripts = []string{
	"hooks/report-session-start.sh",
	"hooks/report-state.sh",
	"hooks/report-title.sh",
	"hooks/report-tool.sh",
	"hooks/report-touched.sh",
	"hooks/detail.jq",
}

func (s *Service) antigravityInstall() error {
	version, err := s.releaseVersion()
	if err != nil {
		return err
	}
	dir, err := antigravityPluginDir()
	if err != nil {
		return err
	}
	hooks, err := s.fetchFile(version, antigravityHooksFile)
	if err != nil {
		return err
	}
	scripts := make(map[string][]byte, len(antigravityScripts))
	for _, source := range antigravityScripts {
		data, err := s.fetchFile(version, source)
		if err != nil {
			return err
		}
		scripts[filepath.Base(source)] = data
	}
	if err := os.MkdirAll(filepath.Join(dir, antigravityScriptDir), 0o755); err != nil {
		return fmt.Errorf("create %s: %w", dir, err)
	}
	for name, data := range scripts {
		// Executable: Antigravity runs a hook command through `sh -c`, which
		// needs the bit on a shebang'd script the way any other shell would.
		// detail.jq is read rather than run, and carrying one mode for both is
		// cheaper than a table saying which is which.
		if err := writeFile(filepath.Join(dir, antigravityScriptDir, name), data, 0o755); err != nil {
			return err
		}
	}
	if err := writeFile(filepath.Join(dir, antigravityHooksFile), hooks, 0o644); err != nil {
		return err
	}
	manifest, err := antigravityManifestFile(version)
	if err != nil {
		return err
	}
	if err := writeFile(filepath.Join(dir, antigravityManifest), manifest, 0o644); err != nil {
		return err
	}
	return s.antigravityRegisterMCP()
}

// antigravityManifestFile is the manifest lich writes, carrying the released
// version it just installed. The whole directory is lich's, so this file is the
// record a marker line is elsewhere — and a manifest without the field is a
// copy somebody else installed, which lich reports as not installed rather than
// claiming a version it cannot update.
func antigravityManifestFile(version string) ([]byte, error) {
	body, err := json.MarshalIndent(map[string]string{
		"name":        pluginName,
		"version":     version,
		"description": "Installed by lich — replace with the plugin from " + marketplaceRepo,
	}, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("encode %s: %w", antigravityManifest, err)
	}
	return append(body, '\n'), nil
}

// antigravityRegisterMCP gives an Antigravity session the operations for driving
// the sessions beside it. `agy mcp add` is the supported interface — it owns the
// document and its formatting, which is the whole reason lich shells out instead
// of merging JSON the way it must for oh-my-pi — and it updates an existing
// entry rather than refusing one, so a reinstall is not a special case.
//
// A lich that cannot name its own binary registers nothing rather than a command
// that cannot run: the reports are the half Antigravity needs to be useful, and
// they do not depend on it.
func (s *Service) antigravityRegisterMCP() error {
	lichBin := s.lichBin()
	if lichBin == "" {
		return nil
	}
	return s.run(providers.Antigravity, "mcp", "add", relay.MCPServerName, lichBin, relay.MCPSubcommand)
}

// antigravityInstalledVersion reads the version off the manifest lich wrote, or
// ("", false) when the plugin is absent, unreadable, or was not written by lich.
func (s *Service) antigravityInstalledVersion() (string, bool) {
	dir, err := antigravityPluginDir()
	if err != nil {
		return "", false
	}
	data, err := os.ReadFile(filepath.Join(dir, antigravityManifest))
	if err != nil {
		return "", false
	}
	var manifest struct {
		Version string `json:"version"`
	}
	if err := json.Unmarshal(data, &manifest); err != nil || manifest.Version == "" {
		return "", false
	}
	return manifest.Version, true
}

// antigravityPluginDir resolves the directory Antigravity discovers global
// customizations in — `~/.gemini/config/plugins/<name>/`, one directory per
// plugin. The root answers to no environment variable of its own: 1.1.19 falls
// back to a hardcoded ".gemini" under the home when it cannot resolve one, so
// the home is the only thing to honour, and internal/sandbox and
// internal/terminal/transcript.go resolve it the same way.
func antigravityPluginDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolve home directory: %w", err)
	}
	return filepath.Join(home, ".gemini", "config", "plugins", pluginName), nil
}
