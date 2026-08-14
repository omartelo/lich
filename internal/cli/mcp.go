package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/omartelo/lich/internal/relay"
	"github.com/omartelo/lich/internal/spawn"
)

// `lich mcp` is the same surface as the commands beside it, offered as MCP
// tools instead of as argv. It exists because a command line only helps an
// agent that already knows the command exists: a slash command has to be typed,
// a documented flag has to be read. An MCP tool is in the agent's own tool list
// from the first turn, so nobody has to be told anything — which is the whole
// point, since the person who would need telling is the one who does not read
// release notes.
//
// It speaks the stdio transport: one JSON-RPC 2.0 message per line on stdin and
// stdout. stdout therefore carries protocol and nothing else — anything this
// package would print for a human goes to stderr, which the host logs.
//
// Registration is stdio rather than HTTP for one reason: the config lich passes
// at spawn lands in the provider's argv, and /proc/<pid>/cmdline is readable by
// any user on the machine while /proc/<pid>/environ is not. A URL registration
// would put ?token= in argv. A stdio one carries no secret at all — the server
// inherits the coordinates from the PTY's environment, where they already were.

// mcpProtocolVersion is what this server advertises when a client asks for
// nothing. A client that names its own version gets that version back: this
// implements the tools subset, which every revision of the protocol carries
// unchanged, so agreeing with the client costs nothing and refusing it would
// only lose a working session over a date.
const mcpProtocolVersion = "2025-06-18"

// mcpMaxWait caps how long any tool here blocks, whatever the caller asks for.
//
// Claude Code moves an MCP call that runs past 120 seconds into the background
// and delivers its result later as a notification. A tool that waits longer
// therefore does not wait at all — it detaches, and the agent reports that it is
// "waiting in the background" while the answer lands somewhere it has stopped
// looking. Seen on the first real run, at the caller's own request: it asked for
// 120 seconds and then 180.
//
// The ticket is what makes the cap cheap. A wait that ends unanswered costs one
// more tool call, inside the agent's own turn, instead of a detached task.
const mcpMaxWait = 90

// jsonRPC frames one message in either direction. ID is absent on a
// notification, which is answered with nothing at all.
type jsonRPC struct {
	Version string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method,omitempty"`
	Params  json.RawMessage `json:"params,omitempty"`
	Result  any             `json:"result,omitempty"`
	Error   *jsonRPCError   `json:"error,omitempty"`
}

type jsonRPCError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// JSON-RPC error codes this server can emit.
const (
	codeMethodNotFound = -32601
	codeInvalidParams  = -32602
)

// mcpTool is one entry in the tool list. run returns the text the agent sees;
// an error it returns is reported as a failed tool call rather than as a
// protocol error, so the agent reads what went wrong and can act on it.
// ReadOnly marks the tools that change nothing, which a client may auto-allow.
type mcpTool struct {
	Name        string
	Description string
	Schema      map[string]any
	ReadOnly    bool
	Run         func(c *client, args mcpArgs) (string, error)
}

// mcpInstructions is the server's own briefing, injected into the client's
// system prompt at initialize. The tool descriptions each explain one door;
// this is the only place the whole journey — fan out, carry on, collect — is
// told as one, and the difference between an agent that orchestrates and one
// that never realizes it could.
const mcpInstructions = `lich runs coding-agent sessions side by side in one window; these tools are how this session works with the others.

Delegate in parallel: for work that can run beside yours, open a worker session in its own git worktree (open_session with worktree) so it gets its own checkout, then hand it the task with send_to_session. Check list_worktrees first — a branch already checked out is opened, not created. Workers are full sessions on the user's screen: visible, steerable, and yours to close (close_session) when the work is done. That is the difference from the subagents your own harness runs, and it is what the user is asking for when they say to fan work out across branches or worktrees: a subagent has no checkout of its own and no card they can open, read or take over mid-task. Reach for one of those to read something and throw it away, not to run the implementation somebody asked to see.

Never poll for results. A send that outlives its wait hands back a ticket and you carry on with your own work; when results are ready, one short [lich] note arrives at your prompt — collect everything at once with wait_for_answer (no ticket). Polling in a loop burns the tokens this design exists to save.

When a [lich] message carrying a ticket arrives at YOUR prompt, you are the worker: do the task, then answer with ` + relay.ToolReply + ` — a concise report (what was done, where, what remains), never a transcript.`

