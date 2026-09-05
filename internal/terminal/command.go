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

// How each provider reopens an existing conversation by id: Claude Code,
// Antigravity, oh-my-pi, Cursor CLI and Kiro CLI take a flag of their own, Codex
// a subcommand, and opencode and Crush happen to agree on one spelling. Every
// one of them was read off that CLI's own --help.
//
// Kiro's is kept apart from the two `--resume` spellings for the same reason
// they are kept apart from each other: its own `--resume` takes no id at all
// (it reopens the most recent conversation in the current directory), so the
// flag that carries one is a different flag with a different name.
//
// Cursor's spells the same word Claude Code's does and is kept apart from it on
// purpose: its value is optional (`--resume [chatId]`, measured on 2026.08.11),
// so the two flags are only the same string, not the same flag, and a shared
// constant would hide the day one of them moves.
const (
	claudeResumeFlag      = "--resume"
	codexResumeSubcmd     = "resume"
	antigravityResumeFlag = "--conversation"
	ompResumeFlag         = "-r"
	sessionResumeFlag     = "--session"
	cursorResumeFlag      = "--resume"
	kiroResumeFlag        = "--resume-id"
)

// How the three providers that can branch a conversation spell it
// (providers.SupportsFork). Two of them add a flag to the resume they already
// take; Codex replaces the subcommand, because its `fork` and `resume` are
// siblings with the same argument shape — `codex fork <SESSION_ID>` (0.151.0).
//
// A fork asks the provider to copy the conversation into a new one of its own,
// so the id lich holds names the *parent* for exactly one spawn: the copy gets
// an id the provider assigns, and lich learns it the way it learns every other
// one, from that session's own session-start report.
const (
	claudeForkFlag   = "--fork-session"
	codexForkSubcmd  = "fork"
	opencodeForkFlag = "--fork"
)

// kiroChatSubcmd is the subcommand a Kiro session runs, and it is not optional.
// Kiro splits the flags lich passes across two parsers: its root takes --agent
// and --resume-id, but --model and --trust-all-tools belong to `chat` alone, and
// either one at the root exits 2 with `unexpected argument` before the session
// exists (measured on 2.21.0). `chat` takes every flag the root does, so lich
// spawns it whether or not those two are set rather than assembling a different
// command per setting.
const kiroChatSubcmd = "chat"

// kiroAgentFlag picks the agent profile a Kiro session runs. lich passes it only
// to reach the hooks: Kiro keeps them inside an agent config and its built-in
// `kiro_default` cannot be shadowed by a file (measured on 2.21.0 — a
// `kiro_default.json` in the agents directory is ignored outright), so the only
// way to register a hook is an agent of lich's own, which internal/agentplugin
// writes and this flag selects.
const kiroAgentFlag = "--agent"

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
// session exists. A provider missing here gets no flag rather than somebody
// else's.
var skipPermissionFlags = map[string]string{
	providers.Claude:      "--dangerously-skip-permissions",
	providers.Codex:       "--dangerously-bypass-approvals-and-sandbox",
	providers.Antigravity: "--dangerously-skip-permissions",
	providers.OpenCode:    "--auto",
	providers.OMP:         "--auto-approve",
	providers.Crush:       "--yolo",
	providers.Cursor:      "--force",
	providers.Kiro:        "--trust-all-tools",
}

// modelFlags is how each provider is told which model to run, for a session
// opened with one named (internal/spawn). Read off each provider's own `--help`,
// like skipPermissionFlags, and every one of them takes the value as a separate
// argument.
//
// Crush is absent and stays absent: its `-m` lives on the non-interactive `run`
// subcommand only, and the TUI lich spawns has no model flag at all (measured
// against 0.88.0). Naming a model there would mean writing the config file the
// user owns, which is the same line lich already draws for MCP registration.
var modelFlags = map[string]string{
	providers.Claude:      "--model",
	providers.Codex:       "--model",
	providers.Antigravity: "--model",
	providers.OpenCode:    "--model",
	providers.OMP:         "--model",
	providers.Cursor:      "--model",
	providers.Kiro:        "--model",
}

// SupportsModel reports whether a provider can be told which model to run when
// lich spawns it. It is what rejects `--model` for a kind that would silently
// drop it, so the caller hears about it instead of getting a session on the
// wrong model.
func SupportsModel(kind string) bool {
	_, ok := modelFlags[kind]
	return ok
}

