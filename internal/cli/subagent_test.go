package cli

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/omartelo/lich/internal/spawn"
)

const subagentPending = `{"ticket":"a1b2c3d4","target":"Session 4","status":"pending","answer":""}`

func TestOpenSubagentOpensAndHandsOverAsASubagent(t *testing.T) {
	f := newFakeLich(t, openedBody)
	f.answers = map[string]answer{"relay.SendSubagent": {status: 200, body: subagentPending}}

	code, stdout, stderr := run(t, f,
		"open", "--subagent", "--kind", "claude", "--worktree", "auth-fix", "--base", "main",
		"--model", "opus", "--prompt", "port the parser", "--json")
	if code != 0 {
		t.Fatalf("exit = %d, stderr = %q", code, stderr)
	}
	if len(f.calls) != 2 || f.calls[0].method != "spawn.OpenSubagent" || f.calls[1].method != "relay.SendSubagent" {
		t.Fatalf("calls = %+v, want spawn.OpenSubagent then relay.SendSubagent", f.calls)
	}
	want := spawn.OpenSubagentOptions{From: "s1", Kind: "claude", Worktree: "auth-fix", Base: "main", Model: "opus"}
	if got := optionsOf[spawn.OpenSubagentOptions](t, f.calls[0]); got != want {
		t.Errorf("open options = %+v, want %+v", got, want)
	}
	// --json keeps its one shape: the session, and the delivery beside it.
	var out map[string]any
	if err := json.Unmarshal([]byte(stdout), &out); err != nil {
		t.Fatalf("stdout is not one JSON object: %v\n%s", err, stdout)
	}
	if out["label"] != "auth-fix" || out["delivery"].(map[string]any)["ticket"] != "a1b2c3d4" {
		t.Errorf("stdout = %v, want the session and its delivery", out)
	}
}

func TestOpenSubagentIsRefusedBeforeOpening(t *testing.T) {
	cases := map[string][]string{
		"--prompt":  {"open", "--subagent"},
		"--project": {"open", "--subagent", "--prompt", "x", "--project", "lich"},
		"--folder":  {"open", "--subagent", "--prompt", "x", "--folder", "Apps"},
		"--private": {"open", "--subagent", "--prompt", "x", "--private"},
	}
	for flag, args := range cases {
		t.Run(flag, func(t *testing.T) {
			f := newFakeLich(t, openedBody)
			code, _, stderr := run(t, f, args...)
			if code == 0 || len(f.calls) != 0 {
				t.Fatalf("exit %d, calls %+v; want a refusal with nothing opened", code, f.calls)
			}
			if !strings.Contains(stderr, flag) {
				t.Errorf("stderr = %q, want it to name %s", stderr, flag)
			}
		})
	}
}

func TestOpenSubagentNeedsACallingSession(t *testing.T) {
	f := newFakeLich(t, openedBody)
	outside := func(key string) string {
		if key == "LICH_SESSION_ID" {
			return ""
		}
		return f.env(key)
	}
	var stdout, stderr bytes.Buffer
	code := dispatch([]string{"open", "--subagent", "--prompt", "x"}, &client{
		env: outside, version: testVersion, stdout: &stdout, stderr: &stderr, running: noInstance,
	})
	if code == 0 || len(f.calls) != 0 {
		t.Fatalf("exit %d, calls %+v; want a refusal with nothing opened", code, f.calls)
	}
	if !strings.Contains(stderr.String(), "LICH_SESSION_ID") {
		t.Errorf("stderr = %q, want it to say why", stderr.String())
	}
}
