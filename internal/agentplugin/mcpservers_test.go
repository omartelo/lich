package agentplugin

import (
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/omartelo/lich/internal/providers"
	"github.com/omartelo/lich/internal/relay"
)

// writeMCPConfig drops one JSON document into dir.
func writeMCPConfig(t *testing.T, dir, name, body string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("create %s: %v", dir, err)
	}
	if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
		t.Fatalf("write %s: %v", name, err)
	}
}

// opencodeConfigHome isolates opencode's global config directory the way its own
// resolver reads it — the xdg variable, on every OS, since opencode applies the
// convention rather than the OS-native location.
func opencodeConfigHome(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	t.Setenv("OPENCODE_CONFIG_DIR", "")
	t.Setenv("XDG_CONFIG_HOME", root)
	return filepath.Join(root, "opencode")
}

// The names are what a card divides `mcp__<server>_<tool>` against, so omp's own
// document is the list — lich's entry in it and every server beside it.
func TestMCPServersReadsOMPDocument(t *testing.T) {
	dir := ompHome(t)
	writeMCPConfig(t, dir, ompMCPFile, `{"mcpServers":{
		"lich":{"command":"/usr/bin/lich"},
		"ai-memory":{"command":"memory"}
	}}`)

	got := MCPServers(providers.OMP, t.TempDir())

	if want := []string{"ai-memory", "lich"}; !slices.Equal(got, want) {
		t.Errorf("MCPServers = %v, want %v", got, want)
	}
}

// opencode holds its servers under a different key, in the config document
// beside the plugin directory.
func TestMCPServersReadsOpenCodeGlobalConfig(t *testing.T) {
	dir := opencodeConfigHome(t)
	writeMCPConfig(t, dir, "opencode.jsonc", `{"mcp":{"ai-memory":{"type":"remote"}}}`)

	got := MCPServers(providers.OpenCode, t.TempDir())

	if want := []string{"ai-memory", "lich"}; !slices.Equal(got, want) {
		t.Errorf("MCPServers = %v, want %v", got, want)
	}
}

// opencode merges three documents into its global config, and a server declared
// in any of them is a server whose tools reach the session.
func TestMCPServersMergesEveryOpenCodeGlobalDocument(t *testing.T) {
	dir := opencodeConfigHome(t)
	writeMCPConfig(t, dir, "config.json", `{"mcp":{"first":{}}}`)
	writeMCPConfig(t, dir, "opencode.json", `{"mcp":{"second":{}}}`)

	got := MCPServers(providers.OpenCode, t.TempDir())

	if want := []string{"first", "lich", "second"}; !slices.Equal(got, want) {
		t.Errorf("MCPServers = %v, want %v", got, want)
	}
}

// A project registers servers of its own, in the four documents opencode reads
// out of every directory from the session's own up — which is why the answer is
// per session rather than per provider.
func TestMCPServersReadsOpenCodeProjectConfig(t *testing.T) {
	opencodeConfigHome(t)
	root := t.TempDir()
	deep := filepath.Join(root, "packages", "web")
	writeMCPConfig(t, root, "opencode.json", `{"mcp":{"repo-root":{}}}`)
	writeMCPConfig(t, filepath.Join(root, opencodeProjectDir), "opencode.jsonc",
		`{"mcp":{"repo-dot-dir":{}}}`)
	writeMCPConfig(t, deep, "opencode.jsonc", `{"mcp":{"package":{}}}`)

	got := MCPServers(providers.OpenCode, deep)

	want := []string{"lich", "package", "repo-dot-dir", "repo-root"}
	if !slices.Equal(got, want) {
		t.Errorf("MCPServers = %v, want %v", got, want)
	}
}

// A cwd that is not an absolute path is not a directory to walk up from: doing
// it anyway would climb lich's own working directory and report servers this
// session has no way to reach.
func TestMCPServersIgnoresARelativeCwd(t *testing.T) {
	opencodeConfigHome(t)

	for _, cwd := range []string{"", "relative/path"} {
		if got := MCPServers(providers.OpenCode, cwd); !slices.Equal(got, []string{relay.MCPServerName}) {
			t.Errorf("MCPServers(%q) = %v, want just lich's own", cwd, got)
		}
	}
}

