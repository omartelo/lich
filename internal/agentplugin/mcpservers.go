package agentplugin

import (
	"bytes"
	"encoding/json"
	"maps"
	"os"
	"path/filepath"
	"slices"

	"github.com/omartelo/lich/internal/providers"
	"github.com/omartelo/lich/internal/relay"
)

// The MCP server names a card reads a tool report against. Two harnesses spell
// an MCP tool with a single underscore between the server and the tool
// (oh-my-pi's `mcp__<server>_<tool>`, opencode's `<server>_<tool>`), which
// divides only against a list of the servers that exist — so lich reads the
// documents those two take their servers from, at the spawn that will report the
// tools, and hands the names to the card.
//
// Every other provider is answered with lich's own name alone: their tool names
// either carry the doubled underscore the card splits without help, or are not a
// tool name at all (Antigravity's, docs/hooks/session-state.md).

// opencodeMCPKey is the key opencode holds its servers under.
const opencodeMCPKey = "mcp"

// opencodeConfigFiles are the documents opencode reads a config from, in every
// directory it looks in: its global config directory, then `.opencode/` and the
// session's own directory on the way up from the cwd. `config.json` is the
// global-only third name it also merges. Measured off opencode 1.18.18's own
// `Config.loadInstanceState` and `ConfigPaths`.
var (
	opencodeConfigFiles       = []string{"opencode.json", "opencode.jsonc"}
	opencodeGlobalConfigFiles = append([]string{"config.json"}, opencodeConfigFiles...)
)

// opencodeProjectDir is the directory name opencode also reads a config out of,
// in the cwd and in every directory above it.
const opencodeProjectDir = ".opencode"

// MCPServers names the MCP servers whose tools a session of this provider,
// started in cwd, can call — sorted, with lich's own always among them, since
// lich is registered wherever it spawns a session.
func MCPServers(provider, cwd string) []string {
	names := map[string]bool{relay.MCPServerName: true}
	for _, name := range registeredServers(provider, cwd) {
		names[name] = true
	}
	return slices.Sorted(maps.Keys(names))
}

// registeredServers reads the provider's own documents, or answers nothing at
// all — an absent file, one lich cannot parse and a provider that keeps none are
// the same answer, and it is the one that leaves a tool name whole.
func registeredServers(provider, cwd string) []string {
	switch provider {
	case providers.OMP:
		dir, err := ompAgentDir()
		if err != nil {
			return nil
		}
		return serverKeys(filepath.Join(dir, ompMCPFile), mcpServersKey)
	case providers.OpenCode:
		return opencodeServers(cwd)
	}
	return nil
}

// opencodeServers reads every document opencode merges its config from: the
// global directory, then — because a project may register a server of its own —
// `opencode.json`, `opencode.jsonc` and `.opencode/`'s pair in cwd and in every
// directory above it. opencode stops that walk at the project root and lich does
// not, which makes this a superset: the cost of the extra names is a split lich
// would not otherwise offer, and the cost of stopping short is a tool the card
// cannot divide.
func opencodeServers(cwd string) []string {
	var out []string
	if dir, err := opencodeConfigDir(); err == nil {
		out = append(out, dirServers(dir, opencodeGlobalConfigFiles)...)
	}
	// $OPENCODE_CONFIG names one more document, merged on top of the rest rather
	// than replacing them (measured on 1.18.18).
	if extra := os.Getenv("OPENCODE_CONFIG"); extra != "" {
		out = append(out, serverKeys(extra, opencodeMCPKey)...)
	}
	for _, dir := range ancestors(cwd) {
		out = append(out, dirServers(dir, opencodeConfigFiles)...)
		out = append(out, dirServers(filepath.Join(dir, opencodeProjectDir), opencodeConfigFiles)...)
	}
	return out
}

// dirServers is every server named by any of files in dir.
func dirServers(dir string, files []string) []string {
	var out []string
	for _, file := range files {
		out = append(out, serverKeys(filepath.Join(dir, file), opencodeMCPKey)...)
	}
	return out
}

// ancestors is dir and every directory above it, up to the filesystem root.
// Empty for a relative or empty path: a walk from one would climb whatever the
// process's own working directory happens to be, which is not the session's.
func ancestors(dir string) []string {
	if !filepath.IsAbs(dir) {
		return nil
	}
	var out []string
	for {
		out = append(out, dir)
		parent := filepath.Dir(dir)
		if parent == dir {
			return out
		}
		dir = parent
	}
}

// serverKeys are the keys of the object under key in the JSON document at path.
func serverKeys(path, key string) []string {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var doc map[string]json.RawMessage
	if err := json.Unmarshal(jsonc(data), &doc); err != nil {
		return nil
	}
	var servers map[string]json.RawMessage
	if err := json.Unmarshal(doc[key], &servers); err != nil {
		return nil
	}
	return slices.Collect(maps.Keys(servers))
}

// opencodeConfigDir is where opencode merges its global config from: the parent
// of the plugin directory, resolved the same xdg way, with $OPENCODE_CONFIG_DIR
// moving it outright (measured on 1.18.18).
func opencodeConfigDir() (string, error) {
	if dir := os.Getenv("OPENCODE_CONFIG_DIR"); dir != "" {
		return dir, nil
	}
	dir, err := opencodePluginDir()
	if err != nil {
		return "", err
	}
	return filepath.Dir(dir), nil
}

// jsonc strips what a JSON document may not carry and a `.jsonc` one may: `//`
// and `/* */` comments, and a comma before a closing brace or bracket. opencode
// parses its config with both allowed (measured on 1.18.18), so a lich that
// refused them would miss servers the harness itself loads — and the extension
// exists for exactly the comments encoding/json rejects.
//
// String literals are copied through untouched, which is what keeps a URL's
// `//` and a prompt's `/*` intact.
func jsonc(data []byte) []byte {
	out := make([]byte, 0, len(data))
	inString, escaped := false, false
	for i := 0; i < len(data); i++ {
		c := data[i]
		if inString {
			out = append(out, c)
			switch {
			case escaped:
				escaped = false
			case c == '\\':
				escaped = true
			case c == '"':
				inString = false
			}
			continue
		}
		switch {
		case c == '"':
			inString = true
			out = append(out, c)
		case c == '/' && i+1 < len(data) && data[i+1] == '/':
			for i < len(data) && data[i] != '\n' {
				i++
			}
			// Back one, so the loop's own step lands on the newline and keeps it:
			// dropping it would join this line to the next.
			i--
		case c == '/' && i+1 < len(data) && data[i+1] == '*':
			end := bytes.Index(data[i+2:], []byte("*/"))
			if end < 0 {
				// Unterminated: everything after it is comment as far as any
				// parser is concerned, and what is left may still parse.
				return out
			}
			i += 2 + end + 1
		case c == '}' || c == ']':
			// The comma before a closer is trailing. Taken off what has already
			// been written rather than looked for ahead: a comment can sit
			// between the two, and by here it is gone.
			out = append(dropTrailingComma(out), c)
		default:
			out = append(out, c)
		}
	}
	return out
}

// dropTrailingComma removes the last comma in out when nothing but space
// follows it, along with the space between. Called only where a closer is about
// to be written, which is the one place such a comma is trailing.
func dropTrailingComma(out []byte) []byte {
	trimmed := bytes.TrimRight(out, " \t\r\n")
	if len(trimmed) > 0 && trimmed[len(trimmed)-1] == ',' {
		return trimmed[:len(trimmed)-1]
	}
	return out
}
