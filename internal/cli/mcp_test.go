package cli

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

// speak runs the MCP server over one scripted conversation and returns the
// responses in order. Every message is one line, which is the stdio transport.
func speak(t *testing.T, f *fakeLich, messages ...string) []jsonRPC {
	t.Helper()
	var stdout, stderr bytes.Buffer
	c := &client{
		env:     f.env,
		stdin:   strings.NewReader(strings.Join(messages, "\n") + "\n"),
		stdout:  &stdout,
		stderr:  &stderr,
		running: noInstance,
	}
	if code := dispatch([]string{"mcp"}, c); code != 0 {
		t.Fatalf("exit = %d, stderr = %q", code, stderr.String())
	}

	var replies []jsonRPC
	decoder := json.NewDecoder(&stdout)
	for decoder.More() {
		var reply jsonRPC
		if err := decoder.Decode(&reply); err != nil {
			t.Fatalf("undecodable response: %v", err)
		}
		replies = append(replies, reply)
	}
	return replies
}

// resultOf re-decodes a response's result into v, which is how a test reads a
// map[string]any the server built.
func resultOf(t *testing.T, reply jsonRPC, v any) {
	t.Helper()
	raw, err := json.Marshal(reply.Result)
	if err != nil {
		t.Fatalf("marshal result: %v", err)
	}
	if err := json.Unmarshal(raw, v); err != nil {
		t.Fatalf("decode result: %v", err)
	}
}

// textOf pulls the text out of a tool result's content blocks.
func textOf(t *testing.T, reply jsonRPC) (string, bool) {
	t.Helper()
	var result struct {
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
		IsError bool `json:"isError"`
	}
	resultOf(t, reply, &result)
	if len(result.Content) != 1 || result.Content[0].Type != "text" {
		t.Fatalf("want one text block, got %+v", result.Content)
	}
	return result.Content[0].Text, result.IsError
}

func TestMCPHandshakeAdvertisesTools(t *testing.T) {
	f := newFakeLich(t, `null`)

	replies := speak(t, f, `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-03-26"}}`)
	if len(replies) != 1 {
		t.Fatalf("got %d replies, want 1", len(replies))
	}

	var result struct {
		ProtocolVersion string `json:"protocolVersion"`
		Capabilities    struct {
			Tools *struct{} `json:"tools"`
		} `json:"capabilities"`
		ServerInfo struct {
			Name string `json:"name"`
		} `json:"serverInfo"`
	}
	resultOf(t, replies[0], &result)

	if result.ProtocolVersion != "2025-03-26" {
		t.Errorf("protocolVersion = %q, want the client's own", result.ProtocolVersion)
	}
	if result.Capabilities.Tools == nil {
		t.Error("the server did not advertise tools")
	}
	if result.ServerInfo.Name != "lich" {
		t.Errorf("serverInfo.name = %q", result.ServerInfo.Name)
	}
}

func TestMCPHandshakeFallsBackToItsOwnVersion(t *testing.T) {
	f := newFakeLich(t, `null`)

	replies := speak(t, f, `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}`)
	var result struct {
		ProtocolVersion string `json:"protocolVersion"`
	}
	resultOf(t, replies[0], &result)
	if result.ProtocolVersion != mcpProtocolVersion {
		t.Errorf("protocolVersion = %q, want %q", result.ProtocolVersion, mcpProtocolVersion)
	}
}

func TestMCPNotificationsGetNoReply(t *testing.T) {
	f := newFakeLich(t, `null`)

	replies := speak(t, f,
		`{"jsonrpc":"2.0","method":"notifications/initialized"}`,
		`{"jsonrpc":"2.0","id":1,"method":"ping"}`,
	)
	if len(replies) != 1 {
		t.Fatalf("got %d replies, want 1 — a notification must be answered with nothing", len(replies))
	}
	if string(replies[0].ID) != "1" {
		t.Errorf("the one reply answered id %s", replies[0].ID)
	}
}