// briefingFlags is how each provider spells "append this to your system
// prompt", for the two that spell it at all: Claude Code and oh-my-pi share
// --append-system-prompt (read off both `--help`, like every other table here).
//
// Claude Code also has an `--append-system-prompt-file`, which would keep the
// briefing out of argv and out of /proc/<pid>/cmdline — it works (measured on
// 2.1.233), and it is absent from `--help`, named only inside `--bare`'s own
// description. That is the whole reason lich still passes the text flag: an
// undocumented flag that goes away is a spawn that dies before the session
// exists, which is the rule skipPermissionFlags states above. Revisit when it
// appears in `--help`; the briefing has two variants, so it is two files.
//
// They do not mean the same thing by the value. omp's flag takes text *or* a
// file, and picks between them by looking for a newline: a value without one is
// opened as a path, and a read that fails falls back to the literal string. The
// briefing is a single line, so on omp it takes the file branch, fails
// ENAMETOOLONG on a 600-byte "path" and arrives as text — right by fallback
// rather than by design (measured on 17.3.4). Give the briefing a newline and it
// takes the literal branch instead, which lands the same text; what must not
// happen is a briefing short enough to name a real file.
//
// The other five are absent because none of them has an append flag. Codex
// takes `model_instructions_file`, which *replaces* its base instructions —
// swapping the whole system prompt of a provider for a file lich wrote is not a
// thing lich does for one paragraph. opencode's `instructions` and Crush's
// `system_prompt_prefix` are config keys read by every run of that provider on
// the machine, and static config cannot ask whether it is running inside lich: a
// briefing written there would tell a session started outside lich that it is
// inside one. Those three read the same point in lich's MCP instructions
// instead, which costs nothing and is true only where it is read.
//
// What could ask, if the gap is ever worth closing, is the plugin's own hooks —
// they run code and can read LICH_SESSION_ID. Two of the three have somewhere to
// put it: Codex's SessionStart returns `additionalContext` (0.144.5) and
// opencode's plugin API has `experimental.chat.system.transform`, which mutates
// the system messages (1.18.18). Crush has neither: `PreToolUse` is its only
// event (0.88.0), so its briefing would arrive at the first tool call, and a
// turn that answers without one would never see it at all.
var briefingFlags = map[string]string{
	providers.Claude: "--append-system-prompt",
	providers.OMP:    "--append-system-prompt",
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
// roster, or nil when the provider keeps no roster, the session is resuming, or
// the name is unusable. Claude Code is the only one with a roster, and left to
// itself it names a session after its working directory — the same directory
// for every session lich runs in one checkout, which is exactly the set the
// user has to tell apart. A name that would be read as a flag is dropped rather
// than passed on.
//
// lich names a session at birth only. Claude Code writes the name into the
// transcript and restores it on --resume, including one the user changed with
// /rename; passing --name again overrides that silently, so a resume would undo
// a decision the user made inside their own session. The cost is a transcript
// carrying no name at all — a spawn whose name flagValue rejected — which
// resumes into Claude Code's own fallback, the working directory, and that is
// the same string for every session in one checkout.
//
// A fork is a birth, which is why it names one despite carrying a resume id.
// The conversation the provider is about to write is new, so there is no
// /rename inside it to undo — and the copy inherits the parent's name (measured
// on 2.1.261: the forked transcript carries the parent's `agent-name` line), so
// staying silent here is what would leave two cards answering to one string in
// the roster the relay resolves against.
func nameArgs(kind, name, resume string, fork bool) []string {
	if kind != providers.Claude || (resume != "" && !fork) {
		return nil
	}
	name, ok := flagValue(name)
	if !ok {
		return nil
	}
	return []string{claudeNameFlag, name}
}

// flagValue trims a value lich is about to hand a provider flag and reports
// whether it can be passed at all. Nothing to pass is one answer; a value that
// starts with a dash is the other — the provider would read it as another flag,
// and the one thing lich must never do is turn a name or a model somebody typed
// into an argument that changes how the agent runs.
func flagValue(value string) (string, bool) {
	value = strings.TrimSpace(value)
	if value == "" || strings.HasPrefix(value, "-") {
		return "", false
	}
	return value, true
}

// providerArgs assembles the whole argument list for one spawn. Ordering is
// decided here rather than by appending in call order, because the providers
// constrain it in opposite directions: Codex spells resume as a subcommand, so
// its global options have to come first, while Claude Code's --mcp-config is
// variadic and reads everything after it as another config path, so it has to
// come last. Kiro's subcommand opens the session rather than a conversation, so
// it comes before every flag.
func providerArgs(
	kind, name, resume, model, lichBin, agent string, fork, skipPermissions bool,
) []string {
	mcp := mcpArgs(kind, lichBin)
	args := append([]string{}, subcommandArgs(kind)...)
	args = append(args, nameArgs(kind, name, resume, fork)...)
	args = append(args, agentArgs(kind, agent)...)
	args = append(args, resumeArgs(kind, resume, fork)...)
	args = append(args, skipPermissionArgs(kind, skipPermissions)...)
	args = append(args, modelArgs(kind, model)...)
	args = append(args, briefingArgs(kind)...)
	if kind == providers.Codex {
		return append(mcp, args...)
	}
	return append(args, mcp...)
}

// subcommandArgs returns the subcommand that opens a session, or nil when the
// provider's root command is the session. Kiro CLI is the only one that needs
// it, and it goes first: the flags after it are split across Kiro's two parsers
// and only `chat` accepts them all (kiroChatSubcmd).
//
// Codex spells a subcommand too, but its is `resume` — a way of opening one
// conversation rather than the session itself — so it rides resumeArgs.
func subcommandArgs(kind string) []string {
	if kind != providers.Kiro {
		return nil
	}
	return []string{kiroChatSubcmd}
}

// agentArgs returns the arguments that pick the agent profile a provider runs,
// or nil when there is none to name. Kiro CLI is the only provider with one,
// and it is not a preference: the agent is where its hooks live, so a session
// spawned without it reports nothing (docs/hooks/).
//
// Empty when the plugin is not installed, which is what keeps a warning off the
// screen of a user who never asked for one: a `--agent` Kiro cannot find is not
// fatal — it falls back to `kiro_default` and says so on the first line of the
// session (measured on 2.21.0) — but that line would be there on every spawn.
func agentArgs(kind, agent string) []string {
	if kind != providers.Kiro {
		return nil
	}
	agent, ok := flagValue(agent)
	if !ok {
		return nil
	}
	return []string{kiroAgentFlag, agent}
}

// modelArgs returns the arguments that pick the model a provider runs, or nil
// when none was named or the provider has no flag for it. The name is passed
// through unchecked — every provider spells its own model names, they change
// with each release, and a list kept here would reject a model that works. A
// wrong one dies in the provider's own error message, which is the one the user
// can act on. A value that would be read as a flag is dropped instead, as it is
// for a session name.
func modelArgs(kind, model string) []string {
	flag, wired := modelFlags[kind]
	model, usable := flagValue(model)
	if !wired || !usable {
		return nil
	}
	return []string{flag, model}
}

// briefingArgs returns the arguments that append lich's own briefing to a
// provider's system prompt, or nil for a provider with no spelling for it. The
// briefing is worded for what this spawn actually registered: a provider handed
// lich's MCP server is pointed at the tools, everyone else at the command line
// (relay.SpawnBriefing).
func briefingArgs(kind string) []string {
	flag, ok := briefingFlags[kind]
	if !ok {
		return nil
	}
	return []string{flag, relay.SpawnBriefing(providers.AcceptsMCPServer(kind))}
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
// a shell either.
//
// fork asks for the conversation to be branched rather than continued, and it
// is honoured only where the provider has a verb for it: a fork flag invented
// for one that has none would kill the spawn before the session exists, so the
// three that do are spelled below and every other kind resumes as it always
// did. providers.SupportsFork is what stops the offer reaching here at all.
func resumeArgs(kind, resume string, fork bool) []string {
	if resume == "" {
		return nil
	}
	switch kind {
	case providers.Claude:
		if fork {
			return []string{claudeResumeFlag, resume, claudeForkFlag}
		}
		return []string{claudeResumeFlag, resume}
	case providers.Codex:
		if fork {
			return []string{codexForkSubcmd, resume}
		}
		return []string{codexResumeSubcmd, resume}
	case providers.OpenCode:
		if fork {
			return []string{sessionResumeFlag, resume, opencodeForkFlag}
		}
		return []string{sessionResumeFlag, resume}
	case providers.Antigravity:
		return []string{antigravityResumeFlag, resume}
	case providers.OMP:
		return []string{ompResumeFlag, resume}
	case providers.Crush:
		return []string{sessionResumeFlag, resume}
	case providers.Cursor:
		return []string{cursorResumeFlag, resume}
	case providers.Kiro:
		return []string{kiroResumeFlag, resume}
	}
	return nil
}
