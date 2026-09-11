package cli

import (
	"bytes"
	"strings"
	"testing"
)

func TestBrowserOpenCallsThisSessionsBrowser(t *testing.T) {
	f := newFakeLich(t, `{"id":"b1","url":"https://example.com","owner":"s1","headed":false}`)

	code, stdout, stderr := run(t, f, "browser", "open", "https://example.com")
	if code != 0 {
		t.Fatalf("exit = %d, stderr = %q", code, stderr)
	}
	call := f.only(t)
	if call.method != "browser.Open" {
		t.Errorf("method = %q", call.method)
	}
	if len(call.args) != 2 || call.args[0] != "s1" || call.args[1] != "https://example.com" {
		t.Errorf("args = %v, want this session and the url", call.args)
	}
	if !strings.Contains(stdout, "https://example.com") {
		t.Errorf("stdout = %q", stdout)
	}
}

func TestBrowserClickSendsATarget(t *testing.T) {
	f := newFakeLich(t, `null`)

	code, _, stderr := run(t, f, "browser", "click", "--index", "3")
	if code != 0 {
		t.Fatalf("exit = %d, stderr = %q", code, stderr)
	}
	call := f.only(t)
	if call.method != "browser.Click" {
		t.Errorf("method = %q", call.method)
	}
	if len(call.args) != 2 || call.args[0] != "s1" {
		t.Errorf("args = %v", call.args)
	}
	target, _ := call.args[1].(map[string]any)
	if target["index"] != float64(3) {
		t.Errorf("target = %v", call.args[1])
	}
}

func TestBrowserPressSendsKeyAndOptionalTarget(t *testing.T) {
	f := newFakeLich(t, `null`)

	code, _, stderr := run(t, f, "browser", "press", "--index", "2", "Enter")
	if code != 0 {
		t.Fatalf("exit = %d, stderr = %q", code, stderr)
	}
	call := f.only(t)
	if call.method != "browser.Press" {
		t.Errorf("method = %q", call.method)
	}
	if len(call.args) != 3 || call.args[0] != "s1" || call.args[1] != "Enter" {
		t.Errorf("args = %v", call.args)
	}
	target, _ := call.args[2].(map[string]any)
	if target["index"] != float64(2) {
		t.Errorf("target = %v", call.args[2])
	}
}

func TestMCPBrowserPressCallsPress(t *testing.T) {
	f := newFakeLich(t, `null`)

	replies := speak(t, f, `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":
		{"name":"browser_press","arguments":{"key":"Tab"}}}`)
	text, failed := textOf(t, replies[0])
	if failed {
		t.Fatalf("tool reported a failure: %s", text)
	}
	call := f.only(t)
	if call.method != "browser.Press" {
		t.Errorf("method = %q", call.method)
	}
	if len(call.args) != 3 || call.args[0] != "s1" || call.args[1] != "Tab" {
		t.Errorf("args = %v", call.args)
	}
}

func TestBrowserRefusesWithoutASession(t *testing.T) {
	f := newFakeLich(t, `null`)
	var stdout, stderr bytes.Buffer
	code := dispatch([]string{"browser", "open", "https://example.com"}, &client{
		env: func(key string) string {
			if key == "LICH_SESSION_ID" {
				return ""
			}
			return f.env(key)
		},
		version: testVersion, stdout: &stdout, stderr: &stderr, running: noInstance,
	})
	if code != 1 {
		t.Fatalf("exit = %d, want 1", code)
	}
	if !strings.Contains(stderr.String(), "inside a lich session") {
		t.Errorf("stderr = %q", stderr.String())
	}
	if len(f.calls) != 0 {
		t.Errorf("reached the app without a session: %+v", f.calls)
	}
}

func TestMCPBrowserOpenUsesTheCaller(t *testing.T) {
	f := newFakeLich(t, `{"id":"b1","url":"https://example.com","owner":"s1","headed":false}`)

	replies := speak(t, f, `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":
		{"name":"browser_open","arguments":{"url":"https://example.com"}}}`)
	text, failed := textOf(t, replies[0])
	if failed {
		t.Fatalf("tool reported a failure: %s", text)
	}
	call := f.only(t)
	if call.method != "browser.Open" {
		t.Errorf("method = %q", call.method)
	}
	if len(call.args) != 2 || call.args[0] != "s1" {
		t.Errorf("args = %v, want this session", call.args)
	}
}