func TestMCPListsEveryTool(t *testing.T) {
	f := newFakeLich(t, `null`)

	replies := speak(t, f, `{"jsonrpc":"2.0","id":1,"method":"tools/list"}`)
	var result struct {
		Tools []struct {
			Name        string `json:"name"`
			Description string `json:"description"`
			InputSchema struct {
				Type       string                     `json:"type"`
				Properties map[string]json.RawMessage `json:"properties"`
				Required   []string                   `json:"required"`
			} `json:"inputSchema"`
		} `json:"tools"`
	}
	resultOf(t, replies[0], &result)

	want := map[string][]string{
		"list_sessions":    {},
		"open_session":     {},
		"send_to_session":  {"session", "prompt"},
		"wait_for_answer":  {"ticket"},
		"reply_to_session": {"ticket", "answer"},
	}
	if len(result.Tools) != len(want) {
		t.Fatalf("got %d tools, want %d", len(result.Tools), len(want))
	}
	for _, tool := range result.Tools {
		required, known := want[tool.Name]
		if !known {
			t.Errorf("unexpected tool %q", tool.Name)
			continue
		}
		if tool.Description == "" {
			t.Errorf("%s has no description — it is all the agent has to go on", tool.Name)
		}
		if tool.InputSchema.Type != "object" {
			t.Errorf("%s schema type = %q", tool.Name, tool.InputSchema.Type)
		}
		if len(tool.InputSchema.Required) != len(required) {
			t.Errorf("%s required = %v, want %v", tool.Name, tool.InputSchema.Required, required)
		}
		for _, name := range required {
			if _, ok := tool.InputSchema.Properties[name]; !ok {
				t.Errorf("%s requires %q but does not declare it", tool.Name, name)
			}
		}
	}
}

func TestMCPSendToSessionReturnsTheAnswer(t *testing.T) {
	f := newFakeLich(t, `{"ticket":"a1b2c3d4","target":"docs","status":"answered","answer":"3 failures"}`)

	replies := speak(t, f, `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":
		{"name":"send_to_session","arguments":{"session":"docs","prompt":"run the tests","timeout_seconds":20}}}`)

	text, failed := textOf(t, replies[0])
	if failed {
		t.Fatalf("tool reported a failure: %s", text)
	}
	if text != "3 failures" {
		t.Errorf("text = %q, want the answer alone", text)
	}

	call := f.only(t)
	if call.method != "relay.Send" {
		t.Fatalf("method = %q", call.method)
	}
	want := []any{"s1", "docs", "", "run the tests", float64(20)}
	for i := range want {
		if call.args[i] != want[i] {
			t.Errorf("argument %d = %v, want %v", i, call.args[i], want[i])
		}
	}
}

func TestMCPSendPointsAtTheTicketWhenItRunsOut(t *testing.T) {
	f := newFakeLich(t, `{"ticket":"a1b2c3d4","target":"docs","status":"pending","answer":""}`)

	replies := speak(t, f, `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":
		{"name":"send_to_session","arguments":{"session":"docs","prompt":"long errand"}}}`)

	text, failed := textOf(t, replies[0])
	if failed {
		t.Fatalf("a delivered-but-unanswered task was reported as a failure: %s", text)
	}
	for _, want := range []string{"wait_for_answer", "a1b2c3d4"} {
		if !strings.Contains(text, want) {
			t.Errorf("text is missing %q: %s", want, text)
		}
	}
}

func TestMCPListSessionsReturnsThemAsJSON(t *testing.T) {
	f := newFakeLich(t, `[{"label":"docs","project":"lich","kind":"codex"}]`)

	replies := speak(t, f, `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":
		{"name":"list_sessions","arguments":{}}}`)

	text, failed := textOf(t, replies[0])
	if failed {
		t.Fatalf("tool reported a failure: %s", text)
	}
	var peers []struct {
		Label string `json:"label"`
	}
	if err := json.Unmarshal([]byte(text), &peers); err != nil {
		t.Fatalf("list_sessions did not return JSON: %v (%q)", err, text)
	}
	if len(peers) != 1 || peers[0].Label != "docs" {
		t.Errorf("peers = %+v", peers)
	}
}

