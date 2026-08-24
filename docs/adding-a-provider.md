# Adding a provider

A **provider** is an agent CLI lich can run inside a session's PTY. There are
six (`internal/providers.Registry`), and adding a seventh means landing in about
a dozen files across two repositories. This is the map — written after
Antigravity, the last one added, so it names what that change actually touched
rather than what it should have.

Two rules govern the whole exercise, and both are in
[`../CLAUDE.md`](../CLAUDE.md):

- **Every table here is read off that CLI's own `--help` or a real run.** A flag
  guessed from a sibling is a spawn that dies before the session exists, and an
  event name copied from another harness is a report that silently never fires.
  Nothing below is inferred from a provider's documentation alone.
- **A gap is allowed; a silent one is not.** Equal behaviour across providers is
  often impossible. Where the new provider cannot do something the others can,
  the same PR adds a [`ceilings.md`](ceilings.md) bullet naming what is out and
  why.

## Measure first

Before editing anything, get these answers from the real binary. Every one of
them is a row in a table below, and a wrong answer is invisible until a user
hits it.

| Question | How it was answered for Antigravity |
|---|---|
| What is the binary called? | `agy` — the name on `PATH`, not the product's |
| How does it resume a conversation by id? | `--conversation <id>` (`--help`) |
| How does it skip permission prompts? | `--dangerously-skip-permissions` (`--help`) |
| How is it told which model to run? | `--model <name>` (`--help`) |
| Can it append to its system prompt at spawn? | No — its whole flag list has nothing for it |
| Can it be handed an MCP server on its command line? | No — MCP lives behind `agy mcp add` |
| Where are its credentials, config and conversations? | `~/.gemini` (+ `~/.gemini/antigravity-cli/`), found by running it against a throwaway `HOME` |
| What proves a conversation still exists? | `~/.gemini/antigravity-cli/conversations/<id>.db` |
| What are its lifecycle events, and what is on their payloads? | A real run with a hook that dumps stdin: `PreInvocation`, `PreToolUse`, `PostToolUse`, `PostInvocation`, `Stop`, all camelCase |
| How is a plugin installed, and does its CLI install a *release*? | `agy plugin install` takes a directory and clones only a default branch — so lich writes the released files itself |

The rig for the last two is the cheap part and the one worth keeping: point the
binary at a throwaway `HOME`, register a hook that writes its stdin to a file,
and run one short non-interactive turn. That is how the payload field names, the
working directory a hook command runs in, the absence of any `CLAUDE_PLUGIN_ROOT`
and the fact that a hook printing `{}` **denies** the tool call were all
established for Antigravity — none of which is in its docs, and the last two
would each have shipped as a plugin that silently does nothing or breaks every
tool.

## The backend

| File | What it holds | What a new provider adds |
|---|---|---|
| `internal/providers/providers.go` | the registry | an id constant, a `Registry` entry (id, display name, binary names), and a line in `AcceptsMCPServer` if it takes an MCP server on its command line |
| `internal/terminal/command.go` | what a spawn runs | entries in `skipPermissionFlags`, `modelFlags`, `briefingFlags` and `resumeArgs` — each one optional, and absent means "no flag rather than somebody else's" |
| `internal/terminal/resume.go` | whether a resume can be offered | a `ResumeAvailable` case answering from what that provider left on disk |
| `internal/terminal/transcript.go`, `sessiondb.go` | where that state lives | the path resolver the case above calls |
| `internal/sandbox/sandbox.go` | what a confined session can still reach | a `stateDirs` case — a provider missing here confines to a home with no credentials, which is a session that opens and cannot log in |
| `internal/agentplugin/` | the companion plugin | a `<provider>.go` with install / installed-version, plus the four switches and the `supported` list in `agentplugin.go` |
| `internal/cli/mcp.go` | the `open_session` tool schema | the new id in the `kind` description |

