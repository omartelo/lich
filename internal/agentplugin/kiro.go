package agentplugin

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/omartelo/lich/internal/providers"
	"github.com/omartelo/lich/internal/relay"
	"github.com/omartelo/lich/internal/shquote"
)

// The Kiro CLI side. Kiro has no plugin system: its hooks are a block inside an
// *agent config*, and its built-in `kiro_default` cannot be shadowed — a
// `kiro_default.json` in the agents directory is ignored outright (measured on
// 2.21.0). So the only place a hook can be registered is an agent of lich's own,
// which this writes and internal/terminal spawns with `--agent`
// (providers.KiroAgentName spells the name for both).
//
// That makes the whole file lich's, the way Antigravity's plugin directory is,
// so the version lives in its `description` rather than in a marker line hidden
// in a comment: JSON has nowhere to hide one, and rewriting a document lich did
// not write is the line crush.go draws for the same reason.
//
// Three of the four reports are registered. The title report is not: it reads an
// `ai-title` out of a transcript path the harness hands it on stdin, and Kiro
// passes no transcript path at all — see docs/ceilings.md, which also carries
// what a lich-spawned Kiro session gives up by running this agent instead of
// `kiro_default`.

// kiroWriteTools are the tools whose use means a file may have changed, so the
// touched report fires for them and not for a read. They are registered as one
// hook entry each, spelled as bare tool names on purpose: a matcher of `*`
// matched every tool on a real run, and `*` is not a valid regular expression,
// so Kiro's matchers are glob-like rather than regex. A bare literal is the one
// spelling that means the same thing under every candidate semantics — glob,
// regex, exact or substring — which is what keeps this from being the silent
// no-op docs/adding-a-provider.md warns a copied matcher becomes.
//
// The names are the ones a real payload carried (`tool_name` on postToolUse),
// not the ones Kiro's own `--trust-tools` help spells: the classic engine calls
// the same two `fs_write` and `execute_bash`, and only the TUI names — which is
// what lich spawns — are these.
var kiroWriteTools = []string{"write", "shell"}

// kiroHooks is what lich registers, one entry per report Kiro can honour: the
// event, the script from the plugin repository, the argument it takes, and the
// tool it matches ("" for every tool).
//
// Nothing here prints. Kiro feeds a hook's stdout back into the conversation as
// context — that is what its hooks are for — so a report that echoed anything
// would put lich's own bookkeeping in front of the model. The scripts are silent
// by contract (docs/hooks/README.md), which stops being a style choice here.
var kiroHooks = []struct {
	event   string
	script  string
	arg     string
	matcher string
}{
	{event: "agentSpawn", script: "hooks/report-session-start.sh", arg: providers.Kiro},
	{event: "userPromptSubmit", script: "hooks/report-state.sh", arg: "busy"},
	{event: "preToolUse", script: "hooks/report-tool.sh"},
	{event: "postToolUse", script: "hooks/report-state.sh", arg: "busy"},
	{event: "stop", script: "hooks/report-state.sh", arg: "done"},
}

// No timeout is written, and its absence is measured rather than forgotten:
// Kiro blocks the turn behind a hook — its TUI draws "0 of 1 hooks finished"
// while one runs — but a hook sleeping four seconds ran to completion under both
// `timeout_ms: 1000` and `timeout: 1` (2.21.0, on `agentSpawn`). Neither
// spelling bounds anything, and a field lich writes that does nothing is the
// silent no-op this package exists to avoid. What actually bounds a report is
// the script's own request timeout, which is where docs/ceilings.md points.

func (s *Service) kiroInstall() error {
	version, err := s.releaseVersion()
	if err != nil {
		return err
	}
	dir, err := pluginScriptDir(providers.Kiro)
	if err != nil {
		return err
	}
	scripts := map[string][]byte{}
	for _, name := range kiroScripts() {
		data, err := s.fetchFile(version, name)
		if err != nil {
			return err
		}
		scripts[filepath.Base(name)] = data
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create %s: %w", dir, err)
	}
	for name, data := range scripts {
		// Executable: Kiro runs a hook command through a shell, which needs the
		// bit on a shebang'd script the way any other shell would.
		if err := writeFile(filepath.Join(dir, name), data, 0o755); err != nil {
			return err
		}
	}
	path, err := providers.KiroAgentPath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create %s: %w", filepath.Dir(path), err)
	}
	agent, err := kiroAgentFile(version, dir)
	if err != nil {
		return err
	}
	if err := writeFile(path, agent, 0o644); err != nil {
		return err
	}
	return s.kiroRegisterMCP()
}