// serveMCP runs the stdio server until its input ends.
func (c *client) serveMCP(args []string) error {
	flags := newFlagSet("mcp")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if c.stdin == nil {
		return fmt.Errorf("no input to serve")
	}

	decoder := json.NewDecoder(c.stdin)
	encoder := json.NewEncoder(c.stdout)
	for {
		var request jsonRPC
		if err := decoder.Decode(&request); err != nil {
			if errors.Is(err, io.EOF) {
				return nil
			}
			return fmt.Errorf("read message: %w", err)
		}
		response, answer := c.handleMCP(request)
		if !answer {
			continue
		}
		if err := encoder.Encode(response); err != nil {
			return fmt.Errorf("write message: %w", err)
		}
	}
}

// handleMCP answers one message. The second return is false for a notification,
// which the protocol says gets no reply — answering one is a protocol error on
// this side, not a courtesy.
func (c *client) handleMCP(request jsonRPC) (jsonRPC, bool) {
	if len(request.ID) == 0 {
		return jsonRPC{}, false
	}
	response := jsonRPC{Version: "2.0", ID: request.ID}

	switch request.Method {
	case "initialize":
		response.Result = mcpHandshake(request.Params, c.version)
	case "tools/list":
		response.Result = map[string]any{"tools": mcpToolList()}
	case "tools/call":
		response.Result, response.Error = c.callMCPTool(request.Params)
	case "ping":
		response.Result = map[string]any{}
	default:
		response.Error = &jsonRPCError{Code: codeMethodNotFound, Message: "unknown method " + request.Method}
	}
	return response, true
}

// mcpHandshake answers initialize, echoing the client's protocol version when
// it named one (see mcpProtocolVersion).
func mcpHandshake(params json.RawMessage, version string) map[string]any {
	var asked struct {
		ProtocolVersion string `json:"protocolVersion"`
	}
	_ = json.Unmarshal(params, &asked)
	protocol := asked.ProtocolVersion
	if protocol == "" {
		protocol = mcpProtocolVersion
	}
	return map[string]any{
		"protocolVersion": protocol,
		"capabilities":    map[string]any{"tools": map[string]any{}},
		"serverInfo":      map[string]any{"name": relay.MCPServerName, "version": version},
		"instructions":    mcpInstructions,
	}
}

// callMCPTool runs one tool. A tool that fails answers with isError so the
// agent sees the reason; only an unknown tool or unreadable arguments are
// protocol errors.
func (c *client) callMCPTool(params json.RawMessage) (any, *jsonRPCError) {
	var call struct {
		Name      string  `json:"name"`
		Arguments mcpArgs `json:"arguments"`
	}
	if err := json.Unmarshal(params, &call); err != nil {
		return nil, &jsonRPCError{Code: codeInvalidParams, Message: err.Error()}
	}
	for _, tool := range mcpTools {
		if tool.Name != call.Name {
			continue
		}
		text, err := tool.Run(c, call.Arguments)
		if err != nil {
			return mcpText(err.Error(), true), nil
		}
		return mcpText(text, false), nil
	}
	return nil, &jsonRPCError{Code: codeInvalidParams, Message: "unknown tool " + call.Name}
}

// mcpText is a tool result: MCP carries them as content blocks, and everything
// here is text.
func mcpText(text string, failed bool) map[string]any {
	result := map[string]any{"content": []any{map[string]any{"type": "text", "text": text}}}
	if failed {
		result["isError"] = true
	}
	return result
}

func mcpToolList() []map[string]any {
	list := make([]map[string]any, 0, len(mcpTools))
	for _, tool := range mcpTools {
		entry := map[string]any{
			"name":        tool.Name,
			"description": tool.Description,
			"inputSchema": tool.Schema,
		}
		if tool.ReadOnly {
			entry["annotations"] = map[string]any{"readOnlyHint": true}
		}
		list = append(list, entry)
	}
	return list
}

// mcpArgs is one tool call's arguments, read leniently: a model that sends a
// number as a string, or omits an optional field, should not lose its turn to a
// type error.
type mcpArgs map[string]any

func (a mcpArgs) text(key string) string {
	switch v := a[key].(type) {
	case string:
		return v
	case nil:
		return ""
	default:
		return fmt.Sprint(v)
	}
}

// flag reads a boolean, and a string only counts as one when it spells "true".
// A model that sends "false", "yes", or anything else it invented must not read
// as consent, because the one flag this carries discards uncommitted work.
func (a mcpArgs) flag(key string) bool {
	switch v := a[key].(type) {
	case bool:
		return v
	case string:
		return v == "true"
	default:
		return false
	}
}

