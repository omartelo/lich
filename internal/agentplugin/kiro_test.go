package agentplugin

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/omartelo/lich/internal/providers"
)

// decodeKiroAgent parses the agent file lich writes back into the shape Kiro
// reads, so the assertions below are about the bytes on disk rather than about
// the struct that produced them.
func decodeKiroAgent(t *testing.T, version, scriptDir string) map[string]any {
	t.Helper()
	data, err := kiroAgentFile(version, scriptDir)
	if err != nil {
		t.Fatalf("kiroAgentFile = %v, want nil", err)
	}
	var agent map[string]any
	if err := json.Unmarshal(data, &agent); err != nil {
		t.Fatalf("agent file is not JSON: %v", err)
	}
	return agent
}

// hookCommands returns the commands registered for one event, in order.
func hookCommands(t *testing.T, agent map[string]any, event string) []string {
	t.Helper()
	hooks, ok := agent["hooks"].(map[string]any)
	if !ok {
		t.Fatalf("agent has no hooks block")
	}
	entries, ok := hooks[event].([]any)
	if !ok {
		return nil
	}
	var out []string
	for _, entry := range entries {
		e, ok := entry.(map[string]any)
		if !ok {
			t.Fatalf("hook entry for %q is not an object", event)
		}
		command, _ := e["command"].(string)
		out = append(out, command)
	}
	return out
}

// TestKiroAgentRegistersEveryEventKiroHas pins the five hook event names against
// the enum Kiro's own parser accepts. They are camelCase and case-sensitive: a
// `PreToolUse` borrowed from Claude Code is rejected by `kiro-cli agent
// validate`, and a name outside the set is the silent no-op the provider guide
// warns a copied event becomes.
func TestKiroAgentRegistersEveryEventKiroHas(t *testing.T) {
	agent := decodeKiroAgent(t, "0.12.0", "/opt/lich/hooks")
	hooks, ok := agent["hooks"].(map[string]any)
	if !ok {
		t.Fatalf("agent has no hooks block")
	}
	var got []string
	for event := range hooks {
		got = append(got, event)
	}
	slices.Sort(got)
	want := []string{"agentSpawn", "postToolUse", "preToolUse", "stop", "userPromptSubmit"}
	if !slices.Equal(got, want) {
		t.Errorf("registered events = %v, want %v", got, want)
	}
}

// TestKiroAgentReportsTheRightStates proves each contract rides the event that
// can actually close it: the session id at spawn, busy on a prompt and after a
// tool, done at the end of the turn. A busy with no end behind it would pin a
// spinner to the card until the next turn.
func TestKiroAgentReportsTheRightStates(t *testing.T) {
	agent := decodeKiroAgent(t, "0.12.0", "/opt/lich/hooks")
	tests := []struct {
		event string
		want  string
	}{
		{"agentSpawn", "report-session-start.sh' kiro"},
		{"userPromptSubmit", "report-state.sh' busy"},
		{"preToolUse", "report-tool.sh'"},
		{"stop", "report-state.sh' done"},
	}
	for _, tt := range tests {
		commands := hookCommands(t, agent, tt.event)
		if len(commands) == 0 {
			t.Errorf("%s registers nothing", tt.event)
			continue
		}
		// Contains rather than HasSuffix: the script path is shell-quoted, so
		// the argument follows a closing quote rather than the bare path.
		if !strings.Contains(commands[0], tt.want) {
			t.Errorf("%s runs %q, want it to run %q", tt.event, commands[0], tt.want)
		}
	}
}

// TestKiroAgentFiresTouchedOnlyForWrites proves the git-status refresh is
// registered once per write tool and never without a matcher. Firing it for
// every tool would cost a git call per read, which is more than the poll it
// exists to beat; the state report beside it is the one that does want them all.
func TestKiroAgentFiresTouchedOnlyForWrites(t *testing.T) {
	agent := decodeKiroAgent(t, "0.12.0", "/opt/lich/hooks")
	hooks := agent["hooks"].(map[string]any)
	entries, ok := hooks["postToolUse"].([]any)
	if !ok {
		t.Fatalf("postToolUse registers nothing")
	}
	matched := map[string]bool{}
	for _, entry := range entries {
		e := entry.(map[string]any)
		command, _ := e["command"].(string)
		matcher, _ := e["matcher"].(string)
		if !strings.Contains(command, "report-touched.sh") {
			// The state report rides the same event and wants every tool, so it
			// carries no matcher at all.
			if matcher != "" {
				t.Errorf("the state report on postToolUse is limited to %q, want every tool", matcher)
			}
			continue
		}
		if matcher == "" {
			t.Errorf("the touched report fires for every tool, want only writes")
			continue
		}
		matched[matcher] = true
	}
	for _, tool := range []string{"write", "shell"} {
		if !matched[tool] {
			t.Errorf("no touched report matches %q", tool)
		}
	}
	if len(matched) != len(kiroWriteTools) {
		t.Errorf("touched matchers = %v, want exactly %v", matched, kiroWriteTools)
	}
}