// kiroScripts is every file the registration runs, deduplicated — report-state
// is named by three events and fetched once.
func kiroScripts() []string {
	seen := map[string]bool{}
	var out []string
	for _, hook := range kiroHooks {
		if seen[hook.script] {
			continue
		}
		seen[hook.script] = true
		out = append(out, hook.script)
	}
	return append(out, "hooks/report-touched.sh")
}

// kiroAgentFile is the agent lich writes: the hooks, and the least configuration
// around them that leaves a session working.
//
// `prompt` is deliberately absent. It is *additional* guidance rather than the
// model's whole system prompt — an agent carrying none answered, ran commands
// and wrote files on a real run — so leaving it out costs the paragraph
// `kiro_default` adds about subagents, the planner and LSP, and costs nothing
// else. Copying that paragraph instead would freeze a snapshot of Kiro's own
// default into a file lich never updates, which is a session drifting further
// from stock Kiro with every release and saying nothing (docs/ceilings.md).
//
// `resources` is the one thing worth carrying over from `kiro_default`, because
// dropping it would silently stop a project's own context from loading.
func kiroAgentFile(version, scriptDir string) ([]byte, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("resolve home directory: %w", err)
	}
	hooks := map[string][]kiroHook{}
	for _, hook := range kiroHooks {
		hooks[hook.event] = append(hooks[hook.event], kiroHook{
			Command: kiroCommand(scriptDir, hook.script, hook.arg),
			Matcher: hook.matcher,
		})
	}
	for _, tool := range kiroWriteTools {
		hooks["postToolUse"] = append(hooks["postToolUse"], kiroHook{
			Command: kiroCommand(scriptDir, "hooks/report-touched.sh", ""),
			Matcher: tool,
		})
	}
	body, err := json.MarshalIndent(kiroAgent{
		Name: providers.KiroAgentName,
		Description: fmt.Sprintf(
			"Installed by %s v%s — replace with the plugin from %s", markerName, version, marketplaceRepo),
		Tools: []string{"*"},
		Resources: []string{
			"file://AmazonQ.md",
			"file://AGENTS.md",
			"file://README.md",
			"skill://.kiro/skills/*/SKILL.md",
			"skill://" + filepath.ToSlash(filepath.Join(home, ".kiro", "skills", "*", "SKILL.md")),
		},
		IncludeMCPJSON: true,
		Hooks:          hooks,
	}, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("encode the %s agent: %w", providers.KiroAgentName, err)
	}
	return append(body, '\n'), nil
}

// kiroAgent is the agent config lich writes. Only the fields lich sets are here:
// Kiro fills the rest with its own defaults, and a field written as a zero value
// would be lich deciding something it was never asked to.
type kiroAgent struct {
	Name           string                `json:"name"`
	Description    string                `json:"description"`
	Tools          []string              `json:"tools"`
	Resources      []string              `json:"resources"`
	IncludeMCPJSON bool                  `json:"includeMcpJson"`
	Hooks          map[string][]kiroHook `json:"hooks"`
}

// kiroHook is one registered command. Matcher is omitted when empty, which is
// what makes the hook fire for every tool.
type kiroHook struct {
	Command string `json:"command"`
	Matcher string `json:"matcher,omitempty"`
}

// kiroCommand is the shell command one hook runs. The path is absolute and
// quoted: Kiro runs a hook through a shell from the session's own working
// directory, so a relative path would resolve against the user's checkout
// instead of against lich's scripts.
func kiroCommand(scriptDir, script, arg string) string {
	command := shquote.Quote(filepath.Join(scriptDir, filepath.Base(script)))
	if arg != "" {
		command += " " + arg
	}
	return command
}

// kiroRegisterMCP gives a Kiro session the operations for driving the sessions
// beside it. `kiro-cli mcp add --agent` is the supported interface and it writes
// into lich's own agent rather than the user's global mcp.json, so nothing
// outside this install is touched; it replaces an existing entry rather than
// refusing one, so a reinstall is not a special case.
//
// A lich that cannot name its own binary registers nothing rather than a command
// that cannot run: the reports are the half Kiro needs to be useful, and they do
// not depend on it.
func (s *Service) kiroRegisterMCP() error {
	lichBin := s.lichBin()
	if lichBin == "" {
		return nil
	}
	return s.run(providers.Kiro, "mcp", "add",
		"--name", relay.MCPServerName,
		"--command", lichBin,
		"--args", relay.MCPSubcommand,
		"--agent", providers.KiroAgentName,
	)
}

// kiroInstalledVersion reads the version off the agent lich wrote, or ("",
// false) when it is absent, unreadable, or was not written by lich.
func (s *Service) kiroInstalledVersion() (string, bool) {
	path, err := providers.KiroAgentPath()
	if err != nil {
		return "", false
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return "", false
	}
	return versionOf(data, `"description": "Installed by`)
}