// seconds reads a wait from the arguments, clamped to what an MCP call may
// block for (see mcpMaxWait). A caller asking for longer gets the cap, not the
// detachment it was heading for.
func (a mcpArgs) seconds(key string) int {
	asked := 0
	switch v := a[key].(type) {
	case float64:
		asked = int(v)
	case string:
		_, _ = fmt.Sscanf(v, "%d", &asked)
	}
	if asked <= 0 || asked > mcpMaxWait {
		return mcpMaxWait
	}
	return asked
}

// schema builds an inputSchema from named properties and the subset of them
// that is required.
func schema(properties map[string]any, required ...string) map[string]any {
	if required == nil {
		required = []string{}
	}
	return map[string]any{
		"type":       "object",
		"properties": properties,
		"required":   required,
	}
}

func property(kind, description string) map[string]any {
	return map[string]any{"type": kind, "description": description}
}

// mcpTools is the whole tool surface. Adding one is an entry here — the reason
// registering MCP once buys every tool lich grows later, on every provider,
// with no further per-provider work.
var mcpTools = []mcpTool{
	{
		Name: "list_sessions",
		Description: "List the other lich sessions that are live right now and can be given work. " +
			"Returns each session's label (how it is addressed), its project, and which agent runs in it.",
		Schema:   schema(map[string]any{}),
		ReadOnly: true,
		Run: func(c *client, _ mcpArgs) (string, error) {
			var peers []relay.Peer
			if err := c.call("relay.Peers", []any{c.sessionID()}, shortCall, &peers); err != nil {
				return "", err
			}
			if len(peers) == 0 {
				return "No other live sessions.", nil
			}
			out, err := json.Marshal(peers)
			if err != nil {
				return "", err
			}
			return string(out), nil
		},
	},
	{
		Name: "send_to_session",
		Description: "Give another lich session a task. The task is put at that session's prompt " +
			"as if typed there. Returns the answer when it comes back quickly; for anything " +
			"slower you get a ticket and carry on — when the result is ready, a short note " +
			"arrives at your own prompt and wait_for_answer collects it, so you never poll. " +
			"Sessions are addressed by the label on their lich card — call list_sessions for " +
			"the labels. If you were given a name like `myrepo-a1b2`, that is a peer-roster " +
			"name from your own cross-session messaging, not a lich label: reach it with your " +
			"own messaging tool instead of this one.",
		Schema: schema(map[string]any{
			"session": property("string",
				"Label of the target session, exactly as list_sessions returns it. Not a peer-roster name."),
			"prompt":  property("string", "What to ask that session's agent to do."),
			"project": property("string", "Project name, needed only when two live sessions share a label."),
			"timeout_seconds": property("number",
				"Seconds to hold the line for a quick answer, at most 90 — longer is capped, "+
					"because a call running past 120s is detached by the client and stops being a "+
					"wait at all. Running out costs nothing: the answer arrives at your prompt."),
		}, "session", "prompt"),
		Run: func(c *client, args mcpArgs) (string, error) {
			var result relay.Result
			timeout := args.seconds("timeout_seconds")
			call := []any{c.sessionID(), args.text("session"), args.text("project"), args.text("prompt"), timeout}
			if err := c.call("relay.Send", call, waitBudget(timeout), &result); err != nil {
				return "", err
			}
			return mcpOutcome(result), nil
		},
	},
	{
		Name: relay.ToolCollect,
		Description: "Collect the results of tasks you sent with send_to_session. Without a " +
			"ticket it returns every result that is ready — the one call to make when a " +
			"[lich] note says results are waiting — and, when none is, holds the line for " +
			"the next one. With a ticket it waits on that one errand. Use it to pick " +
			"results up, or when you have nothing to do but wait; results announce " +
			"themselves at your prompt either way, so there is never a reason to poll.",
		Schema: schema(map[string]any{
			"ticket": property("string",
				"A ticket send_to_session returned. Omit to collect everything that is ready."),
			"timeout_seconds": property("number",
				"Seconds to hold the line when nothing is ready yet, at most 90. Call again for longer."),
		}),
		Run: func(c *client, args mcpArgs) (string, error) {
			timeout := args.seconds("timeout_seconds")
			if ticket := args.text("ticket"); ticket != "" {
				var result relay.Result
				if err := c.call("relay.Wait", []any{ticket, timeout}, waitBudget(timeout), &result); err != nil {
					return "", err
				}
				return mcpOutcome(result), nil
			}
			var collected relay.Collected
			if err := c.call("relay.Collect", []any{c.sessionID(), timeout}, waitBudget(timeout), &collected); err != nil {
				return "", err
			}
			return collectedText(collected), nil
		},
	},
	{
		Name: "open_session",
		Description: "Open a new lich session and start it. Optionally creates a git worktree " +
			"first and roots the new session in it, which is how you give a task its own " +
			"checkout instead of sharing yours — and optionally hands it the task in the same " +
			"call, so fanning work out costs one call per worker instead of two. Returns the " +
			"names the new session is addressed by, and, when a task came with it, that task's " +
			"outcome: the answer if it was quick, otherwise a ticket to carry on from, exactly " +
			"as send_to_session returns one.",
		Schema: schema(map[string]any{
			"project": property("string",
				"Project to open the session in, by name. Defaults to your own project."),
			"kind": property("string",
				"What the session runs: claude, codex, opencode, omp, crush, or shell. "+
					"Defaults to the same agent you are."),
			"worktree": property("string",
				"Branch name for a new git worktree to root the session in. Omit to open the "+
					"session in the project's own directory, beside yours."),
			"base": property("string",
				"Branch the new worktree starts from. Defaults to the project's current branch."),
			"model": property("string",
				"Model the new session's provider runs, spelled exactly as that provider's own "+
					"--model flag takes it — the name or alias, never a lich name for it. Omit "+
					"to leave the provider on its default. Crush and shell sessions cannot be "+
					"given one."),
			"prompt": property("string",
				"Task to hand the new session as soon as its agent is up. Omit to open it idle "+
					"and send later."),
		}),
		Run: func(c *client, args mcpArgs) (string, error) {
			var opened spawn.Session
			call := []any{
				c.sessionID(), args.text("project"), args.text("kind"),
				args.text("worktree"), args.text("base"), args.text("model"),
			}
			if err := c.call("spawn.Open", call, openCall, &opened); err != nil {
				return "", err
			}
			prompt := args.text("prompt")
			if prompt == "" {
				return openedText(opened), nil
			}
			return openedText(opened) + "\n" + c.handOver(opened, prompt), nil
		},
	},
	{
		Name: "close_session",
		Description: "Close a session opened in lich. Closing the last session in a git " +
			"worktree decides what happens to that checkout, so it needs the worktree " +
			"argument: keep it on disk (the session is parked, and opening a session on " +
			"that branch again resumes its conversation) or remove it. A checkout with " +
			"uncommitted work is only removed with force, because what that discards is in " +
			"no commit and on no remote. You cannot close the session you are running in.",
		Schema: schema(map[string]any{
			"session": property("string",
				"The session to close, by the label on its card or the name it answers to."),
			"project": property("string",
				"Project to narrow to, when the same label exists in more than one."),
			"worktree": property("string",
				"Required when this is the last session in a worktree: \"keep\" leaves the "+
					"checkout on disk, \"remove\" deletes it."),
			"force": property("boolean",
				"Remove a checkout that still has uncommitted work. Ask the user first."),
		}, "session"),
		Run: func(c *client, args mcpArgs) (string, error) {
			var closed spawn.Closed
			call := []any{
				c.sessionID(), args.text("session"), args.text("project"),
				args.text("worktree"), args.flag("force"),
			}
			if err := c.call("spawn.Close", call, openCall, &closed); err != nil {
				return "", err
			}
			return closedText(closed), nil
		},
	},
	{
		Name: "list_worktrees",
		Description: "The git worktrees of a project: what each is called, whether it has " +
			"uncommitted work, and which sessions are open in it. Use it before opening a " +
			"session on a branch (one that is already checked out is opened, not created) " +
			"and before closing one (the last session in a checkout decides its fate).",
		Schema: schema(map[string]any{
			"project": property("string", "Project to list. Defaults to your own."),
		}),
		ReadOnly: true,
		Run: func(c *client, args mcpArgs) (string, error) {
			var checkouts []spawn.Checkout
			call := []any{c.sessionID(), args.text("project")}
			if err := c.call("spawn.Worktrees", call, shortCall, &checkouts); err != nil {
				return "", err
			}
			out, err := json.Marshal(asList(checkouts))
			if err != nil {
				return "", fmt.Errorf("encode the worktrees: %w", err)
			}
			return string(out), nil
		},
	},
	{
		Name: relay.ToolReply,
		Description: "Answer a task another session gave you. Call this when a message at your prompt " +
			"came from lich and carried a ticket; whoever asked is blocked until you do. " +
			"It is the only route back — a peer message does not reach them, because they are " +
			"waiting on the ticket and reading nothing else.",
		Schema: schema(map[string]any{
			"ticket": property("string", "The ticket from the message you were given."),
			"answer": property("string", "Your answer, in full — nothing else is sent back."),
		}, "ticket", "answer"),
		Run: func(c *client, args mcpArgs) (string, error) {
			if err := c.call("relay.Reply", []any{args.text("ticket"), args.text("answer")}, shortCall, nil); err != nil {
				return "", err
			}
			return "Answer sent.", nil
		},
	},
}

