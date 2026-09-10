package terminal

import (
	"strings"
	"testing"
)

// TestClaudeAgentNameReadsTheLastRecord walks the shapes a Claude transcript
// actually carries (measured on 2.1.263): `custom-title` and `agent-name` are
// both written at spawn under the name lich passed, and `agent-name` is written
// again every turn and on every /rename. The name a session answers to is
// therefore the last record in the file, whichever of the two it is.
func TestClaudeAgentNameReadsTheLastRecord(t *testing.T) {
	const (
		head = `{"type":"last-prompt","leafUuid":"a"}
{"type":"custom-title","customTitle":"lich-4f2a"}
{"type":"agent-name","agentName":"lich-4f2a"}
{"type":"mode","mode":"normal"}`
		turn = `{"type":"assistant","message":{"content":[{"type":"text","text":"done"}]}}`
	)
	tests := []struct {
		name string
		tail string
		want string
	}{
		{
			name: "named at birth and never renamed",
			tail: head + "\n" + turn,
			want: "lich-4f2a",
		},
		{
			name: "one rename",
			tail: head + "\n" + turn + "\n" + `{"type":"agent-name","agentName":"reviewer"}`,
			want: "reviewer",
		},
		{
			name: "several renames, the last one wins",
			tail: strings.Join([]string{
				head,
				`{"type":"agent-name","agentName":"reviewer"}`,
				turn,
				`{"type":"agent-name","agentName":"packager"}`,
				turn,
				`{"type":"agent-name","agentName":"shipper"}`,
			}, "\n"),
			want: "shipper",
		},
		{
			name: "a fork's copied history loses to the name written behind it",
			tail: strings.Join([]string{
				`{"type":"custom-title","customTitle":"parent-9c1b"}`,
				`{"type":"agent-name","agentName":"parent-9c1b"}`,
				turn,
				`{"type":"custom-title","customTitle":"lich-4f2a"}`,
				`{"type":"agent-name","agentName":"lich-4f2a"}`,
			}, "\n"),
			want: "lich-4f2a",
		},
		{
			name: "nothing on record",
			tail: turn + "\n" + turn,
			want: "",
		},
		{
			name: "an empty name is not a name",
			tail: `{"type":"agent-name","agentName":"reviewer"}` + "\n" + `{"type":"agent-name","agentName":""}`,
			want: "reviewer",
		},
		{
			name: "a tail cut mid-line reads like any malformed one",
			tail: `,"agentName":"half-a-line"}` + "\n" + `{"type":"agent-name","agentName":"reviewer"}`,
			want: "reviewer",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := claudeAgentName([]byte(tt.tail + "\n")); got != tt.want {
				t.Errorf("claudeAgentName = %q, want %q", got, tt.want)
			}
		})
	}
}
