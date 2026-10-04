package cli

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"github.com/omartelo/lich/internal/spawn"
)

func controlled(state, action, value string) string {
	out, _ := json.Marshal(spawn.Controlled{
		ID: "s2", Project: "lich", Label: "auth-fix", Action: action, Value: value,
		CommandID: "m7", State: state,
	})
	return string(out)
}

func TestControlPostsTheActionAndItsArguments(t *testing.T) {
	f := newFakeLich(t, controlled(spawn.ControlDone, "command", "compact"))
	code, stdout, stderr := run(t, f, "control", "--project", "lich", "auth-fix", "command", "/compact", "keep the plan")
	if code != 0 {
		t.Fatalf("exit = %d, stderr = %q", code, stderr)
	}
	call := f.only(t)
	if call.method != "spawn.Control" {
		t.Errorf("method = %q", call.method)
	}
	want := []any{"s1", "auth-fix", "lich", "command", "/compact", "keep the plan"}
	if !reflect.DeepEqual(call.args, want) {
		t.Errorf("args = %v, want %v", call.args, want)
	}
	if stdout != "\"auth-fix\" ran /compact.\n" {
		t.Errorf("stdout = %q", stdout)
	}
}

// Each outcome is an exit code a script can branch on, and a line that says
// whether the command still goes through.
func TestControlSaysHowFarTheCommandGot(t *testing.T) {
	tests := []struct {
		state, action, value string
		code                 int
		want                 string
	}{
		{spawn.ControlDone, "prompt", "run the tests", 0, `"auth-fix" started a turn on the prompt.`},
		{spawn.ControlDone, "abort", "", 0, `"auth-fix" stopped its turn.`},
		{spawn.ControlDone, "model", "opus", 0, `"auth-fix" uses opus from its next request on.`},
		{spawn.ControlDone, "model", "", 0, `"auth-fix" is back on its own model.`},
		{spawn.ControlDone, "effort", "high", 0, `"auth-fix" runs at high effort from its next request on.`},
		{spawn.ControlDone, "effort", "", 0, `"auth-fix" is back on its own effort level.`},
		{spawn.ControlDelivered, "command", "compact", ExitPending, "id m7) and has not confirmed it yet"},
		{spawn.ControlEnded, "prompt", "go", ExitNoAnswer, `"auth-fix" ended before confirming the command (prompt, id m7)`},
	}
	for _, tc := range tests {
		t.Run(tc.state+" "+tc.action+" "+tc.value, func(t *testing.T) {
			f := newFakeLich(t, controlled(tc.state, tc.action, tc.value))
			args := []string{"control", "auth-fix", tc.action}
			if tc.value != "" {
				args = append(args, tc.value)
			}
			code, stdout, stderr := run(t, f, args...)
			if code != tc.code {
				t.Fatalf("exit = %d, want %d (stderr %q)", code, tc.code, stderr)
			}
			if !strings.Contains(stdout, tc.want) {
				t.Errorf("stdout = %q, want it to say %q", stdout, tc.want)
			}
		})
	}
}

func TestControlJSONKeepsTheExitCode(t *testing.T) {
	f := newFakeLich(t, controlled(spawn.ControlDelivered, "command", "compact"))
	code, stdout, _ := run(t, f, "control", "--json", "auth-fix", "command", "compact")
	if code != ExitPending {
		t.Fatalf("exit = %d, want %d", code, ExitPending)
	}
	var got map[string]any
	if err := json.Unmarshal([]byte(stdout), &got); err != nil {
		t.Fatalf("stdout %q is not JSON: %v", stdout, err)
	}
	if got["state"] != "delivered" || got["command_id"] != "m7" || got["label"] != "auth-fix" {
		t.Errorf("json = %v", got)
	}
}

func TestControlRefusesAMalformedCall(t *testing.T) {
	f := newFakeLich(t, `null`)
	for _, args := range [][]string{
		{"control", "auth-fix"},
		{"control", "auth-fix", "command", "compact", "keep", "extra"},
	} {
		if code, _, stderr := run(t, f, args...); code != 1 || !strings.Contains(stderr, "usage: lich control") {
			t.Errorf("Run(%q) = %d, stderr %q; want 1 with the usage line", args, code, stderr)
		}
	}
	if len(f.calls) != 0 {
		t.Errorf("a malformed call reached the app: %+v", f.calls)
	}
}

func TestControlReportsTheRefusal(t *testing.T) {
	f := newFakeLich(t, `{"error":"\"codex-run\" runs codex, and only a Claude Code session can be controlled"}`)
	f.status = 400
	code, stdout, stderr := run(t, f, "control", "codex-run", "abort")
	if code != 1 || stdout != "" {
		t.Fatalf("exit = %d, stdout = %q; want 1 and nothing on stdout", code, stdout)
	}
	if !strings.HasPrefix(stderr, "lich: ") || !strings.Contains(stderr, "only a Claude Code session") {
		t.Errorf("stderr = %q", stderr)
	}
}

// A command still on its way is a result an agent reads, not a failed call.
func TestMCPControlSessionReportsAPendingCommandAsAResult(t *testing.T) {
	f := newFakeLich(t, controlled(spawn.ControlDelivered, "command", "compact"))
	replies := speak(t, f, `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":
		{"name":"control_session","arguments":{"session":"auth-fix","action":"command","value":"compact","args":"keep the plan"}}}`)

	text, failed := textOf(t, replies[0])
	if failed {
		t.Fatalf("tool reported a failure: %s", text)
	}
	if !strings.Contains(text, "It still goes through") {
		t.Errorf("text = %q", text)
	}
	want := []any{"s1", "auth-fix", "", "command", "compact", "keep the plan"}
	if call := f.only(t); !reflect.DeepEqual(call.args, want) {
		t.Errorf("args = %v, want %v", call.args, want)
	}
}
