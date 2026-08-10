package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"

	"github.com/omartelo/lich/internal/relay"
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

func (a mcpArgs) seconds(key string) int {
	switch v := a[key].(type) {
	case float64:
		return int(v)
	case string:
		var n int
		_, _ = fmt.Sscanf(v, "%d", &n)
		return n
	default:
		return 0
	}
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
		Description: "Give another lich session a task and wait for its agent to answer. " +
			"The task is put at that session's prompt as if typed there. Returns its answer, " +
			"or a ticket to pick the answer up with wait_for_answer if it takes too long.",
		Schema: schema(map[string]any{
			"session": property("string", "Label of the target session, as returned by list_sessions."),
			"prompt":  property("string", "What to ask that session's agent to do."),
			"project": property("string", "Project name, needed only when two live sessions share a label."),
			"timeout_seconds": property("number",
				"How long to wait for an answer. Defaults to 100; the call returns a ticket rather than failing if it runs out."),
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
		Description: "Wait again on a ticket that send_to_session handed back because its wait ran out. " +
			"The task was already delivered; this only waits for the answer.",
		Schema: schema(map[string]any{
			"ticket":          property("string", "The ticket send_to_session returned."),
			"timeout_seconds": property("number", "How long to wait. Defaults to 100."),
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
		Name: "reply_to_session",
		Description: "Answer a task another session gave you. Call this when a message at your prompt " +
			"came from lich and carried a ticket; whoever asked is blocked until you do.",
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
	if result.Status == relay.StatusAnswered {
		return result.Answer
	}
	return fmt.Sprintf(
		"%s has not answered yet. The task was delivered; call wait_for_answer with ticket %q.",
		result.Target, result.Ticket,
	)
}
