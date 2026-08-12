package terminal

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"

	"github.com/omartelo/lich/internal/providers"
	"github.com/omartelo/lich/internal/relay"
)

// What a session's PTY runs: which binary, and with which arguments. Every
// decision here is pure and takes its inputs as parameters, so the spawn path
// (spawnSession) stays a sequence of calls rather than a nest of conditionals.

// defaultBin is the binary spawned when the session's kind is not a known
// provider and no custom path is set — a safety net; real sessions always carry
// a registered provider kind.
const defaultBin = providers.Claude

// KindShell marks a session that runs the user's shell instead of a provider.
const KindShell = "shell"

// How each provider reopens an existing conversation by id: Claude Code takes a
// flag, Codex a subcommand.
const (
	claudeResumeFlag  = "--resume"
	codexResumeSubcmd = "resume"
)

// claudeNameFlag sets the name a session answers to in Claude Code's peer
// roster — the list its cross-session messaging addresses, and what
// `/list-agents` prints.
const claudeNameFlag = "--name"

// skipPermissionFlags is how each provider spells "run every tool without
// asking": every edit and every command goes through unconfirmed. Off unless the
// user turned it on for this checkout's scope in Settings › Providers, which is
// what store.SkipPermissions answers.
//
// Each spelling was read off that provider's own `--help` — they agree on
// nothing, and a flag guessed from a sibling is a spawn that dies before the
// session exists. oh-my-pi is absent because its spelling was never confirmed
// against the binary; a provider missing here gets no flag rather than
// somebody else's.
var skipPermissionFlags = map[string]string{
	providers.Claude:   "--dangerously-skip-permissions",
	providers.Codex:    "--dangerously-bypass-approvals-and-sandbox",
	providers.OpenCode: "--auto",
	providers.Crush:    "--yolo",
}

// How each provider is handed an MCP server on its command line: Claude Code
// takes a JSON string, Codex takes config overrides for its `mcp_servers`
// table. Which providers accept one at all is providers.AcceptsMCPServer.
const (
	claudeMCPFlag   = "--mcp-config"
	codexConfigFlag = "-c"
)

// resolveCommand picks the binary a session runs: the user's shell for "shell"
// sessions, otherwise the provider binary for the session's kind.
func resolveCommand(kind, bin, shellEnv string) string {
	if kind == KindShell {
		if shellEnv == "" {
			return defaultShell
		}
		return shellEnv
	}
	return resolveBin(kind, bin)
}

// resolveBin returns the configured binary, or the provider's default when it is
// empty (falling back to defaultBin for an unknown kind).
func resolveBin(kind, bin string) string {
	if bin != "" {
		return bin
	}
	if def := providers.DefaultBinary(kind); def != "" {
		return def
	}
	return defaultBin
}

// nameArgs returns the arguments that name a session in the provider's peer
// roster, or nil when the provider keeps no roster or the name is unusable.
// Claude Code is the only one with a roster, and left to itself it names a
// session after its working directory — the same directory for every session
// lich runs in one checkout, which is exactly the set the user has to tell
// apart. A name that would be read as a flag is dropped rather than passed on.
func nameArgs(kind, name string) []string {
	if kind != providers.Claude {
		return nil
	}
	name = strings.TrimSpace(name)
	if name == "" || strings.HasPrefix(name, "-") {
		return nil
	}
	return []string{claudeNameFlag, name}
}

// providerArgs assembles the whole argument list for one spawn. Ordering is
// decided here rather than by appending in call order, because the two
// providers constrain it in opposite directions: Codex spells resume as a
// subcommand, so its global options have to come first, while Claude Code's
// --mcp-config is variadic and reads everything after it as another config
// path, so it has to come last.
func providerArgs(kind, name, resume, lichBin string, skipPermissions bool) []string {
	mcp := mcpArgs(kind, lichBin)
	args := append([]string{}, nameArgs(kind, name)...)
	args = append(args, resumeArgs(kind, resume)...)
	args = append(args, skipPermissionArgs(kind, skipPermissions)...)
	if kind == providers.Codex {
		return append(mcp, args...)
	}
	return append(args, mcp...)
}

// skipPermissionArgs returns the flag that drops a provider's permission
// prompts, or nil when it is off or the provider has no spelling wired — a
// stray true must not reach a shell either. The setting is stored per provider,
// so the true that arrives here was ticked for this kind and no other.
func skipPermissionArgs(kind string, skip bool) []string {
	flag, ok := skipPermissionFlags[kind]
	if !skip || !ok {
		return nil
	}
	return []string{flag}
}

// mcpArgs registers lich's own MCP server with the provider being spawned, so
// the agent in that session finds tools for reaching the sessions beside it in
// its own tool list — without the user having to know the feature exists, and
// without lich editing any config the user owns.
//
// The registration is stdio and carries no secret: it names the lich binary and
// its `mcp` subcommand, and the server inherits the loopback coordinates from
// this PTY's environment. A URL registration would have put the token in argv,
// which any user on the machine can read out of /proc/<pid>/cmdline.
//
// Empty when lich cannot name its own binary, or when the provider has no way
// to be told at spawn — never --strict-mcp-config, which would drop the user's
// own MCP servers for the sake of lich's.
func mcpArgs(kind, lichBin string) []string {
	if lichBin == "" || !providers.AcceptsMCPServer(kind) {
		return nil
	}
	switch kind {
	case providers.Claude:
		config := claudeMCPConfig(lichBin)
		if config == "" {
			return nil
		}
		return []string{claudeMCPFlag, config}
	case providers.Codex:
		return []string{
			codexConfigFlag, fmt.Sprintf("mcp_servers.%s.command=%q", relay.MCPServerName, lichBin),
			codexConfigFlag, fmt.Sprintf("mcp_servers.%s.args=[%q]", relay.MCPServerName, relay.MCPSubcommand),
		}
	}
	return nil
}

// claudeMCPConfig is the JSON string Claude Code's --mcp-config accepts in
// place of a file, which is what keeps this registration off the disk entirely.
func claudeMCPConfig(lichBin string) string {
	config := map[string]any{
		"mcpServers": map[string]any{
			relay.MCPServerName: map[string]any{
				"command": lichBin,
				"args":    []string{relay.MCPSubcommand},
			},
		},
	}
	raw, err := json.Marshal(config)
	if err != nil {
		slog.Warn("mcp registration skipped", "err", err)
		return ""
	}
	return string(raw)
}

// resumeArgs returns the arguments that reopen a provider conversation, or nil
// when the session must start fresh. Each provider spells resume its own way,
// and a provider with no spelling here never grows one — the frontend only
// passes a resume id for a kind it knows resumes, but a stray one must not reach
// a shell or opencode/crush either.
func resumeArgs(kind, resume string) []string {
	if resume == "" {
		return nil
	}
	switch kind {
	case providers.Claude:
		return []string{claudeResumeFlag, resume}
	case providers.Codex:
		return []string{codexResumeSubcmd, resume}
	}
	return nil
}