func TestMCPReplyToSession(t *testing.T) {
	f := newFakeLich(t, `null`)

	replies := speak(t, f, `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":
		{"name":"reply_to_session","arguments":{"ticket":"a1b2c3d4","answer":"done"}}}`)

	if text, failed := textOf(t, replies[0]); failed {
		t.Fatalf("tool reported a failure: %s", text)
	}
	call := f.only(t)
	if call.method != "relay.Reply" || call.args[0] != "a1b2c3d4" || call.args[1] != "done" {
		t.Errorf("call = %+v", call)
	}
}

func TestMCPAFailedCallIsAToolErrorNotAProtocolError(t *testing.T) {
	f := newFakeLich(t, `{"error":"no live session named \"ghost\""}`)
	f.status = 500

	replies := speak(t, f, `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":
		{"name":"send_to_session","arguments":{"session":"ghost","prompt":"hello"}}}`)

	if replies[0].Error != nil {
		t.Fatalf("a tool failure came back as a protocol error: %+v", replies[0].Error)
	}
	text, failed := textOf(t, replies[0])
	if !failed {
		t.Error("the tool result was not marked isError")
	}
	if !strings.Contains(text, `no live session named "ghost"`) {
		t.Errorf("text = %q, want the app's own message", text)
	}
}

func TestMCPRejectsUnknownMethodsAndTools(t *testing.T) {
	f := newFakeLich(t, `null`)

	replies := speak(t, f,
		`{"jsonrpc":"2.0","id":1,"method":"resources/list"}`,
		`{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"delete_everything","arguments":{}}}`,
	)
	if len(replies) != 2 {
		t.Fatalf("got %d replies, want 2", len(replies))
	}
	if replies[0].Error == nil || replies[0].Error.Code != codeMethodNotFound {
		t.Errorf("unknown method answered %+v", replies[0])
	}
	if replies[1].Error == nil || replies[1].Error.Code != codeInvalidParams {
		t.Errorf("unknown tool answered %+v", replies[1])
	}
	if len(f.calls) != 0 {
		t.Errorf("a rejected message reached the app: %+v", f.calls)
	}
}

func TestMCPReadsLooseArgumentTypes(t *testing.T) {
	f := newFakeLich(t, `{"ticket":"a1b2c3d4","target":"docs","status":"answered","answer":"ok"}`)

	// A model that sends the timeout as a string should not lose its turn.
	speak(t, f, `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":
		{"name":"send_to_session","arguments":{"session":"docs","prompt":"hi","timeout_seconds":"15"}}}`)

	if got := f.only(t).args[4]; got != float64(15) {
		t.Errorf("timeout = %v, want 15", got)
	}
}