// TestKiroAgentMatchersAreBareToolNames pins the matcher spelling. `*` matched
// every tool on a real run and is not a valid regular expression, so Kiro's
// matchers are glob-like rather than regex — and a bare literal is the one
// spelling that means the same tool under glob, regex, exact and substring
// alike. A matcher that grew an anchor or an alternation here would match
// nothing under the semantics actually in play, silently.
func TestKiroAgentMatchersAreBareToolNames(t *testing.T) {
	for _, tool := range kiroWriteTools {
		if strings.ContainsAny(tool, `^$|()[]*?\.`) {
			t.Errorf("matcher %q carries pattern syntax, want a bare tool name", tool)
		}
	}
}

// TestKiroAgentCommandsAreAbsolute proves a hook names the script by full path.
// Kiro runs a hook through a shell from the session's own working directory, so
// a relative command would be resolved against the user's checkout — where the
// script is not.
func TestKiroAgentCommandsAreAbsolute(t *testing.T) {
	agent := decodeKiroAgent(t, "0.12.0", "/opt/lich/hooks")
	hooks := agent["hooks"].(map[string]any)
	for event := range hooks {
		for _, command := range hookCommands(t, agent, event) {
			if !strings.Contains(command, "/opt/lich/hooks/") {
				t.Errorf("%s runs %q, want it under the installed script directory", event, command)
			}
		}
	}
}

// TestKiroAgentCarriesItsVersion proves an install can be recognised and updated:
// the version lich wrote is what installedVersion reads back, and it survives
// the JSON round-trip it is embedded in.
func TestKiroAgentCarriesItsVersion(t *testing.T) {
	data, err := kiroAgentFile("0.12.0", "/opt/lich/hooks")
	if err != nil {
		t.Fatalf("kiroAgentFile = %v, want nil", err)
	}
	got, ok := versionOf(data, `"description": "Installed by`)
	if !ok {
		t.Fatalf("versionOf found no version in the agent lich wrote")
	}
	if got != "0.12.0" {
		t.Errorf("version = %q, want %q", got, "0.12.0")
	}
}

// TestKiroAgentIsNamedWhatTheSpawnAsksFor is the other half of that link: the
// file lich writes has to declare the same name internal/terminal passes to
// `--agent`, or Kiro falls back to kiro_default and every report goes missing
// while the session still opens.
func TestKiroAgentIsNamedWhatTheSpawnAsksFor(t *testing.T) {
	agent := decodeKiroAgent(t, "0.12.0", "/opt/lich/hooks")
	if got, _ := agent["name"].(string); got != "lich" {
		t.Errorf("agent name = %q, want %q", got, "lich")
	}
}

// TestKiroAgentWritesNoTimeout pins the absence measured on 2.21.0: a hook that
// slept four seconds ran to completion under both `timeout_ms` and `timeout`, so
// neither bounds anything and writing one would promise a cut-off Kiro does not
// make. If a release starts honouring one, this is the test that has to change
// before the field comes back.
func TestKiroAgentWritesNoTimeout(t *testing.T) {
	agent := decodeKiroAgent(t, "0.12.0", "/opt/lich/hooks")
	hooks := agent["hooks"].(map[string]any)
	for event, entries := range hooks {
		for _, entry := range entries.([]any) {
			for field := range entry.(map[string]any) {
				if strings.Contains(strings.ToLower(field), "timeout") {
					t.Errorf("%s carries %q, want no timeout Kiro does not enforce", event, field)
				}
			}
		}
	}
}

// TestKiroScriptsAreFetchedOnce proves the install asks the release for each
// script one time even though report-state is named by three events, and that
// the touched report — which no kiroHooks row names — is fetched at all.
func TestKiroScriptsAreFetchedOnce(t *testing.T) {
	got := kiroScripts()
	want := []string{
		"hooks/report-session-start.sh",
		"hooks/report-state.sh",
		"hooks/report-tool.sh",
		"hooks/report-touched.sh",
	}
	slices.Sort(got)
	slices.Sort(want)
	if !slices.Equal(got, want) {
		t.Errorf("kiroScripts() = %v, want %v", got, want)
	}
}

// kiroFiles is the release the fake server hands back: one body per script the
// Kiro install fetches.
func kiroFiles() map[string]string {
	files := map[string]string{}
	for _, script := range kiroScripts() {
		files[tagged(script)] = "#!/bin/sh\n# " + script + "\nexit 0\n"
	}
	return files
}