// Every absence answers the same way, because the card's fallback for all of
// them is the same: show the tool name whole.
func TestMCPServersAnswersLichAloneWithoutADocument(t *testing.T) {
	ompHome(t)

	for _, tc := range []struct {
		name     string
		provider string
		setup    func() string
	}{
		{name: "no document at all", provider: providers.OMP},
		{
			name:     "a document lich cannot parse",
			provider: providers.OpenCode,
			setup: func() string {
				cwd := t.TempDir()
				writeMCPConfig(t, cwd, "opencode.json", `{"mcp": [not json`)
				return cwd
			},
		},
		{name: "a provider that keeps none", provider: providers.Claude},
		{name: "a provider lich does not know", provider: "nonesuch"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cwd := opencodeConfigHome(t)
			if tc.setup != nil {
				cwd = tc.setup()
			}
			got := MCPServers(tc.provider, cwd)
			if want := []string{relay.MCPServerName}; !slices.Equal(got, want) {
				t.Errorf("MCPServers = %v, want %v", got, want)
			}
		})
	}
}

// $OPENCODE_CONFIG_DIR moves the whole global config directory, and a lich
// reading the xdg one there would report servers no session has.
func TestMCPServersFollowsOpenCodeConfigDirOverride(t *testing.T) {
	opencodeConfigHome(t)
	override := t.TempDir()
	t.Setenv("OPENCODE_CONFIG_DIR", override)
	writeMCPConfig(t, override, "opencode.json", `{"mcp":{"elsewhere":{}}}`)

	got := MCPServers(providers.OpenCode, t.TempDir())

	if want := []string{"elsewhere", "lich"}; !slices.Equal(got, want) {
		t.Errorf("MCPServers = %v, want %v", got, want)
	}
}

// The extension exists for the comments encoding/json rejects, and opencode's
// own parser takes them — so a config lich refused would be a config the harness
// loads and the card cannot see.
func TestMCPServersReadsACommentedJSONC(t *testing.T) {
	opencodeConfigHome(t)
	cwd := t.TempDir()
	writeMCPConfig(t, cwd, "opencode.jsonc", `{
		// the one lich registers
		"$schema": "https://opencode.ai/config.json",
		"mcp": {
			"documented": { "type": "remote", "url": "http://127.0.0.1:1/mcp" }, /* kept */
		},
	}`)

	got := MCPServers(providers.OpenCode, cwd)

	if want := []string{"documented", "lich"}; !slices.Equal(got, want) {
		t.Errorf("MCPServers = %v, want %v", got, want)
	}
}

// $OPENCODE_CONFIG names one more document on top of the rest.
func TestMCPServersReadsTheOpenCodeConfigOverride(t *testing.T) {
	opencodeConfigHome(t)
	extra := filepath.Join(t.TempDir(), "extra.json")
	if err := os.WriteFile(extra, []byte(`{"mcp":{"extra":{}}}`), 0o644); err != nil {
		t.Fatalf("write %s: %v", extra, err)
	}
	t.Setenv("OPENCODE_CONFIG", extra)

	got := MCPServers(providers.OpenCode, t.TempDir())

	if want := []string{"extra", "lich"}; !slices.Equal(got, want) {
		t.Errorf("MCPServers = %v, want %v", got, want)
	}
}

// A `//` inside a string is not a comment, and neither is a `/*`. Asserted on
// jsonc directly: the failure it guards against is a URL cut in half, which no
// document-level assertion would name.
func TestJSONCLeavesStringsAlone(t *testing.T) {
	for _, tc := range []struct{ name, in, want string }{
		{"a url survives", `{"url":"http://x/y"}`, `{"url":"http://x/y"}`},
		{"a block opener in a string survives", `{"p":"/* not a comment"}`, `{"p":"/* not a comment"}`},
		{"an escaped quote does not end the string", `{"p":"a \" // b"}`, `{"p":"a \" // b"}`},
		{"a line comment goes, its newline stays", "{}// x\n{}", "{}\n{}"},
		{"a block comment goes", "{/* x */}", "{}"},
		{"an unterminated block takes the rest", "{}/* x", "{}"},
		{"a trailing comma goes", `{"a":1,}`, `{"a":1}`},
		{"one behind a comment goes too", "{\"a\":1, /* x */ }", `{"a":1}`},
		{"a separating comma stays", `{"a":1,"b":2}`, `{"a":1,"b":2}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := string(jsonc([]byte(tc.in))); got != tc.want {
				t.Errorf("jsonc(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}