// TestMCPCapsTheWaitBelowTheClientsBackgroundThreshold pins the ceiling that
// keeps a wait a wait. Claude Code detaches an MCP call that runs past 120
// seconds, so a caller asking for more than the cap gets the cap — the first
// real run asked for 120 and then 180, and both stopped waiting.
func TestMCPCapsTheWaitBelowTheClientsBackgroundThreshold(t *testing.T) {
	tests := []struct {
		name  string
		asked any
		want  float64
	}{
		{"under the cap is honoured", float64(20), 20},
		{"the cap itself", float64(90), 90},
		{"over the cap is clamped", float64(180), 90},
		{"the client's own threshold is already too long", float64(120), 90},
		{"omitted falls back to the cap", nil, 90},
		{"zero falls back to the cap", float64(0), 90},
		{"negative falls back to the cap", float64(-5), 90},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := newFakeLich(t, `{"ticket":"a1b2c3d4","target":"docs","status":"answered","answer":"ok"}`)
			args := map[string]any{"session": "docs", "prompt": "hi"}
			if tt.asked != nil {
				args["timeout_seconds"] = tt.asked
			}
			call, err := json.Marshal(map[string]any{
				"jsonrpc": "2.0", "id": 1, "method": "tools/call",
				"params": map[string]any{"name": "send_to_session", "arguments": args},
			})
			if err != nil {
				t.Fatalf("marshal: %v", err)
			}
			speak(t, f, string(call))

			if got := f.only(t).args[4]; got != tt.want {
				t.Errorf("timeout = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestMCPStopsCleanlyAtEndOfInput(t *testing.T) {
	f := newFakeLich(t, `null`)

	var stdout, stderr bytes.Buffer
	c := &client{env: f.env, stdin: strings.NewReader(""), stdout: &stdout, stderr: &stderr, running: noInstance}
	if code := dispatch([]string{"mcp"}, c); code != 0 {
		t.Fatalf("exit = %d, stderr = %q", code, stderr.String())
	}
	if stdout.Len() != 0 {
		t.Errorf("an empty conversation produced output: %q", stdout.String())
	}
}

func TestMCPOpenSessionReturnsTheNamesItIsAddressedBy(t *testing.T) {
	f := newFakeLich(t, `{"id":"9f8e","projectId":"p1","project":"lich","label":"auth-fix",
		"name":"auth-fix-9f8e","kind":"claude","path":"/wt/auth-fix","nextSeq":5}`)

	replies := speak(t, f, `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":
		{"name":"open_session","arguments":{"worktree":"auth-fix","base":"main","kind":"codex"}}}`)

	text, failed := textOf(t, replies[0])
	if failed {
		t.Fatalf("tool reported a failure: %s", text)
	}
	call := f.only(t)
	if call.method != "spawn.Open" {
		t.Errorf("method = %q", call.method)
	}
	want := []any{"s1", "", "codex", "auth-fix", "main"}
	if len(call.args) != len(want) {
		t.Fatalf("args = %v, want %v", call.args, want)
	}
	for i := range want {
		if call.args[i] != want[i] {
			t.Errorf("argument %d = %v, want %v", i, call.args[i], want[i])
		}
	}
	// send_to_session is the next call the agent makes, and it addresses the
	// session by one of these two names.
	for _, phrase := range []string{`"auth-fix"`, `"auth-fix-9f8e"`} {
		if !strings.Contains(text, phrase) {
			t.Errorf("result is missing %q:\n%s", phrase, text)
		}
	}
}

func TestMCPOpenSessionDefaultsEveryArgument(t *testing.T) {
	f := newFakeLich(t, `{"id":"9f8e","projectId":"p1","project":"lich","label":"Session 4",
		"name":"lich-9f8e","kind":"claude","path":"","nextSeq":5}`)

	replies := speak(t, f, `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":
		{"name":"open_session","arguments":{}}}`)

	if text, failed := textOf(t, replies[0]); failed {
		t.Fatalf("tool reported a failure: %s", text)
	}
	call := f.only(t)
	want := []any{"s1", "", "", "", ""}
	for i := range want {
		if call.args[i] != want[i] {
			t.Errorf("argument %d = %v, want %v", i, call.args[i], want[i])
		}
	}
}

func TestMCPSendReportsATaskNobodyRead(t *testing.T) {
	f := newFakeLich(t, `{"ticket":"a1b2c3d4","target":"docs","status":"unread"}`)

	replies := speak(t, f, `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":
		{"name":"send_to_session","arguments":{"session":"docs","prompt":"run the tests"}}}`)

	text, failed := textOf(t, replies[0])
	if failed {
		t.Fatalf("tool reported a failure: %s", text)
	}
	if !strings.Contains(text, "never picked the task up") {
		t.Errorf("text = %q, want it to say the task was not read", text)
	}
	// An agent told to wait on this would poll a ticket that no longer exists.
	if strings.Contains(text, "wait_for_answer") {
		t.Errorf("pointed at a ticket for a task nobody read: %q", text)
	}
}