// deliverWait bounds handing a task to a session that was opened a moment ago:
// how long the delivery waits for that session's agent to be the program reading
// its PTY (relay.awaitReady).
//
// It is not a wait for an answer. The worker was created seconds ago and its
// task is minutes of work, so a ticket is the expected outcome and the sender
// carries on. It is short because opening may already have spent up to openCall
// seconds on a worktree, and the two together have to stay under the 120 seconds
// past which the client stops waiting and detaches the call (see mcpMaxWait).
const deliverWait = 30

// handOver gives a just-opened session its first task, wording the outcome the
// way send_to_session words it — the caller should not have to learn two
// spellings of one answer.
//
// A delivery that fails is reported beside the opened session rather than as a
// failed call. The session exists either way, and a failed call would read as if
// nothing had happened — leaving an agent that opens three workers convinced it
// has none.
func (c *client) handOver(opened spawn.Session, prompt string) string {
	var result relay.Result
	call := []any{c.sessionID(), opened.Label, opened.Project, prompt, deliverWait}
	if err := c.call("relay.Send", call, waitBudget(deliverWait), &result); err != nil {
		return fmt.Sprintf(
			"The task did not reach it: %v. The session is open and idle, so send the task "+
				"with send_to_session — a fresh worktree runs the project's setup script "+
				"before its agent, and that can outlast this call.",
			err,
		)
	}
	return mcpOutcome(result)
}

