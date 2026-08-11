package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"

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

// mcpServerName is the name the tools are namespaced under in a client's tool
// list (`mcp__lich__send_to_session` in Claude Code). It is also the key lich
// registers itself under at spawn.
const mcpServerName = "lich"

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
type mcpTool struct {
	Name        string
	Description string
	Schema      map[string]any
	Run         func(c *client, args mcpArgs) (string, error)
}

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
		response.Result = mcpHandshake(request.Params)
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
func mcpHandshake(params json.RawMessage) map[string]any {
	var asked struct {
		ProtocolVersion string `json:"protocolVersion"`
	}
	_ = json.Unmarshal(params, &asked)
	version := asked.ProtocolVersion
	if version == "" {
		version = mcpProtocolVersion
	}
	return map[string]any{
		"protocolVersion": version,
		"capabilities":    map[string]any{"tools": map[string]any{}},
		"serverInfo":      map[string]any{"name": mcpServerName, "version": "1"},
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
		list = append(list, map[string]any{
			"name":        tool.Name,
			"description": tool.Description,
			"inputSchema": tool.Schema,
		})
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

// flag reads a boolean, and only "yes" counts. A model that sends the string
// "false" — or anything else it invented — must not read as consent, because
// the one flag this carries discards uncommitted work.
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
		Schema: schema(map[string]any{}),
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
			"slower the answer is delivered to your own prompt as soon as it exists, so you can " +
			"carry on and do not need to poll. " +
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
		Name: "wait_for_answer",
		Description: "Hold the line again on a ticket send_to_session handed back. Optional: the " +
			"answer reaches your prompt on its own whether or not you call this. Use it only when " +
			"you have nothing else to do and want the answer inside this turn.",
		Schema: schema(map[string]any{
			"ticket": property("string", "The ticket send_to_session returned."),
			"timeout_seconds": property("number",
				"Seconds to wait, at most 90. Call this again with the same ticket for longer."),
		}, "ticket"),
		Run: func(c *client, args mcpArgs) (string, error) {
			var result relay.Result
			timeout := args.seconds("timeout_seconds")
			if err := c.call("relay.Wait", []any{args.text("ticket"), timeout}, waitBudget(timeout), &result); err != nil {
				return "", err
			}
			return mcpOutcome(result), nil
		},
	},
	{
		Name: "open_session",
		Description: "Open a new lich session and start it, so it can be given work with " +
			"send_to_session. Optionally creates a git worktree first and roots the new " +
			"session in it, which is how you give a task its own checkout instead of " +
			"sharing yours. Returns the names the new session is addressed by. " +
			"It starts empty and idle — nobody has asked it anything yet — and its agent " +
			"takes a few seconds to come up, so send it work as a separate step.",
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
		}),
		Run: func(c *client, args mcpArgs) (string, error) {
			var opened spawn.Session
			call := []any{
				c.sessionID(), args.text("project"), args.text("kind"),
				args.text("worktree"), args.text("base"),
			}
			if err := c.call("spawn.Open", call, openCall, &opened); err != nil {
				return "", err
			}
			return openedText(opened), nil
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
		Run: func(c *client, args mcpArgs) (string, error) {
			var checkouts []spawn.Checkout
			call := []any{c.sessionID(), args.text("project")}
			if err := c.call("spawn.Worktrees", call, shortCall, &checkouts); err != nil {
				return "", err
			}
			if checkouts == nil {
				checkouts = []spawn.Checkout{}
			}
			out, err := json.Marshal(checkouts)
			if err != nil {
				return "", fmt.Errorf("encode the worktrees: %w", err)
			}
			return string(out), nil
		},
	},
	{
		Name: "reply_to_session",
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
		"%s is still working. The task was delivered and its answer will be put at your own "+
			"prompt when it arrives, so carry on — there is nothing to poll. Ticket %q, if you "+
			"want to hold the line for it with wait_for_answer instead.",
		result.Target, result.Ticket,
	)
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
