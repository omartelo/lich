package agentplugin

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/omartelo/lich/internal/providers"
	"github.com/omartelo/lich/internal/relay"
)

// The Cursor CLI side, which is not like the other six: lich ships it no hooks
// at all. Cursor executes every Claude Code hook on the machine — the user's own
// and each installed plugin's, `${CLAUDE_PLUGIN_ROOT}` expanded (measured on
// 2026.08.11) — so the plugin installed in Claude Code is already running inside
// every Cursor session, reporting the state, the title, the touched files and
// the chat id. There is nothing here for lich to write that Claude Code's own
// install does not already put there, which is why installing *for* Cursor is
// installing for Claude Code, and why the version reported for Cursor is the
// version installed there. A machine without Claude Code has a Cursor session
// that reports nothing, and this reports it as not installed, which is true.
//
// What that route does not carry is the MCP registration: Cursor takes no
// server on its command line, and a Claude Code plugin declares none. That is
// the one file this writes — an `mcpServers` document under `~/.cursor`, merged
// into whatever is already there, the same trade omp's makes.

const (
	// cursorMCPFile is the document Cursor reads MCP servers from. It sits under
	// `~/.cursor` — resolved off the home with no variable in the way, unlike
	// the config directory that holds the credentials and the chats.
	cursorMCPFile = "mcp.json"
)

// cursorInstall registers lich's MCP server with Cursor. It refuses while the
// plugin is not installed in Claude Code, because that install is where a Cursor
// session's reports come from: registering the tools alone would leave a card
// that answers with lich's own operations and never says it is working.
//
// It does not install into Claude Code itself. That is a second provider's
// state, one button away on the same screen, and an install that silently
// changes another provider is not a thing lich does.
func (s *Service) cursorInstall() error {
	if _, ok := claudeInstalledVersion(); !ok {
		return fmt.Errorf(
			"%s runs the plugin installed in Claude Code: install it there first",
			providerName(providers.Cursor))
	}
	// Merged before anything is written, for the reason omp's install gives: the
	// only way this step fails is a document lich must not replace.
	path, err := cursorMCPPath()
	if err != nil {
		return err
	}
	registration, err := mcpDocument(path, s.lichBin())
	if err != nil {
		return err
	}
	if registration == nil {
		return fmt.Errorf("lich cannot resolve its own binary to register with %s",
			providerName(providers.Cursor))
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create %s: %w", filepath.Dir(path), err)
	}
	return writeFile(path, registration, 0o644)
}

// cursorInstalledVersion is the Claude Code install's version, and only once
// lich's server is registered with Cursor: the reports come from that install
// and the tools from this registration, so a session has the plugin only when
// both are there. ("", false) when either half is missing.
func (s *Service) cursorInstalledVersion() (string, bool) {
	if !cursorRegistered() {
		return "", false
	}
	return claudeInstalledVersion()
}

// cursorRegistered reports whether lich's own server is in Cursor's MCP
// document. False for every failure — no file, a document lich cannot parse, an
// entry under another name — which reads as "not installed" and is what the
// install overwrites.
func cursorRegistered() bool {
	path, err := cursorMCPPath()
	if err != nil {
		return false
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return false
	}
	var config struct {
		Servers map[string]json.RawMessage `json:"mcpServers"`
	}
	if err := json.Unmarshal(data, &config); err != nil {
		return false
	}
	_, ok := config.Servers[relay.MCPServerName]
	return ok
}

// cursorMCPPath is `~/.cursor/mcp.json`. Cursor reads this one off the home
// directly: $CURSOR_CONFIG_DIR and $XDG_CONFIG_HOME move the credentials and the
// chats, and leave this file where it is (measured on 2026.08.11).
func cursorMCPPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolve home directory: %w", err)
	}
	return filepath.Join(home, ".cursor", cursorMCPFile), nil
}