Nothing else in the backend enumerates providers: the settings store keys
everything on the id (`provider.<id>.bin`, `.enabled`, `.sandbox`,
`.skip-permissions`), and `providers.Known` guards ids arriving from outside.

## The frontend

| File | What a new provider adds |
|---|---|
| `frontend/src/lib/session/sessions.ts` | the id in `PROVIDER_KINDS`, and in `RESUMABLE_KINDS` if its CLI reopens a conversation |
| `frontend/src/lib/providers-store.ts` | its `skipPermissionFlags` spelling — the switch in Settings is hidden for a provider absent here, which is what stops the UI promising a flag the spawn has none for |
| `frontend/src/components/ProviderIcon.tsx` | a brand path, or a lucide fallback |
| `frontend/src/lib/session/delegate-prompt.ts` | `TOOL_KINDS` only if it is handed lich's tools at spawn |
| `frontend/src/lib/session/tool-label.ts` | a rule only if it spells MCP tool names in a shape not already handled |

The two `skipPermissionFlags` tables — Go and TypeScript — are the one place a
provider is spelled twice on purpose. Both are pinned in tests as literals
rather than read from each other: this is the flag that hands an agent the
machine, and a test that follows the map cannot see the map hand a provider
somebody else's flag.

## The plugin

The companion repository (`omartelo/lich-plugin`) owns the client side of every
[hook contract](hooks/README.md), and lich cannot see it break. Move the
contract first — the tables in `docs/hooks/` — then both sides.

What a harness needs from the plugin is one of two shapes, decided by the
harness and not by lich:

- **It runs commands.** Then it gets a hook-registration file mapping its own
  event names onto the scripts already in `hooks/`, and the install is either
  its plugin CLI (Claude Code, Codex) or lich writing the files (Antigravity,
  Crush).
- **It loads a module.** Then it gets a single-file client of its own
  (`opencode/lich.js`, `omp/lich.js`) posting the same payloads to the same
  endpoints.

Register only the reports that harness can actually close. A `busy` with no
end-of-turn event behind it pins a spinner to the card until the next turn — a
state that is wrong for longer than it is right, which is why Crush registers
two of the four reports and not four.

**The fixtures are the handshake, and lich moves first.**
[`docs/hooks/fixtures/`](hooks/fixtures/) is every contract as bytes, and the
plugin's suite asserts against the same lines. A provider id lich has not
registered is rejected by `/session-start` — so until the line below lands here,
the plugin cannot carry a fixture for it and its own tests have to name the gap:

```json
{"name":"antigravity conversation id","body":{"session_id":"s1","provider_session_id":"…","provider":"antigravity"}, "accept":{…}}
```

The id in that line is the one the plugin passes its report script
(`report-session-start.sh antigravity`). They have to be the same string, and
nothing but a session wearing the wrong icon says so when they are not.

## The paperwork

Not optional, and all in the same PR:

- **`docs/hooks/*.md`** — a column per harness in each contract's mapping table,
  plus a paragraph for anything that harness does differently.
- **`docs/hooks/fixtures/session-start.jsonl`** — the accepted-provider line
  above, which is what unblocks the plugin's own suite.
- **`docs/ceilings.md`** — a bullet per gap, naming the mechanism and the file.
- **`docs/cli.md`** — the `--kind` list, the MCP registration table, and the
  `doctor` sample output.
- **`CHANGELOG.md`** — under `[Unreleased]`, written for someone who runs the
  released build.
- **`README.md`, `README.zh-CN.md`, `CLAUDE.md`** — the provider list, which is
  also the checklist rule 5 points at.

## The gate

Beyond the [local gate](../CLAUDE.md), one thing is specific to this change: the
provider list is pinned in tests on both sides
(`TestStatusListsEveryHarness`, `TestDetect`, `isSessionKind`), and those
failures are the contract changing rather than a break. Update them naming which
one it is, and never the other way round.