// mcpOutcome words a Send or Wait result for an agent: the answer alone when
// there is one, otherwise what to call next. A bare ticket would leave the
// agent guessing what it is for.
func mcpOutcome(result relay.Result) string {
	switch result.Status {
	case relay.StatusAnswered:
		return result.Answer
	case relay.StatusUnanswered:
		return unansweredText(result.Target)
	case relay.StatusUnread:
		return unreadText(result.Target)
	}
	return fmt.Sprintf(
		"%s is still working. The task was delivered; when its result is ready a short note "+
			"will arrive at your own prompt, so carry on — there is nothing to poll. Collect "+
			"it with wait_for_answer (ticket %q, or no ticket for everything at once).",
		result.Target, result.Ticket,
	)
}

// collectedText words a drain: each result in the order it arrived, then who
// still owes one. Statuses reuse the same words a single wait answers with —
// the reader should not meet two spellings of one outcome.
func collectedText(collected relay.Collected) string {
	if len(collected.Results) == 0 && len(collected.Open) == 0 {
		return "Nothing to collect: no results are waiting and no errand of yours is open."
	}
	var parts []string
	for _, result := range collected.Results {
		switch result.Status {
		case relay.StatusAnswered:
			parts = append(parts, fmt.Sprintf(
				"Answer from %q (ticket %s):\n%s", result.Target, result.Ticket, result.Answer))
		case relay.StatusUnread:
			parts = append(parts, unreadText(result.Target))
		case relay.StatusUnanswered:
			parts = append(parts, unansweredText(result.Target))
		}
	}
	if len(collected.Open) > 0 {
		quoted := make([]string, 0, len(collected.Open))
		for _, label := range collected.Open {
			quoted = append(quoted, fmt.Sprintf("%q", label))
		}
		parts = append(parts, fmt.Sprintf(
			"Still working: %s. Their results will announce themselves at your prompt.",
			strings.Join(quoted, ", ")))
	}
	return strings.Join(parts, "\n\n")
}

// unreadText is what both surfaces say about a task that reached the terminal
// and nothing read: the session never started working on it. What is usually
// there is a question only a person can answer, so this has to send the reader
// to that card rather than suggest waiting or trying the same thing again.
func unreadText(target string) string {
	return fmt.Sprintf(
		"The %q session never picked the task up: it was typed at that prompt and "+
			"nothing read it, so something else has that terminal — a provider still "+
			"starting, or a question of its own on screen (Claude Code asks whether a "+
			"directory is trusted the first time it runs in one). Nothing is queued and "+
			"nothing was answered. Tell the user to open the %q card and clear what is "+
			"on it; the task has to be sent again after that.",
		target, target,
	)
}

// unansweredText is what both surfaces say when the target worked through the
// request and ended its turn without replying here. It has to send the reader
// to the other session: whatever the agent there produced is on its screen and
// nowhere lich can reach, so an answer that reads as "nothing happened" would
// be the one wrong thing to say.
func unansweredText(target string) string {
	return fmt.Sprintf(
		"The %q session finished its turn without answering through lich. "+
			"Whatever it produced is in that session — tell the user to open the %q card to read it.",
		target, target,
	)
}