// TestKiroInstallWritesTheAgentAndTheScripts runs a whole install against
// temporary directories and asserts what a Kiro session would actually find: the
// scripts in lich's own config directory, and an agent in Kiro's agents
// directory naming them. This is the only test that proves the two halves point
// at each other — the file lich writes is useless if its commands name scripts
// that landed somewhere else.
func TestKiroInstallWritesTheAgentAndTheScripts(t *testing.T) {
	s, _ := fileServer(t, kiroFiles())
	log := filepath.Join(t.TempDir(), "cli.log")
	self, err := os.Executable()
	if err != nil {
		t.Fatalf("resolve test binary: %v", err)
	}
	t.Setenv(fakeCLIGuard, "1")
	t.Setenv(fakeCLILog, log)
	s.bins = stubBin(self)
	// Redirects both the config directory the scripts go to and the home the
	// agent hangs off, so nothing here touches the real machine. The two are the
	// same directory on Linux and different ones elsewhere, so each is resolved
	// the way the code under test resolves it.
	configDir := lichConfigHome(t)
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatalf("resolve home directory: %v", err)
	}

	if err := s.Install(providers.Kiro); err != nil {
		t.Fatalf("Install: %v", err)
	}

	agentPath := filepath.Join(home, ".kiro", "agents", "lich.json")
	data, err := os.ReadFile(agentPath)
	if err != nil {
		t.Fatalf("read the agent lich installed: %v", err)
	}
	var agent map[string]any
	if err := json.Unmarshal(data, &agent); err != nil {
		t.Fatalf("installed agent is not JSON: %v", err)
	}
	scriptDir := filepath.Join(configDir, "lich", "plugin", "hooks")
	for _, script := range kiroScripts() {
		installed := filepath.Join(scriptDir, filepath.Base(script))
		info, err := os.Stat(installed)
		if err != nil {
			t.Fatalf("script %s was not installed: %v", script, err)
		}
		// Executable, because Kiro runs the command through a shell and a
		// shebang'd script without the bit is a report that never runs.
		if info.Mode().Perm()&0o111 == 0 {
			t.Errorf("%s is not executable (mode %v)", installed, info.Mode())
		}
	}
	hooks, ok := agent["hooks"].(map[string]any)
	if !ok {
		t.Fatalf("installed agent registers no hooks")
	}
	for event := range hooks {
		for _, command := range hookCommands(t, agent, event) {
			if !strings.Contains(command, scriptDir) {
				t.Errorf("%s runs %q, which is not where the scripts were installed (%s)",
					event, command, scriptDir)
			}
		}
	}
	// The version has to survive the install, or every later update reads the
	// agent as somebody else's file and refuses to touch it.
	if got, ok := s.kiroInstalledVersion(); !ok || got != testVersion {
		t.Errorf("kiroInstalledVersion = %q, %v, want %q, true", got, ok, testVersion)
	}
}

// TestKiroInstallRegistersTheServerInItsOwnAgent proves the MCP registration
// goes through Kiro's own CLI and lands in the agent lich owns — `--agent lich`
// — rather than in the global mcp.json that belongs to the user.
func TestKiroInstallRegistersTheServerInItsOwnAgent(t *testing.T) {
	s, _ := fileServer(t, kiroFiles())
	log := filepath.Join(t.TempDir(), "cli.log")
	self, err := os.Executable()
	if err != nil {
		t.Fatalf("resolve test binary: %v", err)
	}
	t.Setenv(fakeCLIGuard, "1")
	t.Setenv(fakeCLILog, log)
	s.bins = stubBin(self)
	lichConfigHome(t)

	if err := s.Install(providers.Kiro); err != nil {
		t.Fatalf("Install: %v", err)
	}

	calls, err := os.ReadFile(log)
	if err != nil {
		t.Fatalf("the install shelled out to nothing: %v", err)
	}
	got := string(calls)
	for _, want := range []string{"mcp add", "--name lich", "--args mcp", "--agent lich"} {
		if !strings.Contains(got, want) {
			t.Errorf("the install ran %q, want it to carry %q", strings.TrimSpace(got), want)
		}
	}
}

// TestKiroInstallStopsOnAMissingScript proves a release that does not carry
// every script leaves nothing behind to run: the agent is written last, so a
// failed fetch cannot leave hooks naming files that were never installed.
func TestKiroInstallStopsOnAMissingScript(t *testing.T) {
	files := kiroFiles()
	delete(files, tagged("hooks/report-touched.sh"))
	s, _ := fileServer(t, files)
	lichConfigHome(t)
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatalf("resolve home directory: %v", err)
	}

	if err := s.Install(providers.Kiro); err == nil {
		t.Fatalf("Install = nil, want an error for a release missing a script")
	}
	if _, err := os.Stat(filepath.Join(home, ".kiro", "agents", "lich.json")); err == nil {
		t.Errorf("an agent was written even though the install failed")
	}
}
